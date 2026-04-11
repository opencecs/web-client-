#!/usr/bin/env python3
"""WebSocket probe for container projection"""
import socket,struct,base64,os,sys,time,json

def ws_connect(host,port,path):
    s=socket.socket(socket.AF_INET,socket.SOCK_STREAM)
    s.settimeout(10)
    s.connect((host,port))
    key=base64.b64encode(os.urandom(16)).decode()
    req=f"GET {path} HTTP/1.1\r\nHost: {host}:{port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n"
    s.sendall(req.encode())
    resp=b""
    while b"\r\n\r\n" not in resp:
        resp+=s.recv(4096)
    if b"101" not in resp.split(b"\r\n")[0]:
        raise Exception(f"WS fail: {resp[:200]}")
    return s

def ws_send(s,text):
    data=text.encode()
    mask=os.urandom(4)
    h=bytearray([0x81])
    l=len(data)
    if l<126: h.append(0x80|l)
    elif l<65536: h.append(0x80|126); h+=struct.pack("!H",l)
    else: h.append(0x80|127); h+=struct.pack("!Q",l)
    h+=mask
    s.sendall(h+bytearray(data[i]^mask[i%4] for i in range(l)))

def ws_recv(s,timeout=5):
    s.settimeout(timeout)
    try:
        hdr=s.recv(2)
        if len(hdr)<2: return None,None
        op=hdr[0]&0x0F; ml=(hdr[1]&0x80)!=0; ln=hdr[1]&0x7F
        if ln==126: ln=struct.unpack("!H",s.recv(2))[0]
        elif ln==127: ln=struct.unpack("!Q",s.recv(8))[0]
        mk=s.recv(4) if ml else None
        p=bytearray()
        while len(p)<ln:
            c=s.recv(ln-len(p))
            if not c: break
            p+=c
        if mk: p=bytearray(p[i]^mk[i%4] for i in range(len(p)))
        return op,bytes(p)
    except socket.timeout: return -1,None

def probe(slot,dur=8):
    port=30000+(slot-1)*100+7
    path="/lgcloud?user=probe&os=mobile&type=1&quality=1&platform=1&dm=0&width=1280&height=720"
    print(f"\n{'='*50}\nProbe slot {slot} (port {port})\n{'='*50}")
    try:
        s=ws_connect("127.0.0.1",port,path)
        print(f"[{time.strftime('%H:%M:%S')}] WS connected")
    except Exception as e:
        print(f"[{time.strftime('%H:%M:%S')}] Connect failed: {e}"); return
    ws_send(s,'{"id":"heart","data":"1"}')
    t0=time.time(); n=0; got_offer=False
    while time.time()-t0<dur:
        op,pl=ws_recv(s,2)
        if op==-1: ws_send(s,'{"id":"heart","data":"1"}'); continue
        if op is None: print("Connection closed"); break
        n+=1
        if op==1:
            txt=pl.decode('utf-8',errors='replace')
            try:
                m=json.loads(txt); mid=m.get("id","?"); dd=str(m.get("data",""))[:60]
                print(f"[{time.strftime('%H:%M:%S')}] msg#{n}: id={mid} data={dd}")
                if mid=="offer": got_offer=True; print("  >>> OFFER RECEIVED <<<")
            except: print(f"[{time.strftime('%H:%M:%S')}] msg#{n}: text({len(txt)}B)")
        elif op==8: print("Close frame"); break
        else: print(f"[{time.strftime('%H:%M:%S')}] msg#{n}: op={op} ({len(pl)}B)")
    s.close()
    print(f"Result: {n} msgs, offer={'YES' if got_offer else 'NO'}")

if __name__=="__main__":
    slots=[int(x) for x in sys.argv[1:]] if len(sys.argv)>1 else [1,3]
    for sl in slots: probe(sl); time.sleep(1)
