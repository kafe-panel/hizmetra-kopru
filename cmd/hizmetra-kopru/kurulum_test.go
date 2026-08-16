package main

import (
	"os"
	"testing"

	"github.com/kafe-panel/hizmetra-kopru/internal/ayar"
)

// TestBayrakVar — verilen bayrak os.Args'ta varsa bulunur. v0.4.0: somut
// bayrak sabiti (eskiden durumAcArg, "kurulu kopyaya devret" akışıyla
// birlikte kaldırıldı) kalktı; bayrakVar artık jenerik bir yardımcı — gerçek
// kullanımı Track C4'ün "--kaldir-sunucu" bayrağı (bkz. main() ve
// kaldirSunucudanCihazi, aşağıdaki testler).
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

// TestKaldirSunucudanCihaziTokenYokSessizceCikar — negatif kontrol (plan Task
// C5): kayıtlı token yokken kaldirSunucudanCihazi HİÇBİR ağ isteği atmadan,
// panic ETMEDEN döner (installer'ın --kaldir-sunucu çağrısı hiç eşleşmemiş
// bir kurulumda da güvenle çalışmalı).
func TestKaldirSunucudanCihaziTokenYokSessizceCikar(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir()) // os.UserConfigDir (Windows) → izole dizin
	kaldirSunucudanCihazi()          // panic atarsa test zaten FAIL olur
}

// TestKaldirSunucudanCihaziSunucuUlasilamazSessizceCikar — kayıtlı token VAR
// ama sunucuya ulaşılamıyor (loopback'te kapalı port — dış ağa bağımsız,
// anında ECONNREFUSED): yine panic YOK, çağrı sessizce yutulur (bkz.
// main.go: "--kaldir-sunucu" ne olursa olsun os.Exit(0) ile devam eder).
func TestKaldirSunucudanCihaziSunucuUlasilamazSessizceCikar(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	if err := ayar.Kaydet(&ayar.Ayar{Token: "k1-0-test", SunucuURL: "http://127.0.0.1:1"}); err != nil {
		t.Fatal(err)
	}
	kaldirSunucudanCihazi() // panic atarsa test zaten FAIL olur
}
