//go:build !windows

package main

import (
	"runtime"
	"time"
)

// Windows dışı derleme (CI/geliştirme) için stub'lar. Ajan Windows hedeflidir.

func tekKopyaKilidi() bool { return true }

func tekKopyaKilidiBirak() {}

func tekKopyaKilidiBekle(time.Duration) bool { return true }

// ortamBilgisi — Windows dışı karşılığı (bkz. platform_windows.go).
func ortamBilgisi() string { return runtime.GOOS + " · " + runtime.GOARCH }
