//go:build windows

package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// Bu testler GERÇEK Windows'ta koşar (geliştirici makinesi). Üretim
// kimliklerine (mutex adı, Run değeri, gerçek çalışan ajan) DOKUNMAZLAR:
// mutex/Run adı teste özel bir değere geçici olarak değiştirilir ve deferle
// geri alınır.

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
