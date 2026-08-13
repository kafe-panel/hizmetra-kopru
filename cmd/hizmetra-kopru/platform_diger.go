//go:build !windows

package main

// Windows dışı derleme (CI/geliştirme) için stub'lar. Ajan Windows hedeflidir.

func tekKopyaKilidi() bool { return true }

func otomatikBaslatKur() error { return nil }
