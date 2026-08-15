//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Bu testler GERÇEK Windows'ta koşar (geliştirici makinesi). Üretim
// kimliklerine (mutex adı, Run değeri, gerçek çalışan ajan) DOKUNMAZLAR:
// mutex/Run adı teste özel, süreç-kapatma testi kendi kopyaladığı ikiliyi hedefler.

// TestTekKopyaKilidiSemantigi — kilit alınır; tutulurken ikinci alım başarısız;
// bırakınca yeniden alınır. (Onar/Güncelle akışı bu üç adıma dayanır.)
func TestTekKopyaKilidiSemantigi(t *testing.T) {
	eskiAd := mutexAdi
	mutexAdi = fmt.Sprintf(`Local\HizmetraKopruTest%d`, os.Getpid())
	defer func() { tekKopyaKilidiBirak(); mutexAdi = eskiAd }()

	if !tekKopyaKilidi() {
		t.Fatal("ilk kilit alınmalı")
	}
	if kilitTutamak == 0 {
		t.Fatal("sahiplenilen tutamak saklanmalı")
	}
	tutamak := kilitTutamak
	if tekKopyaKilidi() {
		t.Fatal("kilit tutulurken ikinci alım başarılı olmamalı")
	}
	if kilitTutamak != tutamak {
		t.Fatal("başarısız alım sahip tutamağını ezmemeli")
	}
	tekKopyaKilidiBirak()
	if kilitTutamak != 0 {
		t.Fatal("bırakınca tutamak sıfırlanmalı")
	}
	if !tekKopyaKilidiBekle(time.Second) {
		t.Fatal("bırakınca yeniden alınabilmeli")
	}
}

// TestOtomatikBaslatKurKaldir — Run değeri verilen yola tırnaklı yazılır,
// Kaldır siler, ikinci Kaldır hata vermez. Teste özel değer adı kullanılır.
func TestOtomatikBaslatKurKaldir(t *testing.T) {
	eskiAd := autostartAd
	autostartAd = fmt.Sprintf("HizmetraKopruTest%d", os.Getpid())
	defer func() { _ = otomatikBaslatKaldir(); autostartAd = eskiAd }()

	yol := `C:\Users\Test\AppData\Local\HizmetraKopru\hizmetra-kopru.exe`
	if err := otomatikBaslatKur(yol); err != nil {
		t.Fatal(err)
	}
	if got := otomatikBaslatYolu(); got != `"`+yol+`"` {
		t.Fatalf("Run değeri %q, beklenen tırnaklı yol", got)
	}
	if err := otomatikBaslatKaldir(); err != nil {
		t.Fatal(err)
	}
	if got := otomatikBaslatYolu(); got != "" {
		t.Fatalf("Kaldır sonrası değer kalmamalı: %q", got)
	}
	if err := otomatikBaslatKaldir(); err != nil {
		t.Fatalf("değer yokken Kaldır hata vermemeli: %v", err)
	}
}

// TestYardimciSurec — süreç-kapatma testinin hedefi: yalnız HIZMETRA_YARDIMCI_SUREC=1
// ile (kopyalanmış test ikilisi içinde) çalışır ve uyur.
func TestYardimciSurec(t *testing.T) {
	if os.Getenv("HIZMETRA_YARDIMCI_SUREC") != "1" {
		return
	}
	time.Sleep(60 * time.Second)
	os.Exit(0)
}

// TestSureciKapatDigerKopyayiKapatirKendiniKapatmaz — test ikilisini teste özel
// bir adla kopyalayıp başlatır; sureciKapat o adı eşleyerek KENDİ PID'imizi hariç
// tutar → yalnız o süreç kapanır (biz yaşarız), bitmesi beklenir.
func TestSureciKapatDigerKopyayiKapatirKendiniKapatmaz(t *testing.T) {
	kendi, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	kopya := filepath.Join(t.TempDir(), fmt.Sprintf("hizmetra-kopru-sinama-%d.exe", os.Getpid()))
	if err := dosyaKopyala(kendi, kopya); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(kopya, "-test.run=^TestYardimciSurec$")
	cmd.Env = append(os.Environ(), "HIZMETRA_YARDIMCI_SUREC=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	bitti := make(chan error, 1)
	go func() { bitti <- cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill() }() // başarısızlıkta artık kalmasın (Wait'i goroutine toplar)

	// Eşleme HEM kopyanın HEM kendi ikilimizin adını kapsar; PID'imiz hariç
	// tutulduğu için yalnız kopya kapanmalı (üretimde "Güncelle" diyen kopya
	// kendini kapatmasın — plan riski #2).
	kopyaAd, kendiAd := filepath.Base(kopya), filepath.Base(kendi)
	esles := func(s string) bool { return strings.EqualFold(s, kopyaAd) || strings.EqualFold(s, kendiAd) }
	n, err := sureciKapat(esles, windows.GetCurrentProcessId(), 5*time.Second)
	if err != nil {
		t.Fatalf("sureciKapat: %v", err)
	}
	if n != 1 {
		t.Fatalf("yalnız 1 süreç (kopya) kapatılmalıydı, %d", n)
	}
	select {
	case <-bitti: // sonlandı (Wait hata döner: exit 1) — beklenen
	case <-time.After(5 * time.Second):
		t.Fatal("hedef süreç kapanmadı")
	}
	// Buraya geldiysek kendimizi kapatmadık.
}
