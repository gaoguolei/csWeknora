package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// Platform KB defaults are a single global document (system_settings
// "knowledge_base.default_config") shared by every workspace, but the concrete
// storage backend / vector store instances it may name are tenant-scoped rows
// owned by the workspace the operations admin configured it from. Applying such
// an ID verbatim to a KB created in another workspace made the tenant-scoped
// resolvers reject the whole create request ("storage backend is unavailable"),
// which is what these tests pin down: an unresolvable binding must be dropped so
// the caller falls back to its own workspace default, while an ID the workspace
// actually owns (or that is not configured at all) keeps working as before.

type stubSystemSettingService struct {
	interfaces.SystemSettingService
	raw string
}

func (s *stubSystemSettingService) Get(_ context.Context, key string) (*types.SystemSetting, error) {
	if key != types.KBDefaultsKey {
		return nil, nil
	}
	encoded, err := json.Marshal(s.raw)
	if err != nil {
		return nil, err
	}
	return &types.SystemSetting{
		Key:       key,
		ValueType: "string",
		Value:     types.JSON(encoded),
	}, nil
}

// stubStorageResolver resolves only the IDs listed in owned and reports the
// remaining ones as missing, mirroring the tenant-scoped repository lookup.
type stubStorageResolver struct {
	interfaces.StorageBackendResolver
	owned map[string]*types.StorageBackend
}

func (s *stubStorageResolver) ResolveBackend(
	_ context.Context, _ *types.Tenant, backendID, _ string,
) (*types.StorageBackend, error) {
	if backend, ok := s.owned[backendID]; ok {
		return backend, nil
	}
	return nil, errors.New("storage backend not found")
}

type stubVectorStoreService struct {
	interfaces.VectorStoreService
	owned map[string]types.StoreDisplay
}

func (s *stubVectorStoreService) ResolveStoreView(
	_ context.Context, _ uint64, storeID string,
) (types.StoreDisplay, error) {
	if view, ok := s.owned[storeID]; ok {
		return view, nil
	}
	return types.UnavailableStoreDisplay(), nil
}

func newPlatformDefaultsContext(tenantID uint64) context.Context {
	return context.WithValue(
		context.Background(), types.TenantInfoContextKey, &types.Tenant{ID: tenantID},
	)
}

func TestApplyKBPlatformDefaults_DropsForeignStorageBackend(t *testing.T) {
	h := &KnowledgeBaseHandler{
		systemSettingSvc: &stubSystemSettingService{raw: `{"storage_backend_id":"foreign-backend","storage_provider_config":{"provider":"local"}}`},
		storageResolver:  &stubStorageResolver{},
	}
	kb := &types.KnowledgeBase{Type: types.KnowledgeBaseTypeDocument}

	h.applyKBPlatformDefaults(newPlatformDefaultsContext(10002), kb)

	if kb.StorageBackendID != nil {
		t.Fatalf("foreign platform storage backend must be dropped, got %q", *kb.StorageBackendID)
	}
	// The provider names a storage type, so it survives the ID being dropped and
	// lets the workspace resolve its own instance of that type.
	if kb.GetStorageProvider() != "local" {
		t.Fatalf("expected provider projection to survive, got %q", kb.GetStorageProvider())
	}
}

func TestApplyKBPlatformDefaults_KeepsOwnedStorageBackend(t *testing.T) {
	owned := &types.StorageBackend{ID: "own-backend", TenantID: 10002, Provider: "local"}
	h := &KnowledgeBaseHandler{
		systemSettingSvc: &stubSystemSettingService{raw: `{"storage_backend_id":"own-backend"}`},
		storageResolver:  &stubStorageResolver{owned: map[string]*types.StorageBackend{"own-backend": owned}},
	}
	kb := &types.KnowledgeBase{Type: types.KnowledgeBaseTypeDocument}

	h.applyKBPlatformDefaults(newPlatformDefaultsContext(10002), kb)

	if kb.StorageBackendID == nil || *kb.StorageBackendID != "own-backend" {
		t.Fatalf("owned platform storage backend must be applied, got %v", kb.StorageBackendID)
	}
}

func TestApplyKBPlatformDefaults_DropsForeignVectorStore(t *testing.T) {
	h := &KnowledgeBaseHandler{
		systemSettingSvc:   &stubSystemSettingService{raw: `{"vector_store_id":"foreign-store"}`},
		vectorStoreService: &stubVectorStoreService{},
	}
	kb := &types.KnowledgeBase{Type: types.KnowledgeBaseTypeDocument}

	h.applyKBPlatformDefaults(newPlatformDefaultsContext(10002), kb)

	if kb.VectorStoreID != nil {
		t.Fatalf("foreign platform vector store must be dropped, got %q", *kb.VectorStoreID)
	}
}

func TestApplyKBPlatformDefaults_KeepsOwnedVectorStore(t *testing.T) {
	h := &KnowledgeBaseHandler{
		systemSettingSvc: &stubSystemSettingService{raw: `{"vector_store_id":"own-store"}`},
		vectorStoreService: &stubVectorStoreService{owned: map[string]types.StoreDisplay{
			"own-store": {Name: "prod", Source: types.StoreSourceUser, Status: "available"},
		}},
	}
	kb := &types.KnowledgeBase{Type: types.KnowledgeBaseTypeDocument}

	h.applyKBPlatformDefaults(newPlatformDefaultsContext(10002), kb)

	if kb.VectorStoreID == nil || *kb.VectorStoreID != "own-store" {
		t.Fatalf("owned platform vector store must be applied, got %v", kb.VectorStoreID)
	}
}

func TestApplyKBPlatformDefaults_NoBindingsConfigured(t *testing.T) {
	h := &KnowledgeBaseHandler{
		systemSettingSvc: &stubSystemSettingService{raw: `{"summary_model_id":"llm-1"}`},
		storageResolver:  &stubStorageResolver{},
	}
	kb := &types.KnowledgeBase{Type: types.KnowledgeBaseTypeDocument}

	h.applyKBPlatformDefaults(newPlatformDefaultsContext(10002), kb)

	if kb.StorageBackendID != nil || kb.VectorStoreID != nil {
		t.Fatalf("unconfigured bindings must stay empty, got backend=%v store=%v", kb.StorageBackendID, kb.VectorStoreID)
	}
	if kb.SummaryModelID != "llm-1" {
		t.Fatalf("other platform defaults must still apply, got %q", kb.SummaryModelID)
	}
}
