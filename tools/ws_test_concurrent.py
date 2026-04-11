#!/usr/bin/env python3
"""测试: 1)容器是否允许并发WS连接  2)回答SDP后断开,再连是否还发offer"""
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
    if b"101" not in resp.split(b"\r\n")[0]:
        raise Exception(f"WS fail")
    return s

def ws_send(s,text):
    data=text.encode(); mask=os.urandom(4)
    h=bytearray([0x81]); l=len(data)
    if l<126: h.append(0x80|l)
    else: h.append(0x80|126); h+=struct.pack("!H",l)
    h+=mask
    s.sendall(h+bytearray(data[i]^mask[i%4] for i in range(l)))

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
                try: m=json.loads(p.decode()); msgs.append(m.get("id","?"))
                except: msgs.append("?")
            elif op==8: msgs.append("CLOSE"); break
        except: break
    return msgs

port=30007
print("=== 测试1: 并发连接 ===")
print("连接A...")
a=ws_connect(port)
ids_a=ws_recv_ids(a,3)
print(f"  A收到: {ids_a}")
print("连接B (A还未断开)...")
try:
    b=ws_connect(port)
    ids_b=ws_recv_ids(b,3)
    print(f"  B收到: {ids_b}")
    b.close()
except Exception as e:
    print(f"  B连接失败: {e}")
# 检查A是否被踢
print("检查A是否还活着...")
a.settimeout(2)
try:
    hdr=a.recv(2)
    if len(hdr)<2: print("  A已断开(被踢)")
    else: print(f"  A还活着, 收到数据")
except socket.timeout:
    print("  A还活着(无新数据)")
except: print("  A已断开")
a.close()
time.sleep(1)

print("\n=== 测试2: 不断开WS情况下容器多久关闭连接 ===")
s=ws_connect(port)
ids=ws_recv_ids(s,2)
print(f"  初始消息: {ids}")
# 发心跳保活,看容器多久断
start=time.time()
for i in range(30):
    ws_send(s,'{"id":"heart","data":"1"}')
    s.settimeout(2)
    try:
        hdr=s.recv(2)
        if len(hdr)<2:
            print(f"  容器在 {time.time()-start:.1f}s 后关闭连接")
            break
        op=hdr[0]&0x0F; ln=hdr[1]&0x7F
        if ln==126: ln=struct.unpack("!H",s.recv(2))[0]
        elif ln==127: ln=struct.unpack("!Q",s.recv(8))[0]
        s.recv(ln)
        if op==8:
            print(f"  容器在 {time.time()-start:.1f}s 后发送close帧")
            break
    except socket.timeout:
        pass
else:
    print(f"  心跳保活 60s 后容器仍未关闭")
s.close()
print("done")
