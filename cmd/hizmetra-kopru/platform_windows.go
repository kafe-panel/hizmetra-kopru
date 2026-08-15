//go:build windows

package main

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
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

// calisanKopyayiKapat — bu programın çalışan DİĞER kopyalarını kapatır ve
// bitmelerini bekler. Kendi sürecimiz (PID) HARİÇ — aksi halde "Güncelle"
// diyen kopya kendini de kapatırdı. Ad eşlemesi kopyaAdiMi: kurulu ad ve
// tarayıcı yinelenen indirme adları ("hizmetra-kopru (3).exe") — taskkill /IM
// tam ad istediği için yinelenen adları kaçırırdı; bu yüzden Toolhelp ile
// listeleyip TerminateProcess kullanılır (konsol penceresi de açılmaz).
func calisanKopyayiKapat() error {
	n, err := sureciKapat(kopyaAdiMi, windows.GetCurrentProcessId(), 5*time.Second)
	if n > 0 {
		gunluk.Yaz("çalışan kopya kapatıldı: %d süreç", n)
	}
	return err
}

// sureciKapat — adı esles ile eşleşen, PID'i haric olmayan tüm süreçleri
// sonlandırır ve her birinin bitmesini (toplam en çok bekle) bekler. Döner:
// sonlandırılan süreç sayısı ve ilk/son hata (biri kapatılamasa da diğerleri
// denenir).
func sureciKapat(esles func(ad string) bool, haric uint32, bekle time.Duration) (int, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 0, fmt.Errorf("süreç listesi alınamadı: %w", err)
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	var tutamaklar []windows.Handle
	var sonHata error
	for err = windows.Process32First(snap, &pe); err == nil; err = windows.Process32Next(snap, &pe) {
		if pe.ProcessID == haric || !esles(windows.UTF16ToString(pe.ExeFile[:])) {
			continue
		}
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pe.ProcessID)
		if err != nil {
			sonHata = fmt.Errorf("süreç %d açılamadı: %w", pe.ProcessID, err)
			continue
		}
		if err := windows.TerminateProcess(h, 1); err != nil {
			sonHata = fmt.Errorf("süreç %d sonlandırılamadı: %w", pe.ProcessID, err)
			_ = windows.CloseHandle(h)
			continue
		}
		tutamaklar = append(tutamaklar, h)
	}

	son := time.Now().Add(bekle)
	for _, h := range tutamaklar {
		kalan := time.Until(son)
		if kalan < 0 {
			kalan = 0
		}
		ev, _ := windows.WaitForSingleObject(h, uint32(kalan/time.Millisecond))
		if ev != windows.WAIT_OBJECT_0 {
			sonHata = fmt.Errorf("süreç %s içinde kapanmadı", bekle)
		}
		_ = windows.CloseHandle(h)
	}
	return len(tutamaklar), sonHata
}
