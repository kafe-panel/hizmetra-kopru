// Hizmetra Köprü — kafe bilgisayarındaki fiş yazıcılarını Hizmetra Panel'e bağlar.
//
// Çalışma: panelden alınan 6 haneli kurulum kodunu bir kez girersiniz; ajan
// bundan sonra panelden gelen fişleri çekip yazıcıya basar. Buluttaki sunucu
// kafenin yerel ağına ERİŞMEZ — bağlantıyı hep bu program başlatır (giden
// HTTPS), bu yüzden router/firewall ayarı GEREKMEZ.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"fyne.io/systray"
	"github.com/ncruces/zenity"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/ayar"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/kesif"
	"github.com/kafe-panel/hizmetra-kopru/internal/kopru"
	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// Surum — derleme sırasında -ldflags "-X main.Surum=..." ile doldurulur.
var Surum = "0.1.0"

var (
	yapilandirma *ayar.Ayar
	istemci      *api.Client
	ajan         *kopru.Ajan
	durum        = &kopru.Durum{}
	dur          = make(chan struct{})
)

func main() {
	// Aynı PC'de İKİ KOPYA çalışırsa aynı fiş iki kez basılabilir → tek kopya kilidi.
	if !tekKopyaKilidi() {
		_ = zenity.Info("Hizmetra Köprü zaten çalışıyor.\n\nSaat yanındaki (sistem tepsisi) simgesinden ulaşabilirsiniz.",
			zenity.Title("Hizmetra Köprü"))
		return
	}

	dizin, err := ayar.Dizin()
	if err != nil {
		_ = zenity.Error("Ayar klasörü oluşturulamadı: "+err.Error(), zenity.Title("Hizmetra Köprü"))
		return
	}
	gunluk.Baslat(dizin)
	defer gunluk.Kapat()
	gunluk.Yaz("=== Hizmetra Köprü %s başladı ===", Surum)

	yapilandirma, err = ayar.Yukle()
	if err != nil {
		gunluk.Yaz("ayar okunamadı: %v", err)
		yapilandirma = &ayar.Ayar{}
	}
	istemci = api.New(yapilandirma.SunucuAdresi())
	istemci.Token = yapilandirma.Token

	// Token yoksa ilk kurulum: kullanıcıdan 6 haneli kodu iste.
	if istemci.Token == "" {
		if !ilkKurulum() {
			gunluk.Yaz("kurulum tamamlanmadı, çıkılıyor")
			return
		}
	}

	ajan = kopru.Yeni(istemci, yazdir.Bas, kesif.Bul, Surum, durum)
	durum.Ayarla(func(d *kopru.Durum) { d.IsletmeAd = yapilandirma.IsletmeAd })

	go ajan.NabizDongusu(dur)
	go ajan.IsDongusu(dur)
	go surumKontrolDongusu()

	// Ctrl+C / kapanma sinyali (konsoldan çalıştırıldıysa).
	sinyal := make(chan os.Signal, 1)
	signal.Notify(sinyal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sinyal
		systray.Quit()
	}()

	systray.Run(trayHazir, trayBitti)
}

// ilkKurulum — 6 haneli kodu sorar, eşleşir, token'ı kaydeder, autostart kurar.
func ilkKurulum() bool {
	for {
		kod, err := zenity.Entry(
			"Hizmetra Panel'de  Ayarlar → Yazıcılar → Bilgisayar Programı  bölümündeki\n"+
				"6 haneli Kurulum Kodunu girin:",
			zenity.Title("Hizmetra Köprü — Kurulum"),
			zenity.EntryText(""),
		)
		if err != nil { // kullanıcı iptal etti
			return false
		}
		kod = strings.TrimSpace(kod)
		if len(kod) != 6 {
			_ = zenity.Warning("Kod 6 haneli olmalı.", zenity.Title("Hizmetra Köprü"))
			continue
		}

		makineAdi, _ := os.Hostname()
		// TEK exe hem staging hem production'a bağlanabilsin diye kodu bilinen
		// sunucuların HEPSİNDE dene; KABUL eden (token dönen) sunucuyu kullan.
		cevap, sunucu, err := eslestirmeDene(kod, makineAdi)
		if err != nil {
			gunluk.Yaz("eşleştirme başarısız: %v", err)
			mesaj := "Kod geçersiz veya süresi dolmuş.\nPanelden yeni kod alıp tekrar deneyin."
			if err != api.ErrKodGecersiz {
				mesaj = "Sunucuya ulaşılamadı:\n" + err.Error() + "\n\nİnternet bağlantınızı kontrol edin."
			}
			if zenity.Question(mesaj+"\n\nTekrar denemek ister misiniz?",
				zenity.Title("Hizmetra Köprü"), zenity.OKLabel("Tekrar dene"), zenity.CancelLabel("Çık")) != nil {
				return false
			}
			continue
		}

		// Kazanan sunucuya kilitlen: nabız/işler bundan sonra hep oraya gider.
		istemci = api.New(sunucu)
		istemci.Token = cevap.Token
		yapilandirma.Token = cevap.Token
		yapilandirma.IsletmeAd = cevap.IsletmeAd
		yapilandirma.CihazAd = makineAdi
		yapilandirma.SunucuURL = sunucu
		if err := ayar.Kaydet(yapilandirma); err != nil {
			gunluk.Yaz("ayar kaydedilemedi: %v", err)
		}
		gunluk.Yaz("eşleşme başarılı: işletme=%s cihaz=%d", cevap.IsletmeAd, cevap.CihazID)

		if err := otomatikBaslatKur(); err != nil {
			gunluk.Yaz("otomatik başlatma kurulamadı: %v", err)
		}
		_ = zenity.Info(
			fmt.Sprintf("Bağlandı: %s\n\nBilgisayar açıldığında program kendiliğinden çalışacak.\n"+
				"Şimdi panelden yazıcınızı seçebilirsiniz:\nYazıcılar → Yeni Yazıcı → Bulunan Yazıcılar", cevap.IsletmeAd),
			zenity.Title("Hizmetra Köprü — Kurulum tamam"))
		return true
	}
}

// eslestirmeDene — kurulum kodunu bilinen sunucularda (ayar.SunucuAdaylari)
// SIRAYLA dener; KABUL eden (token dönen) sunucu ile cevabı döner. Böylece aynı
// exe hem staging hem production kodlarıyla çalışır — kod hangi panelde
// üretildiyse ajan o sunucuyu bulur. Hiçbiri kabul etmezse en açıklayıcı hatayı
// döner (ulaşılamama > kod-geçersiz).
func eslestirmeDene(kod, makineAdi string) (*api.EslestirCevap, string, error) {
	var sonHata error = api.ErrKodGecersiz
	for _, sunucu := range yapilandirma.SunucuAdaylari() {
		cevap, err := api.New(sunucu).Eslestir(kod, makineAdi, makineAdi, Surum, runtime.GOOS)
		if err == nil {
			gunluk.Yaz("eşleşme sunucusu: %s", sunucu)
			return cevap, sunucu, nil
		}
		gunluk.Yaz("eşleştirme denendi %s: %v", sunucu, err)
		if err != api.ErrKodGecersiz {
			sonHata = err // ulaşılamama gibi hatayı sakla; yine de diğer sunucuyu dene
		}
	}
	return nil, "", sonHata
}

func surumKontrolDongusu() {
	for {
		if bilgi, err := istemci.Surum(); err == nil && bilgi.Surum != "" && bilgi.Surum != Surum {
			gunluk.Yaz("yeni sürüm var: %s (mevcut %s)", bilgi.Surum, Surum)
			systray.SetTooltip("Hizmetra Köprü — güncelleme var: " + bilgi.Surum)
		}
		select {
		case <-dur:
			return
		case <-time.After(12 * time.Hour):
		}
	}
}

func trayHazir() {
	systray.SetIcon(simgeVerisi)
	systray.SetTitle("")
	systray.SetTooltip("Hizmetra Köprü")

	mDurum := systray.AddMenuItem("Bağlanıyor…", "")
	mDurum.Disable()
	systray.AddSeparator()
	mPanel := systray.AddMenuItem("Paneli Aç", "Hizmetra Panel'i tarayıcıda açar")
	mGunluk := systray.AddMenuItem("Günlüğü Aç", "Teknik günlük dosyası")
	systray.AddSeparator()
	mCikis := systray.AddMenuItem("Çıkış", "Programı kapat (fişler basılmaz!)")

	// Durum satırını canlı güncelle.
	go func() {
		for {
			d := durum.Oku()
			metin := "Bağlantı bekleniyor…"
			switch {
			case d.SonHata != "" && !d.Bagli:
				metin = "⚠ " + kisalt(d.SonHata, 60)
			case d.Bagli && !d.SonBaski.IsZero():
				metin = fmt.Sprintf("✓ Bağlı — son fiş %s", d.SonBaski.Format("15:04"))
			case d.Bagli:
				metin = fmt.Sprintf("✓ Bağlı (%d yazıcı)", d.YaziciSayi)
			}
			if d.IsletmeAd != "" {
				metin = d.IsletmeAd + " · " + metin
			}
			mDurum.SetTitle(metin)
			select {
			case <-dur:
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()

	go func() {
		for {
			select {
			case <-mPanel.ClickedCh:
				tarayicidaAc(panelAdresi())
			case <-mGunluk.ClickedCh:
				tarayicidaAc(gunluk.Yolu())
			case <-mCikis.ClickedCh:
				systray.Quit()
				return
			case <-dur:
				return
			}
		}
	}()
}

func trayBitti() {
	close(dur)
	gunluk.Yaz("=== kapanıyor ===")
	gunluk.Kapat()
}

// panelAdresi — API kökünden panel adresini türetir (api.X → panel.X).
func panelAdresi() string {
	s := yapilandirma.SunucuAdresi()
	if strings.Contains(s, "api.") {
		return strings.Replace(s, "api.", "panel.", 1)
	}
	return s
}

func tarayicidaAc(hedef string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", hedef)
	case "darwin":
		cmd = exec.Command("open", hedef)
	default:
		cmd = exec.Command("xdg-open", hedef)
	}
	if err := cmd.Start(); err != nil {
		gunluk.Yaz("açılamadı (%s): %v", hedef, err)
	}
}

func kisalt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
