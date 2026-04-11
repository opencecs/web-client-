package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

// WS 抓包诊断工具 v3：支持自定义 URL 参数
func main() {
	port := flag.Int("port", 30007, "容器投屏 WS 端口")
	rawURL := flag.String("url", "", "完整 WS URL（覆盖 port）")
	hold := flag.Duration("hold", 15*time.Second, "保持连接的时间")
	heartSec := flag.Int("heart", 1, "心跳间隔（秒），0=不发")
	flag.Parse()

	url := *rawURL
	if url == "" {
		url = fmt.Sprintf("ws://127.0.0.1:%d/lgcloud?user=hl&os=mobile&token=123&type=1&quality=1&platform=1&dm=0&width=1280&height=720", *port)
	}

	log.Printf("连接: %s", url)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Printf("连接失败: %v", err)
		if resp != nil {
			log.Printf("HTTP 状态: %d", resp.StatusCode)
		}
		os.Exit(1)
	}
	defer conn.Close()

	log.Printf("连接成功! HTTP %d", resp.StatusCode)

	closeCode := 0
	closeText := ""

	conn.SetCloseHandler(func(code int, text string) error {
		closeCode = code
		closeText = text
		log.Printf(">>> CLOSE 帧: code=%d text=%q", code, text)
		return nil
	})

	conn.SetPingHandler(func(data string) error {
		log.Printf(">>> PING: %q", data)
		conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
		return nil
	})

	// 心跳
	heartStop := make(chan struct{})
	if *heartSec > 0 {
		go func() {
			ticker := time.NewTicker(time.Duration(*heartSec) * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-heartStop:
					return
				case <-ticker.C:
					if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"id":"heart","data":"1"}`)); err != nil {
						log.Printf("心跳失败: %v", err)
						return
					}
				}
			}
		}()
	}

	// 读消息
	readDone := make(chan struct{})
	connectTime := time.Now()
	go func() {
		defer close(readDone)
		for {
			msgType, data, err := conn.ReadMessage()
			elapsed := time.Since(connectTime)
			if err != nil {
				if ce, ok := err.(*websocket.CloseError); ok {
					log.Printf(">>> [+%v] 关闭: code=%d reason=%q", elapsed.Round(time.Millisecond), ce.Code, ce.Text)
				} else {
					log.Printf(">>> [+%v] 读取错误: %v", elapsed.Round(time.Millisecond), err)
				}
				return
			}
			switch msgType {
			case websocket.TextMessage:
				var msg map[string]interface{}
				if json.Unmarshal(data, &msg) == nil {
					id, _ := msg["id"].(string)
					log.Printf(">>> [+%v] TEXT id=%s len=%d", elapsed.Round(time.Millisecond), id, len(data))
					if id == "offer" || id == "candidate" || id == "icecandiate" {
						d, _ := msg["data"].(string)
						if len(d) > 120 {
							d = d[:120] + "..."
						}
						log.Printf("    data=%s", d)
					} else {
						if len(data) < 200 {
							log.Printf("    raw=%s", string(data))
						}
					}
				} else {
					s := string(data)
					if len(s) > 200 {
						s = s[:200] + "..."
					}
					log.Printf(">>> [+%v] TEXT (非JSON) len=%d: %s", elapsed.Round(time.Millisecond), len(data), s)
				}
			case websocket.BinaryMessage:
				log.Printf(">>> [+%v] BINARY len=%d", elapsed.Round(time.Millisecond), len(data))
			}
		}
	}()

	select {
	case <-readDone:
		elapsed := time.Since(connectTime)
		log.Printf("连接持续时间: %v (close: %d %q)", elapsed.Round(time.Millisecond), closeCode, closeText)
	case <-time.After(*hold):
		log.Printf("保持 %v 后主动断开", *hold)
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		select {
		case <-readDone:
		case <-time.After(3 * time.Second):
		}
	case <-sigCh:
		log.Println("用户中断")
	}

	close(heartStop)
	conn.Close()
	log.Println("结束")
}
