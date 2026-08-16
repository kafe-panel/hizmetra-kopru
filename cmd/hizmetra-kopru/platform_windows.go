//go:build windows

package main

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
)

// TEKNİK KİMLİK — görünen ad "Hizmetra Yazıcı" olsa da bu DEĞİŞMEZ: eski
// kurulumla (v0.1/v0.2/v0.3) çakışmayı yakalamak için birebir aynı kalmalı.
// (Test izole ad verebilsin diye değişken.)
var mutexAdi = "Global\\HizmetraKopruTekKopya"

// kilitTutamak — tekKopyaKilidi'nin SAHİBİ olduğumuz mutex tutamağı (0 = yok).
var kilitTutamak windows.Handle

// tekKopyaKilidi — aynı PC'de ikinci kopya çalışmasın (çift baskı riski).
// Sahiplenilen mutex process ömrü boyunca tutulur (kilitTutamak); kapanınca
// Windows serbest bırakır. Başka kopya tutuyorsa false.
//
// ERROR_ALREADY_EXISTS'te CreateMutex var olan nesneye de tutamak döndürür;
// onu hemen KAPATIRIZ. Tutsaydık, sahibi kapandıktan sonra nesne bizim
// tutamağımızla yaşamaya devam eder ve "Güncelle" ile başlattığımız yeni kopya
// yine "zaten çalışıyor" görürdü.
func tekKopyaKilidi() bool {
	ad, err := windows.UTF16PtrFromString(mutexAdi)
	if err != nil {
		return true // isim kurulamadıysa engelleme
	}
	h, err := windows.CreateMutex(nil, false, ad)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		return false
	}
	if err != nil {
		return true // erişim vb. sorun → engelleme (eski davranış)
	}
	kilitTutamak = h
	return true
}

// tekKopyaKilidiBirak — sahiplenilen kilidi bırakır (kurulu kopyaya devretmeden
// hemen önce; yoksa çocuk "zaten çalışıyor" görür).
func tekKopyaKilidiBirak() {
	if kilitTutamak != 0 {
		_ = windows.CloseHandle(kilitTutamak)
		kilitTutamak = 0
	}
}

// tekKopyaKilidiBekle — çalışan kopya kapatıldıktan sonra kilidi devralana kadar
// (en çok sure) dener. Süreç sonlanınca çekirdek tutamaklarını kapatır; nesne
// yok olur olmaz CreateMutex taze nesne kurar.
func tekKopyaKilidiBekle(sure time.Duration) bool {
	son := time.Now().Add(sure)
	for {
		if tekKopyaKilidi() {
			return true
		}
		if time.Now().After(son) {
			return false
		}
		time.Sleep(150 * time.Millisecond)
	}
}

// v0.4.0: otomatikBaslatKur/otomatikBaslatKaldir/otomatikBaslatYolu (HKCU Run
// anahtarı) BURADAN KALDIRILDI — Inno Setup installer'ın [Registry] girdisi
// aynı deseni ("HizmetraYazici" adıyla, tırnaklı exe yolu) artık kendisi
// yazıyor/kaldırıyor (bkz. installer/hizmetra-yazici.iss, plan Track C4).
// Autostart tamamen installer'ın sorumluluğu.
