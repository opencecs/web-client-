#!/usr/bin/env python3
"""测试: 不回答offer后重连, 容器是否还会发offer"""
import socket,struct,base64,os,time,json

def ws_connect(port):
    s=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
    s.settimeout(10)
    s.connect(('127.0.0.1',port))
    key=base64.b64encode(os.urandom(16)).decode()
    path="/lgcloud?user=test&os=mobile&type=1&quality=1&platform=1&dm=0&width=1280&height=720"
    req=f"GET {path} HTTP/1.1\r\nHost: 127.0.0.1:{port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n"
    s.sendall(req.encode())
    resp=b""
    while b"\r\n\r\n" not in resp: resp+=s.recv(4096)
    return s

def ws_recv_ids(s,timeout=3):
    msgs=[]
    s.settimeout(timeout)
    while True:
        try:
            hdr=s.recv(2)
            if len(hdr)<2: break
            op=hdr[0]&0x0F; ln=hdr[1]&0x7F
            if ln==126: ln=struct.unpack("!H",s.recv(2))[0]
            elif ln==127: ln=struct.unpack("!Q",s.recv(8))[0]
            p=bytearray()
            while len(p)<ln:
                c=s.recv(ln-len(p))
                if not c: break
                p+=c
            if op==1:
                try:
                    m=json.loads(p.decode())
                    msgs.append(m.get("id","?"))
                except: msgs.append("parse_err")
        except: break
    return msgs

port=30007
print("=== 坑位1: 不回答offer后重连测试 ===")
for i in range(3):
    print(f"\n第{i+1}次连接...")
    s=ws_connect(port)
    ids=ws_recv_ids(s,3)
    has_offer="offer" in ids
    print(f"  消息IDs: {ids}")
    print(f"  offer: {'YES' if has_offer else 'NO'}")
    s.close()
    time.sleep(1)
print("\ndone")
