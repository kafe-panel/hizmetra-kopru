//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	mutexAdi   = "Global\\HizmetraKopruTekKopya"
	autostartAnahtar = `Software\Microsoft\Windows\CurrentVersion\Run`
	autostartAd      = "HizmetraKopru"
)

// tekKopyaKilidi — aynı PC'de ikinci kopya çalışmasın (çift baskı riski).
// Mutex process ömrü boyunca tutulur; kapanınca Windows otomatik serbest bırakır.
func tekKopyaKilidi() bool {
	ad, err := windows.UTF16PtrFromString(mutexAdi)
	if err != nil {
		return true // isim kurulamadıysa engelleme
	}
	_, err = windows.CreateMutex(nil, false, ad)
	// ERROR_ALREADY_EXISTS → başka kopya çalışıyor.
	return err != windows.ERROR_ALREADY_EXISTS
}

// otomatikBaslatKur — HKCU\...\Run anahtarına kendini yazar.
//
// Windows SERVİSİ olarak kurulmuyor: servisler oturum-0 izolasyonundadır ve
// sistem tepsisinde simge GÖSTEREMEZ. Kafede PC'yi kasiyer açıyor, yani
// kullanıcı oturumu daima var — Run anahtarı hem yeterli hem yönetici hakkı
// istemiyor (kurulum 3 dakikada bitsin diye).
func otomatikBaslatKur() error {
	yol, err := os.Executable()
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, autostartAnahtar, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(autostartAd, `"`+yol+`"`)
}
