//go:build windows

package main

// Windows oto-güncelleme — ALTIN STANDART: hiç sihirbaz/diyalog OLMADAN indir →
// çalışan kopyayı serbest bırak → sessiz installer'ı başlat → uygulamadan çık →
// installer yeni sürümü kurup [Run] WizardSilent adımıyla yeni sürümü açar.
//
// Eski davranış (v0.6.0): installer BAYRAKSIZ (tam sihirbaz) açılıyor + systray.Quit()
// asenkron olduğu için uygulama installer başlarken hâlâ çalışıyordu → AppMutex
// diyaloğu ("Hizmetra Yazıcı çalışıyor…") çıkıyor → kullanıcı takılıp güncelleme
// yarım kalıyordu (emre 2026-08-17: 0.4.2'de kalıyor). Kanıtlı kök neden.
//
// Bayraklar:
//   /VERYSILENT /SUPPRESSMSGBOXES → hiç pencere/diyalog yok.
//   /CLOSEAPPLICATIONS            → biz geç kapanırsak Inno (Restart Manager)
//                                   çalışan kopyayı kapatıp kilitli exe'yi serbest
//                                   bırakır (emniyet kemeri; biz zaten çıkıyoruz).
//   /NORESTART                    → bilgisayarı yeniden başlatma.
//   /NOCANCEL                     → sessiz akışta iptal ucu olmasın.
// Yeniden başlatmayı Restart Manager'a BIRAKMAYIZ (biz hızlı çıktığımız için RM
// kapatacak süreç bulamaz); onun yerine .iss'teki [Run] "Check: WizardSilent"
// girdisi kurulum sonunda yeni sürümü DETERMİNİSTİK açar (bkz. hizmetra-yazici.iss).

import (
	"os"
	"os/exec"
	"path/filepath"

	"fyne.io/systray"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

func guncellePlatform(indirmeURL, yeniSurum string) {
	gunluk.Yaz("güncelle: yeni sürüm (%s) installer'ı indiriliyor…", yeniSurum)
	hedef := filepath.Join(os.TempDir(), "HizmetraYaziciKurulum.exe")
	if err := dosyaIndir(indirmeURL, hedef); err != nil {
		gunluk.Yaz("güncelle hatası: installer indirilemedi: %v", err)
		return
	}
	// Kilidi bırak: installer'ın AppMutex kontrolü bizi "çalışıyor" görmesin ve
	// installer'ın [Run] ile açacağı yeni kopya tek-kopya kilidini alabilsin.
	tekKopyaKilidiBirak()
	bayraklar := []string{
		"/VERYSILENT", "/SUPPRESSMSGBOXES",
		"/CLOSEAPPLICATIONS", "/NORESTART", "/NOCANCEL",
	}
	if err := exec.Command(hedef, bayraklar...).Start(); err != nil {
		gunluk.Yaz("güncelle hatası: installer başlatılamadı: %v", err)
		return
	}
	gunluk.Yaz("güncelle: sessiz installer başlatıldı, uygulama kapanıyor (yeni sürüm otomatik açılır)")
	systray.Quit() // exe serbest kalsın ki installer üstüne yazabilsin
}
