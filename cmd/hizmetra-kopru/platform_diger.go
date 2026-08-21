//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/ayar"
)

// Windows DIŞI (Linux + macOS) tek-kopya kilidi — flock(2) tabanlı.
//
// NEDEN GEREKLİ (2026-08-21 denetimi): Bu dosya eskiden `return true` diyen bir
// STUB'tı ("ajan Windows hedeflidir" varsayımıyla). Ama .deb paketi HEM
// /etc/xdg/autostart (login'de otomatik başlar) HEM /usr/share/applications
// (menüden elle başlatılır) girdisi kuruyor; macOS'ta da LaunchAgent + Finder'dan
// elle açma aynı durumu yaratıyor. Kullanıcı menüden tıklayınca İKİNCİ bir ajan
// doğuyor, ikisi de aynı token'la iş kuyruğuna giriyordu → AYNI FİŞ İKİ KEZ
// basılabiliyordu (Windows'ta mutex ile korunan senaryonun ta kendisi).
//
// flock, süreç ölünce çekirdek tarafından OTOMATİK bırakılır — kilitli dosya
// diskte kalsa bile çökmüş bir kopya sonrakini bloke etmez (pid-dosyası
// yaklaşımının klasik tuzağı burada yok).
var kilitDosya *os.File

// kilitYolu — kilit dosyasının yeri. Öncelik XDG_RUNTIME_DIR (tmpfs, oturuma
// özel, yeniden başlatmada temizlenir); yoksa ajanın kendi ayar dizini; o da
// olmazsa geçici dizin. Kullanıcı-başına ayrılır: aynı makinede farklı
// kullanıcılar birbirini engellemesin.
func kilitYolu() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return filepath.Join(d, "hizmetra-kopru.lock")
	}
	if d, err := ayar.Dizin(); err == nil {
		return filepath.Join(d, "hizmetra-kopru.lock")
	}
	return filepath.Join(os.TempDir(), "hizmetra-kopru.lock")
}

// tekKopyaKilidi — kilidi almaya çalışır. true = bu süreç TEK kopyadır.
// Kilit KURULAMAZSA (izin/dosya sistemi sorunu) true dönülür: fiş basmayı
// engellemek, çift basma riskinden daha kötüdür (Windows'taki aynı tercih).
func tekKopyaKilidi() bool {
	f, err := os.OpenFile(kilitYolu(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return false // kilit BAŞKASINDA → çalışan kopya var
	}
	kilitDosya = f
	return true
}

// tekKopyaKilidiBirak — kilidi bırakır (yeniden eşleştirmede süreç kendini
// yeniden başlatmadan HEMEN ÖNCE; yoksa çocuk süreç kilidi alamaz).
func tekKopyaKilidiBirak() {
	if kilitDosya != nil {
		_ = syscall.Flock(int(kilitDosya.Fd()), syscall.LOCK_UN)
		_ = kilitDosya.Close()
		kilitDosya = nil
	}
}

// tekKopyaKilidiBekle — çalışan kopya kapandıktan sonra kilidi devralana kadar
// (en çok `sure`) dener.
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

// ortamBilgisi — çalışılan platform + mimari (günlüğe yazılır; destek çağrısında
// "hangi makinede" sorusunu cevaplar).
func ortamBilgisi() string { return runtime.GOOS + " · " + runtime.GOARCH }
