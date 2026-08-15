//go:build !windows

package main

import "time"

// Windows dışı derleme (CI/geliştirme) için stub'lar. Ajan Windows hedeflidir.

func tekKopyaKilidi() bool { return true }

func tekKopyaKilidiBirak() {}

func tekKopyaKilidiBekle(time.Duration) bool { return true }

func otomatikBaslatKur(string) error { return nil }

func otomatikBaslatKaldir() error { return nil }

func otomatikBaslatYolu() string { return "" }

func calisanKopyayiKapat() error { return nil }
