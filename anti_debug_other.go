//go:build !linux

package main

// checkAntiDebug 非 Linux 平台不做反调试检测
func checkAntiDebug() {}
