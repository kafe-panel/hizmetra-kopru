//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// modernDarwinKernel — MODERN macOS kanalının gerektirdiği en düşük Darwin
// çekirdek ana sürümü. Modern ikili Go 1.22 ile derlenir ve Go 1.21'den beri
// macOS 10.15 Catalina tabanlıdır (Go 1.20 release notes: "Go 1.21 will require
// macOS 10.15 Catalina or later"). Catalina = Darwin 19.
const modernDarwinKernel = 19

// moderniDestekliyor — bu Mac MODERN kanalı çalıştırabilir mi (macOS ≥ 10.15)?
//
// Windows'taki ikizinin aynısı (bkz. platform_ek_windows.go): ESKİ (Go 1.20)
// ikili yeterince yeni bir macOS'ta çalışıyorsa oto-güncelleme onu MODERN
// pakete taşır — kullanıcı yanlış kartı indirdi diye ömür boyu eski kanalda
// kalmaz. Sürüm okunamazsa GÜVENLİ tarafta kalınır (false → eski kanal korunur):
// yanlışlıkla modern paket indirip eski Mac'te açılmayan bir uygulama kurmaktan
// iyidir.
//
// kern.osrelease sysctl'i ("19.6.0" gibi) kullanılır: sw_vers gibi harici bir
// süreç başlatmaz, her macOS sürümünde vardır.
func moderniDestekliyor() bool {
	s, err := syscall.Sysctl("kern.osrelease")
	if err != nil {
		return false
	}
	ana := s
	if i := strings.IndexByte(ana, '.'); i > 0 {
		ana = ana[:i]
	}
	n, err := strconv.Atoi(strings.TrimSpace(ana))
	if err != nil {
		return false
	}
	return n >= modernDarwinKernel
}

// gizliKomut — macOS'ta gizlenecek bir konsol penceresi yoktur (Windows'a özgü
// sorun); düz exec.Command yeterli. İmza platformlar arası AYNI kalsın diye var.
func gizliKomut(ad string, arg ...string) *exec.Cmd {
	return exec.Command(ad, arg...)
}

// panoOku — panodaki metni döndürür. macOS'ta pbpaste her kurulumda vardır
// (Windows'un PowerShell Get-Clipboard karşılığı). Eskiden bu işlev TÜM
// platformlarda "powershell Get-Clipboard" çağırıyordu → Mac'te her zaman hata
// verip boş dönüyordu, yani paneldeki "Kopyala" akışı Mac'te HİÇ çalışmıyordu.
func panoOku() string {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// uiTrayIcinde — macOS'ta TÜM pencere işleri (kurulum sihirbazı + durum ekranı)
// tepsi döngüsü BAŞLADIKTAN sonra yapılmalıdır: AppKit pencereleri yalnız ana
// iş parçacığında, çalışan bir run loop ile kurulabilir (bkz. main.go akış notu
// ve internal/pencere/pencere_darwin.m).
const uiTrayIcinde = true

// dmgIcindenCalisiyor — uygulama .dmg disk imajının İÇİNDEN mi başlatıldı?
//
// NEDEN (emre 2026-08-21, ekran görüntüsü): Kullanıcı .dmg'yi açıp uygulamayı
// Applications'a SÜRÜKLEMEDEN doğrudan imajın içinden çalıştırıyor. Bu iki
// sorun doğurur: (1) disk imajı SALT OKUNURDUR ve çıkarıldığında uygulama
// kaybolur; (2) LaunchAgent otomatik başlatma kaydı /Volumes/... altındaki
// geçici yolu işaret eder → bilgisayar yeniden başlayınca ajan HİÇ açılmaz ve
// fişler sessizce basılmaz. Bu durumu erken yakalayıp kullanıcıya ne yapacağını
// söylemek, sonradan "kurdum ama fiş basmıyor" desteğinden çok daha ucuzdur.
func dmgIcindenCalisiyor() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if cozulmus, err := filepath.EvalSymlinks(exe); err == nil {
		exe = cozulmus
	}
	return strings.HasPrefix(exe, "/Volumes/")
}
