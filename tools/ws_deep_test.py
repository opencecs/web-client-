#!/usr/bin/env python3
"""深度测试: 模拟完整 WebRTC 握手后断开, 看容器是否还能发 offer"""
import socket,struct,base64,os,time,json,sys

def ws_connect(port):
    s=socket.socket(); s.settimeout(10); s.connect(('127.0.0.1',port))
    k=base64.b64encode(os.urandom(16)).decode()
    path="/lgcloud?user=probe&os=mobile&type=1&quality=1&platform=1&dm=0&width=1280&height=720"
    s.sendall(f"GET {path} HTTP/1.1\r\nHost: 127.0.0.1:{port}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: {k}\r\nSec-WebSocket-Version: 13\r\n\r\n".encode())
    r=b""
    while b"\r\n\r\n" not in r: r+=s.recv(4096)
    if b"101" not in r.split(b"\r\n")[0]: raise Exception("WS fail")
    return s

def ws_send(s,text):
    data=text.encode(); mask=os.urandom(4); h=bytearray([0x81])
    l=len(data)
    if l<126: h.append(0x80|l)
    elif l<65536: h.append(0x80|126); h+=struct.pack("!H",l)
    else: h.append(0x80|127); h+=struct.pack("!Q",l)
    h+=mask
    s.sendall(h+bytearray(data[i]^mask[i%4] for i in range(l)))

def ws_recv_msg(s,timeout=5):
    """返回 (opcode, payload_bytes) 或 (None, None)"""
    s.settimeout(timeout)
    try:
        hdr=s.recv(2)
        if len(hdr)<2: return None,None
        op=hdr[0]&0x0F; ln=hdr[1]&0x7F
        if ln==126: ln=struct.unpack("!H",s.recv(2))[0]
        elif ln==127: ln=struct.unpack("!Q",s.recv(8))[0]
        p=bytearray()
        while len(p)<ln:
            c=s.recv(ln-len(p))
            if not c: break
            p+=c
        return op,bytes(p)
    except socket.timeout: return -1,None
    except: return None,None

def recv_all_msgs(s,timeout=3):
    """收集所有消息直到超时"""
    msgs=[]
    while True:
        op,p = ws_recv_msg(s,timeout)
        if op==-1: break  # timeout
        if op is None: break  # closed
        if op==1:
            try: msgs.append(json.loads(p.decode()))
            except: msgs.append({"raw":p[:100]})
        elif op==8: msgs.append({"id":"WS_CLOSE"}); break
    return msgs

def decode_offer_sdp(offer_data_b64):
    """解码 offer 的 SDP 内容"""
    decoded = base64.b64decode(offer_data_b64)
    txt = decoded.decode("utf-8","replace")
    try:
        wrap = json.loads(txt)
        return wrap.get("sdp", txt)
    except:
        return txt

def make_fake_answer(offer_sdp):
    """基于 offer SDP 构造一个最小的 answer SDP"""
    lines = []
    for line in offer_sdp.replace("\r\n","\n").split("\n"):
        l = line.strip()
        if l.startswith("a=setup:"):
            # offer 是 actpass, answer 应该是 active
            lines.append("a=setup:active")
        elif l.startswith("a=sendrecv"):
            lines.append("a=recvonly")
        elif l.startswith("a=candidate:"):
            continue  # 不发候选
        elif l.startswith("a=end-of-candidates"):
            continue
        else:
            lines.append(l)
    sdp_str = "\r\n".join(lines) + "\r\n"
    answer_json = json.dumps({"type":"answer","sdp":sdp_str})
    return base64.b64encode(answer_json.encode()).decode()

def test_slot(slot, label=""):
    port = 30000 + (slot-1)*100 + 7
    print(f"\n{'='*50}")
    print(f"坑位 {slot} (port {port}) {label}")
    print(f"{'='*50}")

    # 第一步: 连接并获取 offer
    print("\n[步骤1] 连接并获取 offer...")
    s = ws_connect(port)
    msgs = recv_all_msgs(s, 3)
    offer_msg = None
    for m in msgs:
        mid = m.get("id","")
        print(f"  收到: id={mid}")
        if mid == "offer":
            offer_msg = m
    if not offer_msg:
        print("  !! 没有收到 offer, 容器可能卡死")
        s.close()
        return False

    # 解码 offer SDP
    sdp = decode_offer_sdp(offer_msg["data"])
    # 提取关键信息
    ufrag = ""
    for line in sdp.split("\n"):
        if "ice-ufrag:" in line:
            ufrag = line.split("ice-ufrag:")[1].strip()
            break
    print(f"  offer ice-ufrag: {ufrag}")
    print(f"  offer SDP 长度: {len(sdp)} bytes")

    # 第二步: 发送 fake answer
    print("\n[步骤2] 发送 fake SDP answer...")
    fake_answer_b64 = make_fake_answer(sdp)
    answer_msg = json.dumps({"id":"answer","data":fake_answer_b64})
    ws_send(s, answer_msg)
    print(f"  answer 已发送 ({len(fake_answer_b64)} bytes)")

    # 第三步: 保持连接, 发心跳, 观察容器行为
    print("\n[步骤3] 保持连接 10 秒 (模拟预热池)...")
    start = time.time()
    while time.time()-start < 10:
        ws_send(s, '{"id":"heart","data":"1"}')
        op,p = ws_recv_msg(s, 2)
        if op==1:
            try:
                m=json.loads(p.decode())
                print(f"  [{time.time()-start:.1f}s] 收到: id={m.get('id','?')}")
            except: pass
        elif op==8:
            print(f"  [{time.time()-start:.1f}s] 收到 WS close!")
            break
        elif op is None:
            print(f"  [{time.time()-start:.1f}s] 连接断开!")
            break

    # 第四步: 断开
    print("\n[步骤4] 断开连接...")
    s.close()
    time.sleep(2)

    # 第五步: 重新连接, 看是否还能收到 offer
    print("\n[步骤5] 重新连接, 检查是否还能收到 offer...")
    try:
        s2 = ws_connect(port)
        msgs2 = recv_all_msgs(s2, 5)
        got_offer = False
        for m in msgs2:
            mid = m.get("id","")
            print(f"  收到: id={mid}")
            if mid == "offer": got_offer = True
        s2.close()
        if got_offer:
            print("\n  >>> 重连后仍能收到 offer (容器正常) <<<")
        else:
            print("\n  >>> 重连后没有 offer (容器卡死!) <<<")
        return got_offer
    except Exception as e:
        print(f"  重连失败: {e}")
        return False

if __name__=="__main__":
    slots = [int(x) for x in sys.argv[1:]] if len(sys.argv)>1 else [1, 3]
    results = {}
    for sl in slots:
        results[sl] = test_slot(sl)
        time.sleep(2)

    print(f"\n{'='*50}")
    print("总结:")
    for sl,ok in results.items():
        print(f"  坑位 {sl}: {'正常' if ok else '卡死!'}")
