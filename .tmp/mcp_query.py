#!/usr/bin/env python3
import json, subprocess, sys
from pathlib import Path

BIN = str(Path(__file__).resolve().parents[1] / 'server' / 'martmart')

class MCP:
    def __init__(self):
        self.proc = subprocess.Popen([BIN, 'mcp'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
        self.next_id = 0
        self.call('initialize', {
            'protocolVersion': '2025-06-18',
            'clientInfo': {'name': 'pi-local-query', 'version': '0.1'},
            'capabilities': {},
        })
        self.send({'jsonrpc': '2.0', 'method': 'notifications/initialized', 'params': {}})
    def send(self, msg):
        self.proc.stdin.write(json.dumps(msg, separators=(',', ':')) + '\n')
        self.proc.stdin.flush()
    def read_msg(self):
        line = self.proc.stdout.readline()
        if not line:
            raise RuntimeError(f'EOF stderr={self.proc.stderr.read()!r}')
        return json.loads(line)
    def call(self, method, params=None):
        self.next_id += 1
        rid = self.next_id
        msg = {'jsonrpc': '2.0', 'id': rid, 'method': method}
        if params is not None:
            msg['params'] = params
        self.send(msg)
        while True:
            msg = self.read_msg()
            if msg.get('id') == rid:
                return msg
    def tool(self, name, arguments=None):
        res = self.call('tools/call', {'name': name, 'arguments': arguments or {}})
        return res
    def close(self):
        self.proc.terminate()
        try:
            self.proc.wait(timeout=2)
        except Exception:
            self.proc.kill()

if __name__ == '__main__':
    m = MCP()
    try:
        tool = sys.argv[1]
        args = json.loads(sys.argv[2]) if len(sys.argv) > 2 else {}
        print(json.dumps(m.tool(tool, args), ensure_ascii=False, indent=2))
    finally:
        m.close()
