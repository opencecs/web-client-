//go:build !linux

package main

// bindBigCores 非 Linux 平台不做 CPU 亲和性绑定
func bindBigCores() {}
