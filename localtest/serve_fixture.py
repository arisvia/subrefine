import http.server
import socketserver

PORT = 18080


class Handler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        super().end_headers()

    def log_message(self, fmt, *args):
        pass


with socketserver.TCPServer(("127.0.0.1", PORT), Handler) as httpd:
    httpd.serve_forever()
