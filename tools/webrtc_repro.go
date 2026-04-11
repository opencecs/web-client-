// 复现测试: 用 pion/webrtc 建立真实 WebRTC 后断开, 检查容器是否卡死
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

func main() {
	slot := 3
	if len(os.Args) > 1 {
		slot, _ = strconv.Atoi(os.Args[1])
	}
	port := 30000 + (slot-1)*100 + 7
	url := fmt.Sprintf("ws://127.0.0.1:%d/lgcloud?user=test&os=mobile&type=1&quality=1&platform=1&dm=0&width=1280&height=720", port)

	fmt.Printf("=== 坑位 %d (端口 %d) 真实 WebRTC 测试 ===\n", slot, port)

	// 步骤1: 连接 WS, 获取 offer, 用 pion 回答, 保持 10 秒
	fmt.Println("\n[步骤1] 连接并建立真实 WebRTC...")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("WS连接失败:", err)
	}

	var offerSDP string
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for i := 0; i < 10; i++ {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg struct {
			ID   string `json:"id"`
			Data string `json:"data"`
		}
		json.Unmarshal(data, &msg)
		fmt.Printf("  收到: id=%s\n", msg.ID)
		if msg.ID == "offer" {
			decoded, _ := base64.StdEncoding.DecodeString(msg.Data)
			var wrap struct{ SDP string `json:"sdp"` }
			if json.Unmarshal(decoded, &wrap) == nil && wrap.SDP != "" {
				offerSDP = wrap.SDP
			} else {
				offerSDP = string(decoded)
			}
		}
	}
	if offerSDP == "" {
		log.Fatal("没收到 offer!")
	}
	conn.SetReadDeadline(time.Time{}) // 清除 deadline

	// 提取 ufrag
	for _, line := range strings.Split(offerSDP, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "a=ice-ufrag:") {
			fmt.Printf("  offer ufrag: %s\n", strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "a=ice-ufrag:")))
			break
		}
	}

	// 创建 pion PeerConnection
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Fatal("创建 PC 失败:", err)
	}

	pc.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		fmt.Printf("  ICE状态: %s\n", state.String())
	})

	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offerSDP}
	if err := pc.SetRemoteDescription(offer); err != nil {
		log.Fatal("SetRemoteDescription:", err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		log.Fatal("CreateAnswer:", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		log.Fatal("SetLocalDescription:", err)
	}

	answerJSON, _ := json.Marshal(map[string]string{"type": "answer", "sdp": answer.SDP})
	encoded := base64.StdEncoding.EncodeToString(answerJSON)
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"id":"answer","data":"%s"}`, encoded)))
	fmt.Println("  真实 SDP answer 已发送")

	// 步骤2: 保持 30 秒, 观察 ICE 是否连通
	fmt.Println("\n[步骤2] 保持 WebRTC 30 秒...")
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(1 * time.Second)
	for i := 0; i < 30; i++ {
		<-ticker.C
		conn.WriteMessage(websocket.TextMessage, []byte(`{"id":"heart","data":"1"}`))
		if i%5 == 4 {
			fmt.Printf("  %ds...\n", i+1)
		}
	}
	ticker.Stop()

	// 步骤3: 关闭 PC 和 WS
	fmt.Println("\n[步骤3] 关闭 PeerConnection 和 WebSocket...")
	pc.Close()
	conn.Close()
	fmt.Println("  已关闭, 等待 2 秒...")
	time.Sleep(2 * time.Second)

	// 步骤4: 重连检查
	fmt.Println("\n[步骤4] 重新连接, 检查 offer...")
	conn2, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		fmt.Printf("  重连失败: %v\n", err)
		fmt.Println("\n结果: 容器可能卡死!")
		return
	}
	conn2.SetReadDeadline(time.Now().Add(5 * time.Second))
	gotOffer := false
	for {
		_, data, err := conn2.ReadMessage()
		if err != nil {
			break
		}
		var msg struct{ ID string `json:"id"` }
		json.Unmarshal(data, &msg)
		fmt.Printf("  收到: id=%s\n", msg.ID)
		if msg.ID == "offer" {
			gotOffer = true
		}
	}
	conn2.Close()

	if gotOffer {
		fmt.Println("\n结果: 容器正常 (重连后收到 offer)")
	} else {
		fmt.Println("\n结果: 容器卡死! (重连后没有 offer)")
	}
}
