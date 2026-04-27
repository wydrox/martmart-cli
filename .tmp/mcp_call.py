#!/usr/bin/env python3
import json
import subprocess
import sys
from pathlib import Path

BIN = str(Path(__file__).resolve().parents[1] / 'server' / 'martmart')


def main():
    if len(sys.argv) < 2:
        print('usage: mcp_call.py TOOL_NAME [JSON_ARGS]', file=sys.stderr)
        sys.exit(2)
    tool_name = sys.argv[1]
    args = {}
    if len(sys.argv) >= 3 and sys.argv[2].strip():
        args = json.loads(sys.argv[2])

    proc = subprocess.Popen([BIN, 'mcp'], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, bufsize=1)
    next_id = 0

    def send(msg):
        proc.stdin.write(json.dumps(msg, separators=(',', ':')) + '\n')
        proc.stdin.flush()

    def read_msg():
        line = proc.stdout.readline()
        if not line:
            err = proc.stderr.read()
            raise RuntimeError(f'EOF from MCP server. stderr={err!r}')
        return json.loads(line)

    def call(method, params=None):
        nonlocal next_id
        next_id += 1
        req_id = next_id
        msg = {'jsonrpc': '2.0', 'id': req_id, 'method': method}
        if params is not None:
            msg['params'] = params
        send(msg)
        while True:
            msg = read_msg()
            if msg.get('id') == req_id:
                return msg
            # ignore notifications / other messages

    try:
        init = call('initialize', {
            'protocolVersion': '2025-06-18',
            'clientInfo': {'name': 'pi-local-debug', 'version': '0.1'},
            'capabilities': {},
        })
        send({'jsonrpc': '2.0', 'method': 'notifications/initialized', 'params': {}})
        result = call('tools/call', {'name': tool_name, 'arguments': args})
        print(json.dumps({'initialize': init, 'result': result}, ensure_ascii=False, indent=2))
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except Exception:
            proc.kill()


if __name__ == '__main__':
    main()
