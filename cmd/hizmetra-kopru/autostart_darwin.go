//go:build darwin

package main

// macOS otomatik başlatma (LaunchAgent).
//
// Windows'ta autostart Inno Setup installer'ın [Registry] HKCU\Run girdisi,
// Linux'ta .deb'in kurduğu /etc/xdg/autostart .desktop dosyası yapar. macOS'ta
// ise dağıtım biçimi bir .dmg (drag-to-Applications) — installer/postinstall
// YOK — bu yüzden login'de otomatik başlatmayı UYGULAMANIN KENDİSİ kurar:
// ~/Library/LaunchAgents/com.hizmetra.kopru.plist yazılır (RunAtLoad=true).
// Bu, tepsi/menü-çubuğu uygulamalarının (LSUIElement) yaygın "ilk açılışta
// login item kur" desenidir.
//
// main.go bunu açılışta BİR kez çağırır (bkz. otomatikBaslatKur çağrısı). Çağrı
// idempotenttir: plist zaten güncelse dokunmaz; exe taşınırsa yolu düzeltir.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

// launchAgentEtiket — plist Label'ı ve dosya adı kökü. Sabit: kaldırma/güncelleme
// aynı LaunchAgent'ı hedefleyebilsin.
const launchAgentEtiket = "com.hizmetra.kopru"

// otomatikBaslatKur — login'de otomatik başlatma için kullanıcının LaunchAgent
// plist'ini yazar. Best-effort: her hata loglanıp yutulur (autostart kurulamasa
// da uygulama normal çalışmaya devam etmeli). Idempotent: içerik değişmediyse
// diske yazmaz.
func otomatikBaslatKur() {
	home, err := os.UserHomeDir()
	if err != nil {
		gunluk.Yaz("launchagent: ev dizini bulunamadı: %v", err)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		gunluk.Yaz("launchagent: exe yolu bulunamadı: %v", err)
		return
	}
	// Applications altındaki gerçek yolu bul (symlink'ten başlatıldıysa çöz).
	if cozulmus, err := filepath.EvalSymlinks(exe); err == nil {
		exe = cozulmus
	}

	dizin := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(dizin, 0o755); err != nil {
		gunluk.Yaz("launchagent: klasör oluşturulamadı: %v", err)
		return
	}
	plistYolu := filepath.Join(dizin, launchAgentEtiket+".plist")

	istenen := launchAgentPlist(exe)
	if mevcut, err := os.ReadFile(plistYolu); err == nil && string(mevcut) == istenen {
		return // zaten güncel → gereksiz yazma yok
	}
	if err := os.WriteFile(plistYolu, []byte(istenen), 0o644); err != nil {
		gunluk.Yaz("launchagent: plist yazılamadı: %v", err)
		return
	}
	gunluk.Yaz("launchagent kuruldu (login'de otomatik başlatma açık)")
}

// launchAgentPlist — verilen exe yolu için LaunchAgent plist içeriğini üretir.
//
// KeepAlive BİLEREK false: RunAtLoad ile yalnız LOGIN'de başlatılır. true olsaydı
// launchd, kullanıcı tepsi menüsünden "Çıkış" yapınca uygulamayı hemen yeniden
// başlatır ve "Çıkış" işlevsiz kalırdı (Windows Run-anahtarı davranışıyla eş:
// login'de başlar, kullanıcı kapatabilir). Tek-kopya kilidiyle de uyumlu.
func launchAgentPlist(exe string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchAgentEtiket + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + xmlKacis(exe) + `</string>
		<string>--autostart</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>ProcessType</key>
	<string>Interactive</string>
</dict>
</plist>
`
}

// xmlKacis — exe yolundaki XML özel karakterlerini kaçırır (Applications yolunda
// olası değil ama plist bozulmasın diye güvenli).
func xmlKacis(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}
