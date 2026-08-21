//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// moderniDestekliyor — bu makine MODERN kanalı (Go 1.21+ ile derlenen ikili)
// çalıştırabilir mi? Windows'ta eşik Windows 10'dur (Go 1.21'den beri üretilen
// ikililer Win7/8/8.1'de HİÇ açılmaz).
//
// NEDEN GEREKLİ (kanal kilidi, 2026-08-21): İki installer AYNI AppId'yi
// paylaşıyor. Panelde yanlışlıkla "Windows 7/8/8.1" kartına tıklayan bir
// Win10/11 kullanıcısı ESKİ (Go 1.20) ikiliyi kurar; Kanal="eski" damgası
// yüzünden oto-güncelleme ömür boyu eski installer'ı çeker ve makine modern
// kanala BİR DAHA dönemezdi. Artık kanal seçimi damgaya DEĞİL, makinenin
// gerçek sürümüne bakar: eski ikili modern bir Windows'ta çalışıyorsa kendini
// MODERN installer'la günceller (tek yönlü, doğru yönde kurtarma).
//
// RtlGetVersion kullanılır, GetVersionEx DEĞİL: GetVersionEx manifest beyanı
// olmayan ikililere 6.2 (Win8) yalanını söyler; RtlGetVersion daima gerçeği verir.
func moderniDestekliyor() bool {
	return windows.RtlGetVersion().MajorVersion >= 10
}

// gizliKomut — konsol penceresi AÇMADAN bir yardımcı komut çalıştırır.
//
// NEDEN (2026-08-21): Ajan `-H windowsgui` ile derlendiği için kendi konsolu
// YOKTUR. Konsol alt sistemli bir çocuk süreç (powershell vb.) başlatıldığında
// Windows ona YENİ bir konsol penceresi tahsis eder → kurulum ekranı açılırken
// ekrana siyah bir pencere çakıyordu (Win7'de PowerShell 2.0 yavaş açıldığı
// için saniyelerce durabiliyordu). CREATE_NO_WINDOW + HideWindow bunu keser.
func gizliKomut(ad string, arg ...string) *exec.Cmd {
	cmd := exec.Command(ad, arg...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
	return cmd
}

// panoOku — panodaki metni döndürür (okunamazsa boş).
// Get-Clipboard cmdlet'i PowerShell 5.0 ile geldi; Windows 7 (PS 2.0), 8 (3.0)
// ve 8.1 (4.0) üzerinde YOKTUR → komut hata verir, boş dönülür (zararsız:
// kullanıcı kodu elle yazar). Konsol penceresi açılmaz (bkz. gizliKomut).
func panoOku() string {
	out, err := gizliKomut("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// uiTrayIcinde — Windows'ta pencere KENDİ iş parçacığında kendi mesaj döngüsünü
// çalıştırır (bkz. internal/pencere/pencere_windows.go), tepsi döngüsüne bağlı
// değildir; akış sırası değişmez.
const uiTrayIcinde = false
