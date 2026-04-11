//go:build linux

package main

import (
	"bytes"
	"log"
	"os"
)

func init() {
	checkAntiDebug()
}

// checkAntiDebug 检测是否被调试器附加
func checkAntiDebug() {
	// 检查 /proc/self/status 中的 TracerPid
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return
	}
	for _, line := range bytes.Split(data, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("TracerPid:")) {
			val := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("TracerPid:")))
			if !bytes.Equal(val, []byte("0")) {
				log.Fatal("[安全] 检测到调试器，拒绝启动")
			}
			break
		}
	}

	// 检查常见调试环境变量
	debugEnvs := []string{"LINES", "COLUMNS"}
	_ = debugEnvs // 预留

	// 检查 LD_PRELOAD（可能被注入）
	if os.Getenv("LD_PRELOAD") != "" {
		log.Fatal("[安全] 检测到 LD_PRELOAD 注入，拒绝启动")
	}
}
