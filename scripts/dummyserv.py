from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs
import os

serverHost = 'localhost'
serverPort = 8081

RESULTS = {
    'malý':     'adjective_response.html',
    'nahoře':   'adverb_response.html',
    'nebo':     'conjunction_response.html',
    'haló':     'interjection_response.html',
    'okolnost': 'noun_response.html',
    'sto':      'numeral_response.html',
    'ať':       'particle_response.html',
    'vedle_1':  'preposition_response.html',
    'se':       'pronoun_response.html',
    'dělat':    'verb_response.html'
}

class MyServer(BaseHTTPRequestHandler):

    doc_path: str

    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-type", "text/html")
        self.end_headers()
        args = parse_qs(self.path[2:])
        resp_file = RESULTS.get(args.get('slovo', [None])[0])
        if resp_file:
            with open(os.path.join(self.base_path, resp_file)) as fr:
                self.wfile.write(bytes(fr.read(), 'utf-8'))

        self.wfile.write(b'other resource')

if __name__ == '__main__':
    MyServer.base_path = os.path.join(os.path.dirname(__file__), '..', 'testdata', 'lguide')
    webServer = HTTPServer((serverHost, serverPort), MyServer)
    print(f'Server running at http://{serverHost}:{serverPort}')
    try:
        webServer.serve_forever()
    except KeyboardInterrupt:
        pass
    webServer.server_close()
    print('Server stopped')