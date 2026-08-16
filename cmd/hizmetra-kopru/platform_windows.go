//go:build windows

package main

import (
	"errors"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// TEKNİK KİMLİK — görünen ad "Hizmetra Yazıcı" olsa da bunlar DEĞİŞMEZ:
// eski kurulumla (v0.1/v0.2) çakışmayı yakalamak ve aynı Run anahtarını
// güncellemek için birebir aynı kalmalı. (Testler izole ad verebilsin diye var.)
var (
	mutexAdi         = "Global\\HizmetraKopruTekKopya"
	autostartAnahtar = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartAd      = "HizmetraKopru"
)

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

// otomatikBaslatKur — HKCU\...\Run anahtarına verilen yolu yazar.
//
// Windows SERVİSİ olarak kurulmuyor: servisler oturum-0 izolasyonundadır ve
// sistem tepsisinde simge GÖSTEREMEZ. Kafede PC'yi kasiyer açıyor, yani
// kullanıcı oturumu daima var — Run anahtarı hem yeterli hem yönetici hakkı
// istemiyor (kurulum 3 dakikada bitsin diye).
//
// v0.3.0: yol parametre — çağıran KURULU kopyanın yolunu verir
// (%LOCALAPPDATA%\HizmetraKopru\hizmetra-kopru.exe), İndirilenler yolunu değil.
func otomatikBaslatKur(yol string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, autostartAnahtar, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(autostartAd, `"`+yol+`"`)
}

// otomatikBaslatKaldir — Run anahtarındaki değeri siler ("Kaldır"). Değer ya da
// anahtar yoksa hata DÖNMEZ (idempotent).
func otomatikBaslatKaldir() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartAnahtar, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(autostartAd); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// otomatikBaslatYolu — Run anahtarındaki mevcut değer (test/teşhis). Yoksa "".
func otomatikBaslatYolu() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, autostartAnahtar, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(autostartAd)
	if err != nil {
		return ""
	}
	return v
}
