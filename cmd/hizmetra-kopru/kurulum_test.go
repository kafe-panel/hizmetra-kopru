package main

import (
	"os"
	"testing"
)

// TestBayrakVar — verilen bayrak os.Args'ta varsa bulunur. v0.4.0: somut
// bayrak sabiti (eskiden durumAcArg, "kurulu kopyaya devret" akışıyla
// birlikte kaldırıldı) kalktı; bayrakVar artık jenerik bir yardımcı —
// gelecekteki kullanım örneği Track C4'ün "--kaldir-sunucu" bayrağı.
func TestBayrakVar(t *testing.T) {
	const bayrak = "--ornek-bayrak"
	eski := os.Args
	defer func() { os.Args = eski }()

	os.Args = []string{"hizmetra-kopru.exe", bayrak}
	if !bayrakVar(bayrak) {
		t.Fatal("bayrak bulunmalı")
	}
	os.Args = []string{"hizmetra-kopru.exe"}
	if bayrakVar(bayrak) {
		t.Fatal("bayrak yokken bulunmamalı")
	}
}
