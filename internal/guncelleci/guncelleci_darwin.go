//go:build darwin

// Package guncelleci — platform-özgü oto-güncelleme KURULUM mantığı, systray/cgo
// BAĞIMSIZ tutulur. Neden ayrı paket: cmd/hizmetra-kopru (main) macOS'ta
// fyne.io/systray'in Cocoa/cgo arka ucunu import eder → Windows/Linux'ta darwin
// için typecheck EDİLEMEZ (SDK/clang yok). Bu paket cgo'suz olduğu için
// `GOOS=darwin CGO_ENABLED=0 go build ./internal/guncelleci` ile YEREL derlenir —
// dmg-kurulum mantığı böylece macOS runner'ını beklemeden doğrulanır.
package guncelleci

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DmgKur — indirilmiş .dmg'yi bağlar, içindeki .app'i /Applications'a kopyalar,
// Gatekeeper karantinasını temizler ve yeni sürümü açar. Çağıran (guncelle_darwin.go)
// başarıda uygulamadan çıkar. Her adım hata dönebilir; mount HER durumda çözülür.
//
// Unix'te çalışan sürecin executable inode'u silinse bile süreç yaşar → eski
// .app'i silip yenisini kopyalamak güvenlidir; çıkışta 'open' TAZE sürümü başlatır.
func DmgKur(dmgYolu, uygulamaAdi string) error {
	mnt := filepath.Join(os.TempDir(), "hizmetra-guncelle-mnt")
	_ = exec.Command("hdiutil", "detach", mnt, "-force").Run() // önceki kalıntıyı temizle
	if err := os.MkdirAll(mnt, 0o755); err != nil {
		return fmt.Errorf("bağlama klasörü oluşturulamadı: %w", err)
	}
	if cikti, err := exec.Command("hdiutil", "attach", dmgYolu,
		"-nobrowse", "-noverify", "-noautoopen", "-mountpoint", mnt).CombinedOutput(); err != nil {
		return fmt.Errorf(".dmg bağlanamadı: %v (%s)", err, strings.TrimSpace(string(cikti)))
	}
	defer func() { _ = exec.Command("hdiutil", "detach", mnt, "-force").Run() }()

	kaynak := filepath.Join(mnt, uygulamaAdi)
	if _, err := os.Stat(kaynak); err != nil {
		return fmt.Errorf(".app .dmg içinde bulunamadı (%s): %w", kaynak, err)
	}
	hedef := filepath.Join("/Applications", uygulamaAdi)

	// Eski .app'i kaldır + yenisini kopyala (ditto, .app kaynaklarını korur).
	_ = os.RemoveAll(hedef)
	if cikti, err := exec.Command("ditto", kaynak, hedef).CombinedOutput(); err != nil {
		return fmt.Errorf("kopyalama (ditto) başarısız: %v (%s)", err, strings.TrimSpace(string(cikti)))
	}
	// Gatekeeper karantina bayrağını temizle (imzasız indirme aksi halde açılmaz).
	_ = exec.Command("xattr", "-dr", "com.apple.quarantine", hedef).Run()

	// Yeni sürümü aç (-n: biz kapanana kadar kısa süre iki kopya olabilir; macOS'ta
	// tek-kopya kilidi yok, güncelleme anında kabul edilebilir bir örtüşme).
	if cikti, err := exec.Command("open", "-n", hedef).CombinedOutput(); err != nil {
		return fmt.Errorf("yeni sürüm açılamadı: %v (%s)", err, strings.TrimSpace(string(cikti)))
	}
	return nil
}
