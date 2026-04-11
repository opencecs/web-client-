#!/usr/bin/env python3
import socket,struct,base64,os,time,json,sys

def wc(p):
    s=socket.socket(); s.settimeout(10); s.connect(('127.0.0.1',p))
    k=base64.b64encode(os.urandom(16)).decode()
    s.sendall(f"GET /lgcloud?user=probe&os=mobile&type=1&quality=1&platform=1&dm=0&width=1280&height=720 HTTP/1.1\r\nHost: 127.0.0.1:{p}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: {k}\r\nSec-WebSocket-Version: 13\r\n\r\n".encode())
    r=b""
    while b"\r\n\r\n" not in r: r+=s.recv(4096)
    return s

def wr(s,t=3):
    ids=[]; s.settimeout(t)
    while 1:
        try:
            h=s.recv(2)
            if len(h)<2: break
            l=h[1]&0x7F
            if l==126: l=struct.unpack("!H",s.recv(2))[0]
            elif l==127: l=struct.unpack("!Q",s.recv(8))[0]
            p=s.recv(l) if l else b""
            if h[0]&0xF==1:
                try: ids.append(json.loads(p).get("id","?"))
                except: pass
            if h[0]&0xF==8: ids.append("CLOSE"); break
        except: break
    return ids

# 测试1: 并发连接
print("=== 并发WS测试(坑位1) ===")
a=wc(30007); ia=wr(a); print(f"A: {ia}")
try:
    b=wc(30007); ib=wr(b); print(f"B: {ib}"); b.close()
except Exception as e: print(f"B: FAIL {e}")
a.close(); time.sleep(1)

# 测试2: 坑位3重启后
print("\n=== 坑位3 offer测试 ===")
s=wc(30207); i3=wr(s,5); print(f"slot3: {i3}"); s.close()
