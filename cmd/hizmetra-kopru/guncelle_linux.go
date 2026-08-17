//go:build linux

package main

// Linux oto-güncelleme — .deb'i indir → pkexec ile apt-get kur → yeni sürümü
// başlat → uygulamadan çık.
//
// Windows'taki gibi TAM sessiz (parolasız) kurulum Linux'ta güvenlik gereği
// mümkün değil: paket kurulumu root ister. En temiz "altın standart" pkexec'tir:
// polkit tek bir yetki penceresi gösterir, kullanıcı parolayı girer, kurulum
// root ile yapılır (grafik oturumda alışıldık akış). apt-get (dpkg değil)
// bağımlılıkları da çözer; yerel .deb mutlak yolla verilir.
//
// Her hata düşüş dalıyla biter: .deb indirilemez / pkexec reddedilir/başarısız
// olursa indirme adresi tarayıcıda açılır (kullanıcı elle kurabilsin) — davranış
// eski "her zaman tarayıcıda aç"tan hiçbir zaman KÖTÜ değildir.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/systray"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

func guncellePlatform(indirmeURL, yeniSurum string) {
	gunluk.Yaz("güncelle: yeni sürüm (%s) .deb indiriliyor…", yeniSurum)
	deb := filepath.Join(os.TempDir(), "hizmetra-kopru-guncelleme.deb")
	if err := dosyaIndir(indirmeURL, deb); err != nil {
		gunluk.Yaz("güncelle hatası: .deb indirilemedi: %v — indirme tarayıcıda açılıyor", err)
		tarayicidaAc(indirmeURL)
		return
	}
	// pkexec: polkit yetki penceresi → kullanıcı parolayı girince root ile kurar.
	// apt-get local .deb'i mutlak yolla kabul eder ve bağımlılıkları çözer.
	kur := exec.Command("pkexec", "apt-get", "install", "-y", deb)
	if cikti, err := kur.CombinedOutput(); err != nil {
		gunluk.Yaz("güncelle hatası: pkexec apt-get başarısız: %v (%s) — .deb tarayıcıda açılıyor",
			err, strings.TrimSpace(string(cikti)))
		tarayicidaAc(indirmeURL)
		return
	}
	// Yeni binary /usr/bin/hizmetra-kopru'ya yazıldı; yeni sürümü başlat + çık.
	if err := exec.Command("/usr/bin/hizmetra-kopru").Start(); err != nil {
		gunluk.Yaz("güncelle: yeni sürüm başlatılamadı (%v) — bir sonraki login'de otomatik açılır", err)
	}
	gunluk.Yaz("güncelle: yeni sürüm kuruldu, uygulama yeniden başlatılıyor")
	systray.Quit()
}
