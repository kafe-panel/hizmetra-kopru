//go:build windows

package main

import (
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TEKNİK KİMLİK — mutex adı ana ajanla BİREBİR AYNI: aynı bilgisayarda modern
// ve eski ajan aynı anda çalışıp çift baskı yapmasın (bir makine yalnız birini
// kurar; yine de aynı kilit güvenli tarafta tutar).
var mutexAdi = "Global\\HizmetraKopruTekKopya"

var kilitTutamak windows.Handle

// tekKopyaKilidi — aynı PC'de ikinci kopya çalışmasın (çift baskı riski).
func tekKopyaKilidi() bool {
	ad, err := windows.UTF16PtrFromString(mutexAdi)
	if err != nil {
		return true
	}
	h, err := windows.CreateMutex(nil, false, ad)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		return false
	}
	if err != nil {
		return true
	}
	kilitTutamak = h
	return true
}

// tekKopyaKilidiBirak — sahiplenilen kilidi bırakır (yeniden başlatmadan önce).
func tekKopyaKilidiBirak() {
	if kilitTutamak != 0 {
		_ = windows.CloseHandle(kilitTutamak)
		kilitTutamak = 0
	}
}

// ortamBilgisi — çalışılan Windows sürümü + mimari (günlük). RtlGetVersion
// GERÇEK sürümü verir (manifest yalanına düşmez).
func ortamBilgisi() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d (derleme %d) · %s",
		v.MajorVersion, v.MinorVersion, v.BuildNumber, runtime.GOARCH)
}

var (
	user32DLL      = windows.NewLazySystemDLL("user32.dll")
	procMessageBox = user32DLL.NewProc("MessageBoxW")
)

const mbIconWarning = 0x00000030 // MB_ICONWARNING

// mesajKutusu — kullanıcıya native bir uyarı kutusu gösterir (zenity yerine;
// user32.MessageBoxW tüm Windows sürümlerinde vardır, cgo GEREKMEZ).
func mesajKutusu(metin string) {
	t, err := windows.UTF16PtrFromString(metin)
	if err != nil {
		return
	}
	b, err := windows.UTF16PtrFromString("Hizmetra Yazıcı")
	if err != nil {
		return
	}
	_, _, _ = procMessageBox.Call(
		0,
		uintptr(unsafe.Pointer(t)),
		uintptr(unsafe.Pointer(b)),
		uintptr(mbIconWarning),
	)
}
