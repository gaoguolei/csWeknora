"""Guest-only authenticated HTTP/WebSocket relay to one fixed project port."""
import hmac
import http.server
import json
import select
import socket
import sys

class Relay(http.server.BaseHTTPRequestHandler):
    rbufsize = 0
    def handle(self):
        self.connection.settimeout(30)
        self.raw_requestline = self.rfile.readline(65537)
        if len(self.raw_requestline) > 65536 or not self.parse_request():
            return
        if not hmac.compare_digest(self.headers.get('X-WeKnora-Preview-Relay', ''), self.server.secret):
            self.send_error(403)
            return
        if not self.path.startswith('/') or self.path.startswith('//') or self.command == 'CONNECT':
            self.send_error(400)
            return
        upgrade = self.headers.get('Upgrade', '').lower() == 'websocket'
        with socket.create_connection(('127.0.0.1', self.server.target_port), timeout=15) as upstream:
            headers = [f'{self.command} {self.path} HTTP/1.1', f'Host: localhost:{self.server.target_port}']
            for key, value in self.headers.items():
                if key.lower() not in ('host', 'x-weknora-preview-relay', 'e2b-traffic-access-token', 'connection'):
                    headers.append(f'{key}: {value}')
            headers.append('Connection: Upgrade' if upgrade else 'Connection: close')
            upstream.sendall(('\r\n'.join(headers) + '\r\n\r\n').encode('latin1'))
            upstream.settimeout(30)
            sockets = [upstream, self.connection]
            while True:
                ready, _, _ = select.select(sockets, [], [], 90)
                if not ready:
                    break
                for source in ready:
                    data = source.recv(65536)
                    if not data:
                        return
                    (self.connection if source is upstream else upstream).sendall(data)
    def log_message(self, *args):
        pass

if __name__ == '__main__':
    config = json.loads(sys.argv[1])
    server = http.server.ThreadingHTTPServer(('0.0.0.0', config['relay_port']), Relay)
    server.daemon_threads = True
    server.target_port = config['port']
    server.secret = config['secret']
    server.serve_forever()
