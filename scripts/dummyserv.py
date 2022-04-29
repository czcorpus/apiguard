from http.server import BaseHTTPRequestHandler, HTTPServer
import sys

serverHost = 'localhost'
serverPort = 8081

class MyServer(BaseHTTPRequestHandler):

    doc_path: str

    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-type", "text/html")
        self.end_headers()
        with open(self.doc_path) as fr:
            self.wfile.write(bytes(fr.read(), 'utf-8'))

if __name__ == '__main__':
    MyServer.doc_path = sys.argv[1]
    webServer = HTTPServer((serverHost, serverPort), MyServer)
    print(f'Server running at http://{serverHost}:{serverPort}')
    try:
        webServer.serve_forever()
    except KeyboardInterrupt:
        pass
    webServer.server_close()
    print('Server stopped')