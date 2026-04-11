package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	checkIntegrity()
}

// checkIntegrity 校验自身二进制完整性，防止被篡改
// 读取与二进制同目录下的 .sha256 校验文件进行比对
func checkIntegrity() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	// 解析符号链接
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return
	}

	// 读取校验文件（与二进制同名 + .sha256 后缀）
	hashFile := exePath + ".sha256"
	hashData, err := os.ReadFile(hashFile)
	if err != nil {
		// 校验文件不存在则跳过（开发模式）
		return
	}
	expectedHash := strings.TrimSpace(string(hashData))
	// sha256sum 输出格式为 "hash  filename"，只取 hash 部分
	if parts := strings.Fields(expectedHash); len(parts) > 0 {
		expectedHash = parts[0]
	}
	if expectedHash == "" {
		return
	}

	// 计算自身 SHA256
	f, err := os.Open(exePath)
	if err != nil {
		log.Fatal("[安全] 无法读取可执行文件")
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal("[安全] 计算校验和失败")
	}

	actualHash := hex.EncodeToString(h.Sum(nil))
	if actualHash != expectedHash {
		fmt.Fprintf(os.Stderr, "[安全] 二进制完整性校验失败，程序可能被篡改\n")
		os.Exit(1)
	}
}
