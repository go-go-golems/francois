#!/usr/bin/env python3
"""One-request, loopback-only gate credentials for a browser validation context.
Never logs responses. Browser must restrict HTTP credentials to sandbox origin.
"""
import http.server,json,subprocess
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  if self.path!='/gate-7ad90d9e-931a-40b6-b2f1-c8f3e9c7b6ce':self.send_error(404);return
  d=json.loads(subprocess.check_output(['vault','kv','get','-format=json','kv/apps/francois/sandbox/ingress']))['data']['data']
  self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers()
  self.wfile.write(json.dumps({'username':d['username'],'password':d['password'],'origin':'https://sandbox.slop.boutique'}).encode())
s=http.server.HTTPServer(('127.0.0.1',18770),Handler);s.timeout=90
try:s.handle_request()
finally:s.server_close()
