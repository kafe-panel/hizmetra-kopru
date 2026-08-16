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

// v0.4.0: TestOtomatikBaslatKurKaldir KALDIRILDI — otomatikBaslatKur/Kaldir/Yolu
// artık yok (autostart Inno Setup installer'ın [Registry] girdisine devredildi,
// bkz. platform_windows.go ve installer/hizmetra-yazici.iss).
