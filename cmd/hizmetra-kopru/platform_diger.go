//go:build !windows

package main

import "time"

// Windows dışı derleme (CI/geliştirme) için stub'lar. Ajan Windows hedeflidir.

func tekKopyaKilidi() bool { return true }

func tekKopyaKilidiBirak() {}

func tekKopyaKilidiBekle(time.Duration) bool { return true }
