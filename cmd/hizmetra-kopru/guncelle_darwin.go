//go:build darwin

package main

// macOS oto-güncelleme — ALTIN STANDART: .dmg'yi indir → içindeki .app'i
// /Applications'a kopyala → Gatekeeper karantinasını temizle → yeni sürümü aç →
// çık. GERÇEK sessiz güncelleme: kullanıcı .dmg'yi açıp elle sürüklemez.
//
// Karmaşık dmg-kurulum mantığı internal/guncelleci'dedir (systray/cgo BAĞIMSIZ →
// Windows'ta bile GOOS=darwin ile YEREL typecheck edilir). Bu dosya yalnız ince
// glue: indir + guncelleci.DmgKur + systray.Quit. Her hata düşüş dalıyla biter
// (.dmg tarayıcıda/Finder'da açılır — eski davranıştan hiçbir zaman KÖTÜ değil).

import (
	"os"
	"path/filepath"

	"fyne.io/systray"

	"github.com/kafe-panel/hizmetra-kopru/internal/guncelleci"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

// uygulamaAdi — .dmg içindeki ve /Applications altındaki bundle adı. paket.sh
// bu adla üretir ("Hizmetra Yazıcı.app", Türkçe boşluklu).
const uygulamaAdi = "Hizmetra Yazıcı.app"

func guncellePlatform(indirmeURL, yeniSurum string) {
	gunluk.Yaz("güncelle: yeni sürüm (%s) .dmg indiriliyor…", yeniSurum)
	dmg := filepath.Join(os.TempDir(), "HizmetraYazici-guncelleme.dmg")
	if err := dosyaIndir(indirmeURL, dmg); err != nil {
		gunluk.Yaz("güncelle hatası: .dmg indirilemedi: %v — indirme tarayıcıda açılıyor", err)
		tarayicidaAc(indirmeURL)
		return
	}
	if err := guncelleci.DmgKur(dmg, uygulamaAdi); err != nil {
		gunluk.Yaz("güncelle hatası: %v — .dmg elle kurulum için açılıyor", err)
		tarayicidaAc(dmg) // .dmg'yi Finder'da aç: kullanıcı Applications'a sürükler
		return
	}
	gunluk.Yaz("güncelle: yeni sürüm kuruldu, uygulama yeniden başlatılıyor")
	systray.Quit()
}
