//go:build !windows

package main

import "runtime"

// Windows dışı derleme (CI/vet) için stub'lar. Eski-Windows ajanı yalnız
// Windows'ta çalışır; bu stub'lar `go build`/`go vet`in Linux'ta geçmesi için.

func tekKopyaKilidi() bool { return true }

func tekKopyaKilidiBirak() {}

func ortamBilgisi() string { return runtime.GOOS + " · " + runtime.GOARCH }

func mesajKutusu(string) {}
