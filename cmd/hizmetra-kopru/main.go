// Hizmetra Yazıcı — kafe bilgisayarındaki fiş yazıcılarını Hizmetra Panel'e bağlar.
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
	durumsrv "github.com/kafe-panel/hizmetra-kopru/internal/durum" // paket adı 'durum' — global 'durum' değişkeniyle çakışmasın
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/kesif"
	"github.com/kafe-panel/hizmetra-kopru/internal/kopru"
	"github.com/kafe-panel/hizmetra-kopru/internal/pencere"
	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// Surum — derleme sırasında -ldflags "-X main.Surum=..." ile doldurulur.
var Surum = "0.1.0"

var (
	yapilandirma *ayar.Ayar
	istemci      *api.Client
	ajan         *kopru.Ajan
	durum        = &kopru.Durum{}
	durumURL     string // durum penceresinin yerel adresi (token'lı) — LOGLANMAZ
	dur          = make(chan struct{})
)

func main() {
	dizin, err := ayar.Dizin()
	if err != nil {
		_ = zenity.Error("Ayar klasörü oluşturulamadı: "+err.Error(), zenity.Title("Hizmetra Yazıcı"))
		return
	}
	gunluk.Baslat(dizin)
	defer gunluk.Kapat()

	// Aynı PC'de İKİ KOPYA çalışırsa aynı fiş iki kez basılabilir → tek kopya kilidi.
	// Kilit tutuluysa kurulu kopya çalışıyordur: "zaten çalışıyor" demek yerine
	// Güncelle / Onar / Kaldır sorulur (v0.3.0, madde 2). kuruluAkisi true dönerse
	// çalışan kopya kapatılmış, kilit devralınmış ve normal başlangıç devam eder.
	if !tekKopyaKilidi() && !kuruluAkisi() {
		return
	}
	gunluk.Yaz("=== Hizmetra Yazıcı %s başladı ===", Surum)

	yapilandirma, err = ayar.Yukle()
	if err != nil {
		gunluk.Yaz("ayar okunamadı: %v", err)
		yapilandirma = &ayar.Ayar{}
	}
	istemci = api.New(yapilandirma.SunucuAdresi())
	istemci.Token = yapilandirma.Token

	// Kurulu (token var) ama başka yoldan açıldıysa — İndirilenler'deki yeni exe,
	// eski kopya çalışmıyorken — kurulu konuma taşı, Run anahtarını oraya çevir ve
	// oradan başlat. Böylece Run anahtarı hiçbir zaman silinebilir bir indirme
	// dosyasını göstermez. Zaten kurulu yoldaysak yalnız Run anahtarı tazelenir.
	if istemci.Token != "" && kuruluKopyayaDevret(durumAcArg) {
		gunluk.Yaz("kurulu kopya devraldı, bu süreç çıkıyor")
		return
	}

	// Durum penceresi sunucusunu ERKEN başlat: kurulum bitince "Tamam"dan sonra
	// otomatik açılabilsin (ozet globalleri canlı okur, ajan sonra başlasa da olur).
	baslatDurumSunucusu()

	// Token yoksa ilk kurulum: kullanıcıdan 6 haneli kodu iste.
	if istemci.Token == "" {
		switch ilkKurulum() {
		case kurulumIptal:
			gunluk.Yaz("kurulum tamamlanmadı, çıkılıyor")
			return
		case kurulumDevredildi:
			gunluk.Yaz("kurulum tamam; kurulu kopya devraldı, bu süreç çıkıyor")
			return
		}
	} else if bayrakVar(durumAcArg) {
		// Devralan kopya: kullanıcı az önce exe'ye tıkladı → durum penceresi geri bildirimi.
		durumPenceresiniAc()
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

// kurulumSonuc — ilkKurulum'un çıktısı.
type kurulumSonuc int

const (
	kurulumIptal      kurulumSonuc = iota // kullanıcı vazgeçti → çık
	kurulumDevam                          // eşleşti; bu süreç ajan olarak devam eder
	kurulumDevredildi                     // eşleşti; kurulu kopya başlatıldı → bu süreç çıkar
)

// ilkKurulum — 6 haneli kodu sorar, eşleşir, token'ı kaydeder, kendini kurulu
// konuma kopyalar, autostart'ı KOPYALANAN yola kurar ve (başka yoldan
// açıldıysa) kurulu kopyaya devreder.
func ilkKurulum() kurulumSonuc {
	for {
		// Çok satırlı metin: Windows diyaloğu tek uzun satırı kırpıyordu ("Bil..." kesiği).
		kod, err := zenity.Entry(
			"Kuruluma hoş geldiniz!\n\n"+
				"1) Hizmetra Panel'i açın\n"+
				"2) Ayarlar → Yazıcılar → Bilgisayar Programı\n"+
				"3) \"Kurulum Kodu Üret\"e basın\n\n"+
				"Paneldeki 6 haneli kodu aşağıya yazın:\n\n"+
				"Devam ederek Kullanım Koşulları'nı kabul etmiş olursunuz:\n"+
				"github.com/kafe-panel/hizmetra-kopru/blob/main/KULLANIM.md",
			zenity.Title("Hizmetra Yazıcı — Kurulum"),
			zenity.EntryText(panodanKodOner()),
		)
		if err != nil { // kullanıcı iptal etti
			return kurulumIptal
		}
		kod = strings.TrimSpace(kod)
		if len(kod) != 6 {
			_ = zenity.Warning("Kod 6 haneli olmalı.", zenity.Title("Hizmetra Yazıcı"))
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
				zenity.Title("Hizmetra Yazıcı"), zenity.OKLabel("Tekrar dene"), zenity.CancelLabel("Çık")) != nil {
				return kurulumIptal
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

		// Kendini kurulu konuma kopyala; Run anahtarı KOPYALANAN yola yazılır
		// (İndirilenler yolu değil — kullanıcı indirdiği dosyayı silince
		// otomatik başlatma kırılmasın).
		kuruluYolu, kopyalandi := kuruluKopyayaKur()
		_ = zenity.Info(
			fmt.Sprintf("Bağlandı: %s\n\nBilgisayar açıldığında program kendiliğinden çalışacak.\n"+
				"Şimdi panelden yazıcınızı seçebilirsiniz:\nYazıcılar → Yeni Yazıcı → Bulunan Yazıcılar", cevap.IsletmeAd),
			zenity.Title("Hizmetra Yazıcı — Kurulum tamam"))
		// Başka yoldan (İndirilenler) açıldıysak kurulu kopyaya devret: durum
		// penceresini o açar (--durum-ac). Devredilemezse buradan devam.
		if kopyalandi && kuruluKopyayiBaslat(kuruluYolu, durumAcArg) {
			return kurulumDevredildi
		}
		// "Tamam"dan sonra durum penceresini aç: bağlantı + hesap tek bakışta görünsün.
		durumPenceresiniAc()
		return kurulumDevam
	}
}

// Kurulu-kopya diyaloğu seçenekleri (zenity.List satırları).
const (
	secimGuncelle = "Güncelle (yeni sürümü kur)"
	secimOnar     = "Onar (yeniden eşleştir)"
	secimKaldir   = "Kaldır (bu bilgisayardan)"
)

// kuruluSecimi — kilit tutuluyken (kurulu kopya çalışıyor) ne yapılacağını
// sorar. Vazgeç/kapat → "".
func kuruluSecimi() string {
	s, err := zenity.List(
		"Zaten çalışıyor. Ne yapalım?",
		[]string{secimGuncelle, secimOnar, secimKaldir},
		zenity.Title("Hizmetra Yazıcı "+Surum),
		zenity.OKLabel("Tamam"), zenity.CancelLabel("Vazgeç"),
		zenity.DefaultItems(secimGuncelle), zenity.DisallowEmpty(),
	)
	if err != nil {
		return ""
	}
	return s
}

// kuruluAkisi — kurulu kopya çalışırken bu exe açıldı (yeni sürüm indirildi ya
// da kullanıcı sorun yaşıyor). Seçim:
//   - Güncelle: çalışanı kapat → kendini kurulu konuma kopyala → Run anahtarı →
//     yeni yoldan başlat → çık. (Zaten kurulu yoldaysak: buradan devam.)
//   - Onar: çalışanı kapat → config sil → normal akış ilkKurulum'a düşer.
//   - Kaldır: çalışanı kapat → Run anahtarı sil → config sil → kurulu exe'yi
//     sil (kendimiz değilse) → bilgi → çık.
//
// true → çağıran normal başlangıçla DEVAM etsin (kilit artık bizde).
func kuruluAkisi() bool {
	secim := kuruluSecimi()
	gunluk.Yaz("kurulu kopya çalışıyor; seçim: %q (sürüm %s)", secim, Surum)
	switch secim {
	case secimGuncelle, secimOnar, secimKaldir:
	default:
		return false
	}

	// Üç seçenek de önce çalışan kopyayı kapatır ve tek-kopya kilidini devralır.
	if err := calisanKopyayiKapat(); err != nil {
		gunluk.Yaz("çalışan kopya kapatılamadı: %v", err)
	}
	if !tekKopyaKilidiBekle(5 * time.Second) {
		gunluk.Yaz("kilit devralınamadı (çalışan kopya hâlâ ayakta)")
		_ = zenity.Error("Çalışan kopya kapatılamadı.\n\nSaat yanındaki simgeden Çıkış yapıp tekrar deneyin.",
			zenity.Title("Hizmetra Yazıcı"))
		return false
	}

	switch secim {
	case secimGuncelle:
		if kuruluKopyayaDevret(durumAcArg) {
			gunluk.Yaz("güncelleme: kurulu kopya başlatıldı, bu süreç çıkıyor")
			return false
		}
		return true // zaten kurulu yoldayız (veya kopyalanamadı): yeni sürüm olarak buradan devam
	case secimOnar:
		if err := ayar.Sil(); err != nil {
			gunluk.Yaz("ayar silinemedi: %v", err)
		}
		return true // token yok → ilkKurulum (yeniden eşleştirme)
	default: // secimKaldir
		if err := otomatikBaslatKaldir(); err != nil {
			gunluk.Yaz("otomatik başlatma kaldırılamadı: %v", err)
		}
		if err := ayar.Sil(); err != nil {
			gunluk.Yaz("ayar silinemedi: %v", err)
		}
		exeSilindi := kuruluKopyayiSil()
		mesaj := "Hizmetra Yazıcı bu bilgisayardan kaldırıldı.\n\n" +
			"• Otomatik başlatma kapatıldı\n" +
			"• Bağlantı anahtarı silindi\n\n" +
			"Panelde de bu bilgisayarı kaldırın:\nYazıcılar → Bilgisayar Programı → Kaldır"
		if !exeSilindi { // kurulu exe biziz (çalışan dosya silinemez) → kullanıcı silsin
			if kendi, err := os.Executable(); err == nil {
				mesaj += "\n\nProgram dosyasını kapandıktan sonra silebilirsiniz:\n" + kendi
			}
		}
		_ = zenity.Info(mesaj, zenity.Title("Hizmetra Yazıcı — Kaldırıldı"))
		return false
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
			systray.SetTooltip("Hizmetra Yazıcı — güncelleme var: " + bilgi.Surum)
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
	systray.SetTooltip("Hizmetra Yazıcı")

	mDurum := systray.AddMenuItem("Bağlanıyor…", "")
	mDurum.Disable()
	systray.AddSeparator()
	mGoster := systray.AddMenuItem("Durumu Göster", "Bağlantı, yazıcılar ve fiş günlüğünü penceresinde gösterir")
	mPanel := systray.AddMenuItem("Paneli Aç", "Hizmetra Panel'i tarayıcıda açar")
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
			case <-mGoster.ClickedCh:
				durumPenceresiniAc()
			case <-mPanel.ClickedCh:
				tarayicidaAc(panelAdresi())
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

// durumPenceresiniAc — durum penceresini GERÇEK bir native pencerede
// (WebView2) açmayı dener (emre 2026-08-16: "bilgisayardaki uygulamada
// görünmesi gerek" — sistem tarayıcısında DEĞİL). Pencere açılamazsa
// (WebView2 Runtime kurulu değil, oluşturma hatası/zaman aşımı) TEK düşüş
// dalı: eski davranış — sistem tarayıcısında aç. "Paneli Aç" tray menüsü BU
// FONKSİYONU KULLANMAZ; o gerçek bir dış web sitesi, hep tarayicidaAc ile açılır.
func durumPenceresiniAc() {
	if durumURL == "" {
		return
	}
	if err := pencere.Ac(durumURL, 900, 640); err != nil {
		gunluk.Yaz("durum penceresi açılamadı (%v), tarayıcıya düşülüyor", err)
		tarayicidaAc(durumURL)
	}
}

func kisalt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// baslatDurumSunucusu — yerel durum penceresi sunucusunu (127.0.0.1) açar ve
// açılacak token'lı adresi durumURL'e koyar. Başarısız olursa ajan çalışmaya
// devam eder; yalnız "Durumu Göster" işlevsiz kalır.
func baslatDurumSunucusu() {
	s := durumsrv.Yeni("Hizmetra Yazıcı", Surum, ozetTopla, gunluk.SonSatirlar)
	u, err := s.Baslat()
	if err != nil {
		gunluk.Yaz("durum penceresi başlatılamadı: %v", err)
		return
	}
	durumURL = u
	gunluk.Yaz("durum penceresi yerelde hazır") // token'lı URL LOGLANMAZ (gizlilik)
}

// ozetTopla — durum penceresinin gösterdiği anlık özeti globallerden derler.
func ozetTopla() durumsrv.Ozet {
	d := durum.Oku()
	var yaziciAdlari []string
	if yzc, err := kesif.Bul(); err == nil {
		for _, y := range yzc {
			yaziciAdlari = append(yaziciAdlari, y.Ad)
		}
	}
	sonBaski := ""
	if !d.SonBaski.IsZero() {
		sonBaski = d.SonBaski.Format("15:04")
	}
	return durumsrv.Ozet{
		Bagli:     d.Bagli,
		IsletmeAd: yapilandirma.IsletmeAd,
		Sunucu:    yapilandirma.SunucuAdresi(),
		Surum:     Surum,
		Yazicilar: yaziciAdlari,
		SonIsler:  sonIsSatirlari(20),
		SonHata:   d.SonHata,
		SonBaski:  sonBaski,
	}
}

// sonIsSatirlari — bellekteki günlükten baskı işi satırlarını süzer (fiş özeti).
func sonIsSatirlari(n int) []string {
	var out []string
	for _, s := range gunluk.SonSatirlar(200) {
		if strings.Contains(s, "iş #") {
			out = append(out, s)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

// panodanKodOner — panoda tam 6 haneli sayı varsa giriş alanına önerir (panel
// "Kopyala" butonuyla akışı tek tıka indirir). Pano okunamazsa boş döner.
func panodanKodOner() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard").Output()
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(out))
	if len(s) == 6 && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		return s
	}
	return ""
}
