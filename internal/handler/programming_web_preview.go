package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const previewLeaseTTL = 10 * time.Minute

var previewHostPattern = regexp.MustCompile(`^wkpreview-([a-f0-9]{32})\.`)

type previewLease struct {
	UserID    string
	TenantID  uint64
	SessionID string
	SandboxID string
	Port      int
	RelayPort int
	Secret    string
	Host      string
	AppOrigin string
	Expires   time.Time
}
type previewLeaseStore struct {
	redis  *redis.Client
	mu     sync.Mutex
	leases map[string]previewLease
}

func (s *previewLeaseStore) get(ctx context.Context, token string) (previewLease, bool) {
	var lease previewLease
	if s.redis != nil {
		raw, err := s.redis.Get(ctx, "weknora:web-preview:"+token).Bytes()
		if err != nil || json.Unmarshal(raw, &lease) != nil {
			return lease, false
		}
	} else {
		s.mu.Lock()
		lease = s.leases[token]
		s.mu.Unlock()
	}
	return lease, lease.UserID != "" && time.Now().Before(lease.Expires)
}
func (s *previewLeaseStore) put(ctx context.Context, token string, lease previewLease) error {
	lease.Expires = time.Now().Add(previewLeaseTTL)
	if s.redis != nil {
		raw, _ := json.Marshal(lease)
		return s.redis.Set(ctx, "weknora:web-preview:"+token, raw, previewLeaseTTL).Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, item := range s.leases {
		if time.Now().After(item.Expires) {
			delete(s.leases, key)
		}
	}
	s.leases[token] = lease
	return nil
}

// drop forgets a lease whose relay stopped answering, so the next discovery
// mints a fresh one instead of leaving the panel pointed at a frame that only
// ever returns 502.
func (s *previewLeaseStore) drop(ctx context.Context, token string) {
	if s.redis != nil {
		s.redis.Del(ctx, "weknora:web-preview:"+token)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.leases, token)
}

// Install before application CORS/auth/routes: a preview origin must NEVER
// fall through to the authenticated application, including on an invalid lease.
func (h *ProgrammingHandler) WebPreviewMiddleware(client *redis.Client) gin.HandlerFunc {
	h.previewLeases = &previewLeaseStore{redis: client, leases: make(map[string]previewLease)}
	return func(c *gin.Context) {
		hostname := strings.ToLower(c.Request.Host)
		if host, _, err := net.SplitHostPort(hostname); err == nil {
			hostname = host
		}
		if !strings.HasPrefix(hostname, "wkpreview-") {
			c.Next()
			return
		}
		c.Abort()
		match := previewHostPattern.FindStringSubmatch(hostname)
		if len(match) != 2 {
			c.Status(http.StatusNotFound)
			return
		}
		lease, ok := h.previewLeases.get(c.Request.Context(), match[1])
		if !ok || !strings.EqualFold(lease.Host, c.Request.Host) {
			c.Status(http.StatusNotFound)
			return
		}
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, lease.UserID)
		ctx = context.WithValue(ctx, types.TenantIDContextKey, lease.TenantID)
		if _, err := h.sessions.GetOwnedSession(ctx, lease.SessionID); err != nil {
			c.Status(http.StatusForbidden)
			return
		}
		if c.Request.URL.Path == "/.weknora/preview-health" {
			// Only the owning application's browser may read this dedicated
			// probe. Project responses otherwise retain their CORS isolation.
			if c.GetHeader("Origin") != lease.AppOrigin || c.Request.Method != http.MethodGet {
				c.Status(http.StatusForbidden)
				return
			}
			c.Header("Access-Control-Allow-Origin", lease.AppOrigin)
			c.Header("Vary", "Origin")
			c.Header("Cache-Control", "no-store")
			probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			req, _ := http.NewRequestWithContext(probeCtx, http.MethodGet, "http://localhost/", nil)
			req.Header.Set("X-WeKnora-Preview-Relay", lease.Secret)
			response, err := h.workspace.RoundTripWebPreview(probeCtx, lease.SessionID, lease.SandboxID, lease.RelayPort, req)
			if response != nil && response.Body != nil {
				defer response.Body.Close()
			}
			if err != nil || response == nil || response.StatusCode >= 400 {
				c.JSON(502, gin.H{"status": "unreachable"})
				return
			}
			c.JSON(200, gin.H{"status": "ready"})
			return
		}
		proxy := &httputil.ReverseProxy{
			Rewrite: func(p *httputil.ProxyRequest) {
				p.Out.URL.Scheme = "http"
				p.Out.URL.Host = "localhost"
				p.Out.Host = "localhost"
				for _, name := range []string{"Authorization", "X-API-Key", "X-Tenant-ID", "X-Embed-Session", "X-External-User-Token", sandbox.InboundTokenHeader} {
					p.Out.Header.Del(name)
				}
				p.Out.Header.Set("X-WeKnora-Preview-Relay", lease.Secret)
				// Project websocket Origin checks should see the project listener.
				if p.Out.Header.Get("Origin") != "" {
					p.Out.Header.Set("Origin", "http://localhost:"+strconv.Itoa(lease.Port))
				}
			},
			Transport: previewRoundTripper(func(req *http.Request) (*http.Response, error) {
				return h.workspace.RoundTripWebPreview(ctx, lease.SessionID, lease.SandboxID, lease.RelayPort, req)
			}),
			ModifyResponse: func(response *http.Response) error {
				// Gateways can return an HTTP error without a transport error.
				// Retire that URL so the next successful discovery navigates the
				// iframe again, including after a JS module failed to load.
				switch response.StatusCode {
				case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
					h.previewLeases.drop(ctx, match[1])
				}
				response.Header.Del(sandbox.InboundTokenHeader)
				response.Header.Del("X-WeKnora-Preview-Relay")
				response.Header.Del("X-Frame-Options")
				response.Header.Set("Content-Security-Policy", "frame-ancestors "+lease.AppOrigin+"; sandbox allow-scripts allow-same-origin allow-forms allow-downloads")
				response.Header.Set("Referrer-Policy", "no-referrer")
				response.Header.Set("Cache-Control", "no-store")
				response.Header.Del("Access-Control-Allow-Origin")
				response.Header.Del("Access-Control-Allow-Credentials")
				// Project cookies are restricted to this project's unique origin.
				cookies := response.Cookies()
				response.Header.Del("Set-Cookie")
				for _, cookie := range cookies {
					cookie.Domain = ""
					response.Header.Add("Set-Cookie", cookie.String())
				}
				if location, err := url.Parse(response.Header.Get("Location")); err == nil && location.Host != "" && (location.Hostname() == "localhost" || location.Hostname() == "127.0.0.1") && location.Port() == strconv.Itoa(lease.Port) {
					location.Scheme = ""
					location.Host = ""
					response.Header.Set("Location", location.String())
				}
				return nil
			},
			ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
				// The relay inside the sandbox stopped answering, or it can no
				// longer reach the project's port. Dropping the lease lets the
				// next poll mint a fresh URL and, when the project is really
				// gone, report that instead — so the panel returns to a state the
				// user can act on rather than a frame that only ever shows 502.
				h.previewLeases.drop(ctx, match[1])
				http.Error(w, "Project preview is unavailable. Reopen Web preview to reconnect.", http.StatusBadGateway)
			},
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<20)
		proxy.ServeHTTP(c.Writer, c.Request.WithContext(ctx))
	}
}

type previewRoundTripper func(*http.Request) (*http.Response, error)

func (f previewRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Public base routes *.base to this application's HTTP listener. Development
// uses *.localhost on the API listener, requiring no DNS or desktop sandbox.
// A literal IP origin is pointed at itself through nip.io's wildcard DNS, so the
// base follows whatever address the browser actually used: a pinned address in
// WEKNORA_PREVIEW_BASE_URL goes stale the moment the host moves, and the only
// symptom is a blank preview frame. A named origin still needs that variable,
// since its own wildcard DNS and certificate are not ours to assume.
func previewPublicBase(request *http.Request) (*url.URL, error) {
	raw := strings.TrimSpace(os.Getenv("WEKNORA_PREVIEW_BASE_URL"))
	if raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return nil, errors.New("invalid preview base")
		}
		u.Path = ""
		return u, nil
	}
	origin, err := url.Parse(request.Header.Get("Origin"))
	if err != nil || origin == nil || origin.Host == "" || origin.User != nil || (origin.Scheme != "http" && origin.Scheme != "https") {
		return nil, errors.New("preview domain required")
	}
	// Hostname from the origin, port from this request, because the two sources
	// answer different questions:
	//
	//   * The hostname must be the address the browser actually used. A dev
	//     server proxies the API to the backend and rewrites Host, so the request
	//     host reads "localhost" even for a browser on another machine — and
	//     *.localhost resolves to that browser's own loopback, which refuses the
	//     connection. The origin names the machine the page was loaded from.
	//   * The port must be the backend's, since the preview relay lives on this
	//     listener. In development the page is served by another port entirely,
	//     so an origin-derived port would point the frame at the dev server.
	//
	// The scheme follows the origin, which is what the browser enforces.
	hostname := origin.Hostname()
	_, port, err := net.SplitHostPort(request.Host)
	if err != nil {
		port = ""
	}
	if port == "" {
		port = "80"
		if origin.Scheme == "https" {
			port = "443"
		}
	}
	// Loopback needs no DNS: *.localhost already resolves.
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return &url.URL{Scheme: origin.Scheme, Host: net.JoinHostPort("localhost", port)}, nil
	}
	// Any other literal address was reachable enough for the browser to load the
	// app from it, so preview subdomains point at that same address.
	ip := net.ParseIP(hostname)
	if ip == nil || ip.To4() == nil {
		return nil, errors.New("preview domain required")
	}
	return &url.URL{
		Scheme: origin.Scheme,
		Host:   net.JoinHostPort(strings.ReplaceAll(ip.String(), ".", "-")+".nip.io", port),
	}, nil
}

func (h *ProgrammingHandler) DiscoverWebPreview(c *gin.Context) {
	session, ok := h.loadOwnedSession(c)
	if !ok {
		return
	}
	var request struct {
		Start               bool        `json:"start"`
		PreviewID           string      `json:"preview_id"`
		AgentID             string      `json:"agent_id"`
		AgentSourceTenantID json.Number `json:"agent_source_tenant_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	sourceTenantID := uint64(0)
	if raw := string(request.AgentSourceTenantID); raw != "" {
		var parseErr error
		sourceTenantID, parseErr = strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			c.Status(http.StatusBadRequest)
			return
		}
	}
	app, err := url.Parse(c.GetHeader("Origin"))
	if err != nil || app == nil || (app.Scheme != "http" && app.Scheme != "https") || app.Host == "" || app.User != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	base, err := previewPublicBase(c.Request)
	if err != nil {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "setup_required"}})
		return
	}
	ctx := c.Request.Context()
	sessionID := programmingSessionID(c)
	if h.workspace == nil {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "unsupported"}})
		return
	}
	if request.Start && h.previewStarter != nil {
		pin := service.SandboxPin{}
		// Existing pins stay authoritative, including for shared agents and
		// sessions whose selected agent was changed since sandbox creation.
		if session.SandboxConfigID == "" {
			var agent *types.CustomAgent
			if sourceTenantID == 0 && h.previewAgents != nil {
				agent, _ = h.previewAgents.GetAgentByID(ctx, request.AgentID)
			}
			if agent == nil && h.previewShares != nil {
				tenantID, _ := types.TenantIDFromContext(ctx)
				agent, _ = h.previewShares.GetSharedAgentForTenant(ctx, tenantID, types.TenantRoleFromContext(ctx), request.AgentID, sourceTenantID)
			}
			if agent == nil || strings.TrimSpace(agent.Config.SandboxConfigID) == "" {
				c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "sandbox_unconfigured"}})
				return
			}
			pin = service.SandboxPin{ConfigID: strings.TrimSpace(agent.Config.SandboxConfigID), TenantID: agent.TenantID}
		}
		if err := h.previewStarter.EnsureSessionWorkspace(ctx, sessionID, pin); err != nil {
			status := "sandbox_start_failed"
			if errors.Is(err, sandbox.ErrPreviewUnsupported) {
				status = "unsupported"
			}
			c.JSON(200, gin.H{"success": true, "data": gin.H{"status": status}})
			return
		}
	}
	preview, err := h.workspace.DiscoverWebPreview(ctx, sessionID, request.Start)
	if err != nil {
		status := "error"
		if request.Start {
			status = "start_failed"
		}
		if errors.Is(err, sandbox.ErrNoLiveSessionSandbox) {
			status = "sandbox_not_running"
		}
		if errors.Is(err, sandbox.ErrSandboxPaused) {
			status = "paused"
		}
		if errors.Is(err, sandbox.ErrPreviewUnsupported) {
			status = "unsupported"
		}
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": status}})
		return
	}
	if preview.Status != "ready" {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": preview.Status, "detail": preview.Detail}})
		return
	}
	sandboxID, ok := h.workspace.BoundSandboxID(ctx, sessionID)
	if !ok {
		c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "sandbox_not_running"}})
		return
	}
	userID, _ := types.UserIDFromContext(ctx)
	tenantID, _ := types.TenantIDFromContext(ctx)
	if userID == "" || tenantID == 0 || h.previewLeases == nil {
		c.Status(http.StatusUnauthorized)
		return
	}
	token := request.PreviewID
	old, found := h.previewLeases.get(ctx, token)
	if !found || old.UserID != userID || old.TenantID != tenantID || old.SessionID != sessionID || old.SandboxID != sandboxID || old.Port != preview.Port || old.Secret != preview.Secret || old.AppOrigin != app.Scheme+"://"+app.Host || !strings.HasSuffix(old.Host, "."+base.Host) {
		var entropy [16]byte
		if _, err := rand.Read(entropy[:]); err != nil {
			c.Status(500)
			return
		}
		token = hex.EncodeToString(entropy[:])
	}
	base.Host = "wkpreview-" + token + "." + base.Host
	lease := previewLease{UserID: userID, TenantID: tenantID, SessionID: sessionID, SandboxID: sandboxID, Port: preview.Port, RelayPort: preview.RelayPort, Secret: preview.Secret, Host: base.Host, AppOrigin: app.Scheme + "://" + app.Host}
	if err := h.previewLeases.put(ctx, token, lease); err != nil {
		c.Status(503)
		return
	}
	c.JSON(200, gin.H{"success": true, "data": gin.H{"status": "ready", "url": base.String() + "/", "preview_id": token, "port": preview.Port}})
}
