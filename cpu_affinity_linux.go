//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

// bindBigCores 将进程绑定到 ARM 大核 (通过 sysfs 最大频率自动检测)
func bindBigCores() {
	if runtime.GOARCH != "arm64" {
		return
	}

	log.Printf("[性能] 正在检测 ARM 大核...")

	bigCores, bigFreq, totalCores := detectBigCores()
	if len(bigCores) == 0 {
		log.Printf("[性能] 未检测到大核信息，跳过 CPU 亲和性设置")
		return
	}

	log.Printf("[性能] 检测到 %d 个 CPU 核心，其中 %d 个大核 (最大频率 %dMHz)", totalCores, len(bigCores), bigFreq/1000)

	// 构建 CPU 亲和性掩码
	var mask [128]byte // 1024 位，足够覆盖所有核心
	for _, cpu := range bigCores {
		mask[cpu/8] |= 1 << (uint(cpu) % 8)
	}

	// sched_setaffinity(pid=0 表示当前进程, cpusetsize, mask)
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_SETAFFINITY,
		0,
		uintptr(len(mask)),
		uintptr(unsafe.Pointer(&mask[0])),
	)
	if errno != 0 {
		log.Printf("[性能] CPU 亲和性设置失败: %v", errno)
		return
	}

	runtime.GOMAXPROCS(len(bigCores))
	log.Printf("[性能] 已绑定到大核 %v，GOMAXPROCS=%d", bigCores, len(bigCores))
}

// detectBigCores 通过 sysfs 读取各核心最大频率，返回频率最高的核心列表、大核频率、总核心数
func detectBigCores() (bigCores []int, bigFreq uint64, totalCores int) {
	var maxFreq uint64
	freqs := make(map[int]uint64)

	for i := 0; i < 16; i++ {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/cpufreq/cpuinfo_max_freq", i)
		data, err := os.ReadFile(path)
		if err != nil {
			break
		}
		var freq uint64
		fmt.Sscanf(string(data), "%d", &freq)
		freqs[i] = freq
		if freq > maxFreq {
			maxFreq = freq
		}
	}

	totalCores = len(freqs)
	if maxFreq == 0 {
		return nil, 0, totalCores
	}

	for cpu, freq := range freqs {
		if freq == maxFreq {
			bigCores = append(bigCores, cpu)
		}
	}
	return bigCores, maxFreq, totalCores
}
