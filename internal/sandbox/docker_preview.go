package sandbox

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

// Engine exec provides a byte tunnel without publishing container ports or
// depending on host reachability of bridge IPs (including Docker Desktop).
const dockerPreviewTunnel = `import os,select,socket,sys
s=socket.create_connection(('127.0.0.1',int(sys.argv[1])),15)
s.settimeout(None)
while True:
 ready,_,_=select.select([s,0],[],[],90)
 if not ready: break
 for source in ready:
  data=s.recv(65536) if source is s else os.read(0,65536)
  if not data: sys.exit(0)
  if source is s:
   sys.stdout.buffer.write(data);sys.stdout.buffer.flush()
  else: s.sendall(data)
`

type dockerPreviewConn struct {
	net.Conn
	reader    *io.PipeReader
	closeFunc func()
	once      sync.Once
}

func (c *dockerPreviewConn) Read(b []byte) (int, error) { return c.reader.Read(b) }
func (c *dockerPreviewConn) Close() error {
	c.once.Do(func() { c.closeFunc(); _ = c.reader.Close() })
	return nil
}

func (c *DockerRemoteClient) RoundTripPreview(ctx context.Context, handle RemoteSandboxHandle, port int, req *http.Request) (*http.Response, error) {
	if handle == nil || !ValidProjectPort(port) {
		return nil, ErrPreviewUnsupported
	}
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(dialCtx context.Context, _, _ string) (net.Conn, error) {
		created, err := c.api.ExecCreate(dialCtx, handle.ID(), client.ExecCreateOptions{Cmd: []string{"python3", "-u", "-c", dockerPreviewTunnel, strconv.Itoa(port)}, AttachStdin: true, AttachStdout: true, AttachStderr: true, User: DefaultSandboxExecUser})
		if err != nil {
			return nil, err
		}
		attached, err := c.api.ExecAttach(dialCtx, created.ID, client.ExecAttachOptions{})
		if err != nil {
			return nil, err
		}
		reader, writer := io.Pipe()
		conn := &dockerPreviewConn{Conn: attached.Conn, reader: reader, closeFunc: attached.Close}
		go func() { _, err := stdcopy.StdCopy(writer, io.Discard, attached.Reader); _ = writer.CloseWithError(err) }()
		return conn, nil
	}}
	out := req.Clone(ctx)
	out.URL.Scheme = "http"
	out.URL.Host = "localhost:" + strconv.Itoa(port)
	out.Host = out.URL.Host
	out.RequestURI = ""
	return transport.RoundTrip(out)
}
