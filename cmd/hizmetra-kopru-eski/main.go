// Hizmetra Yazıcı — ESKİ WINDOWS sürümü (Windows 7 / 8 / 8.1 uyumlu).
//
// NEDEN AYRI BİR İKİLİ (2026-08-20):
// Ana ajan (cmd/hizmetra-kopru) modern GUI için systray + WebView2 kullanır ve
// Go 1.21+ ile derlenir. Go 1.21'den beri üretilen ikililer Windows 10/Server
// 2016 ve üstünü ZORUNLU kılar — Windows 7/8/8.1'de HİÇ AÇILMAZ. Sahadaki bir
// kısım kafe kasası hâlâ Win7/8.1 (emre 2026-08-20: "gittiğim işletmede
// Windows 10'dan da eski bir sürümdeydi, çok büyük problem").
//
// Bu ikili:
//   - Go 1.20 ile derlenir (Win7 SP1 / 8 / 8.1 / 10 / 11 hepsinde çalışır),
//   - systray/WebView2/zenity KULLANMAZ (bu bağımlılıklar yeni Go ister);
//     arayüzü VARSAYILAN TARAYICIDA yerel bir sayfada gösterir (WebView2
//     eski Windows'ta yoktur ve modern ajanda zaten tarayıcı düşüşü vardı),
//   - ÇEKİRDEK MANTIK ORTAK: eşleştirme (internal/kurulum), fiş çekme/basma
//     (internal/kopru + internal/yazdir + internal/kesif), durum sayfası
//     (internal/durum), ayar/günlük — hepsi ana ajanla AYNI paketler.
//
// Kısıtlar (bilerek, v1): tepsi ikonu yok (arka planda sessiz çalışır, arayüz
// tarayıcıda), otomatik güncelleme şeridi yok (kullanıcı installer'ı yeniden
// indirir). Fiş basma davranışı ana ajanla birebir aynıdır.
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/ayar"
	durumsrv "github.com/kafe-panel/hizmetra-kopru/internal/durum"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/kesif"
	"github.com/kafe-panel/hizmetra-kopru/internal/kopru"
	"github.com/kafe-panel/hizmetra-kopru/internal/kurulum"
	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// Surum — derleme sırasında -ldflags "-X main.Surum=..." ile doldurulur.
var Surum = "0.1.0"

var (
	yapilandirma *ayar.Ayar
	istemci      *api.Client
	ajan         *kopru.Ajan
	durum        = &kopru.Durum{}
	durumURL     string // durum sayfasının yerel adresi (token'lı) — LOGLANMAZ
	dur          = make(chan struct{})
)

func main() {
	// "--kaldir-sunucu": Inno Setup installer'ın [UninstallRun] adımı çağırır
	// (dosyalar silinmeden HEMEN ÖNCE) — cihaz kaydını sunucudan sessizce siler.
	if bayrakVar("--kaldir-sunucu") {
		kaldirSunucudanCihazi()
		os.Exit(0)
	}

	dizin, err := ayar.Dizin()
	if err != nil {
		mesajKutusu("Ayar klasörü oluşturulamadı: " + err.Error())
		return
	}
	gunluk.Baslat(dizin)
	defer gunluk.Kapat()

	// Aynı PC'de İKİ KOPYA çalışırsa aynı fiş iki kez basılabilir → tek kopya kilidi.
	// Kilit başkasındaysa çalışan kopyanın durum sayfasını öne getirmeye çalış, çık.
	if !tekKopyaKilidi() {
		digerKopyayaOdaklanDene()
		return
	}
	gunluk.Yaz("=== Hizmetra Yazıcı (eski Windows) %s başladı === (%s)", Surum, ortamBilgisi())

	yapilandirma, err = ayar.Yukle()
	if err != nil {
		gunluk.Yaz("ayar okunamadı: %v", err)
		yapilandirma = &ayar.Ayar{}
	}
	istemci = api.New(yapilandirma.SunucuAdresi())
	istemci.Token = yapilandirma.Token

	// Durum sayfası sunucusunu ERKEN başlat: eşleşme bitince tarayıcıda açılabilsin.
	baslatDurumSunucusu()

	// Token yoksa ilk kurulum: kullanıcıdan 6 haneli kodu iste (tarayıcı sayfası).
	if istemci.Token == "" {
		if ilkKurulum() == kurulumIptal {
			gunluk.Yaz("kurulum tamamlanmadı, çıkılıyor")
			return
		}
	}

	ajan = kopru.Yeni(istemci, yazdir.Bas, kesif.Bul, Surum, durum)
	ajan.YetkisizGeldi = yenidenEslestir // token kalıcı geçersizse otomatik yeniden eşleştir
	durum.Ayarla(func(d *kopru.Durum) { d.IsletmeAd = yapilandirma.IsletmeAd })

	go ajan.NabizDongusu(dur)
	go ajan.IsDongusu(dur)

	// Kullanıcı uygulamayı ELLE açtıysa (kısayol/installer [Run]) durum sayfasını
	// tarayıcıda göster. Windows açılışında OTOMATİK başlatmada (--autostart)
	// sessiz arka planda kal — her login'de tarayıcı pop-up olmasın.
	if !bayrakVar("--autostart") && durumURL != "" {
		tarayicidaAc(durumURL)
	}

	// Tepsi yok: kapanma sinyaline kadar arka planda çalış.
	sinyal := make(chan os.Signal, 1)
	signal.Notify(sinyal, os.Interrupt, syscall.SIGTERM)
	<-sinyal
	close(dur)
	gunluk.Yaz("=== kapanıyor ===")
}

// ─────────────────────────── ilk kurulum (tarayıcı) ───────────────────────────

type kurulumSonuc int

const (
	kurulumIptal kurulumSonuc = iota
	kurulumDevam
)

// ilkKurulum — 6 haneli kurulum kodunu, yerel bir HTTP sayfasında (varsayılan
// tarayıcıda açılır) sorar. Modern ajandaki WebView penceresinin karşılığı;
// eski Windows'ta WebView2 olmadığından hep tarayıcı kullanılır.
func ilkKurulum() kurulumSonuc {
	makineAdi, _ := os.Hostname()
	sihirbaz := kurulum.Yeni(Surum, panodanKodOner(), func(kod string) (*api.EslestirCevap, string, error) {
		return eslestirmeDene(kod, makineAdi)
	})
	kurulumURL, err := sihirbaz.Baslat()
	if err != nil {
		gunluk.Yaz("kurulum sihirbazı başlatılamadı: %v", err)
		mesajKutusu("Kurulum ekranı başlatılamadı: " + err.Error())
		return kurulumIptal
	}
	tarayicidaAc(kurulumURL)

	// Eşleşmeyi bekle. Tarayıcı sekmesinin kapanışını izleyecek API yok (modern
	// ajanın WebView'inde OneGetir yoklaması vardı; tarayıcıda yok) — bu yüzden
	// yalnız başarılı eşleşme (Sonuc kanalı) beklenir. Kullanıcı vazgeçerse
	// programı tepsiden değil, Görev Yöneticisi'nden ya da yeniden başlatmayla
	// kapatır (nadir yol; eski Windows için kabul edilebilir sadelik).
	es := <-sihirbaz.Sonuc()
	return kurulumTamamlandi(es, makineAdi)
}

func kurulumTamamlandi(es kurulum.EslesmeSonucu, makineAdi string) kurulumSonuc {
	istemci = api.New(es.Sunucu)
	istemci.Token = es.Cevap.Token
	yapilandirma.Token = es.Cevap.Token
	yapilandirma.IsletmeAd = es.Cevap.IsletmeAd
	yapilandirma.CihazAd = makineAdi
	yapilandirma.SunucuURL = es.Sunucu
	if err := ayar.Kaydet(yapilandirma); err != nil {
		gunluk.Yaz("ayar kaydedilemedi: %v", err)
	}
	gunluk.Yaz("eşleşme başarılı: işletme=%s cihaz=%d", es.Cevap.IsletmeAd, es.Cevap.CihazID)
	// Kurulum sayfası zaten "Bağlandı: {işletme}" ekranını gösterdi; ekstra sekme
	// açıp kullanıcıyı şaşırtma. Durum sayfası kısayoldan/panelden erişilebilir.
	return kurulumDevam
}

// ─────────────────────────── yeniden eşleştirme ───────────────────────────

// yenidenEslestir — token KALICI geçersiz (art arda 401: cihaz panelden silinmiş).
// Modern ajandaki sync.Once + systray.Quit yerine: token'ı temizle, süreci
// yeniden başlat (boş token → ilkKurulum yeni kod ister).
func yenidenEslestir() {
	yenidenEslestirGovde("token kalıcı geçersiz (401)")
}

func yenidenEslestirGovde(sebep string) {
	gunluk.Yaz("%s → token temizlenip yeniden başlatılıyor", sebep)
	yapilandirma.Token = ""
	if err := ayar.Kaydet(yapilandirma); err != nil {
		gunluk.Yaz("token temizlenemedi: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		gunluk.Yaz("executable yolu alınamadı, yeniden başlatılamıyor: %v", err)
		return
	}
	tekKopyaKilidiBirak() // yeni süreç kilidi alabilsin
	if err := exec.Command(exe).Start(); err != nil {
		gunluk.Yaz("yeniden başlatma başarısız: %v", err)
		return
	}
	os.Exit(0)
}

// eslestirmeDene — kurulum kodunu bilinen sunucularda SIRAYLA dener (staging +
// production); KABUL eden (token dönen) sunucu ile cevabı döner.
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
			sonHata = err
		}
	}
	return nil, "", sonHata
}

// ─────────────────────────── durum sayfası (tarayıcı) ───────────────────────────

// baslatDurumSunucusu — yerel durum sayfası sunucusunu (127.0.0.1) açar ve
// tarayıcıda açılacak token'lı adresi durumURL'e koyar.
func baslatDurumSunucusu() {
	s := durumsrv.Yeni("Hizmetra Yazıcı", Surum, ozetTopla, gunluk.SonSatirlar, odaklanGeldi, guncelle)
	s.YenidenEslestirAyarla(func() { yenidenEslestirGovde("durum sayfasından yeniden eşleştir") })
	u, port, err := s.Baslat()
	if err != nil {
		gunluk.Yaz("durum sayfası başlatılamadı: %v", err)
		return
	}
	durumURL = u
	yapilandirma.SonBilinenPort = port
	yapilandirma.SonBilinenToken = s.Token
	if err := ayar.Kaydet(yapilandirma); err != nil {
		gunluk.Yaz("durum sunucusu adresi kaydedilemedi: %v", err)
	}
	gunluk.Yaz("durum sayfası yerelde hazır") // token'lı URL LOGLANMAZ
}

// odaklanGeldi — durum sunucusunun /odaklan ucuna POST geldiğinde (aynı PC'de
// ikinci kopya açılmaya çalışıldı): durum sayfasını tarayıcıda aç.
func odaklanGeldi() {
	gunluk.Yaz("/odaklan alındı: durum sayfası tarayıcıda açılıyor")
	if durumURL != "" {
		tarayicidaAc(durumURL)
	}
}

// digerKopyayaOdaklanDene — tek-kopya kilidi alınamadı: çalışan kopyanın durum
// sunucusuna POST /odaklan dener (o da sayfayı tarayıcıda açar). Ulaşılamazsa
// kullanıcıya ne yapacağını söyler.
func digerKopyayaOdaklanDene() {
	a, err := ayar.Yukle()
	if err != nil || a.SonBilinenPort == 0 || a.SonBilinenToken == "" {
		gunluk.Yaz("çalışan kopyanın adresi bilinmiyor, sessizce çıkılıyor")
		return
	}
	istemciHTTP := http.Client{Timeout: 1 * time.Second}
	adres := fmt.Sprintf("http://127.0.0.1:%d/odaklan?t=%s", a.SonBilinenPort, a.SonBilinenToken)
	r, err := istemciHTTP.Post(adres, "", nil)
	if err != nil {
		gunluk.Yaz("çalışan kopyaya ulaşılamadı: %v", err)
		mesajKutusu(
			"Hizmetra Yazıcı bu bilgisayarda zaten çalışıyor ama yanıt vermiyor.\n\n" +
				"Görev Yöneticisi'nden (Ctrl+Shift+Esc) hizmetra-kopru.exe'yi kapatıp " +
				"programı yeniden açın. Sorun sürerse bilgisayarı yeniden başlatın.")
		return
	}
	_ = r.Body.Close()
	gunluk.Yaz("çalışan kopya öne getirildi (durum %d)", r.StatusCode)
}

// guncelle — durum sayfasındaki "Güncelle" butonu (POST /guncelle). Eski Windows
// sürümünde otomatik güncelleme YOK: yeni sürüm indirme adresi varsa tarayıcıda
// açar, kullanıcı installer'ı yeniden çalıştırır.
func guncelle() {
	d := durum.Oku()
	if d.IndirmeURL == "" {
		gunluk.Yaz("güncelle: indirme adresi yok")
		return
	}
	tarayicidaAc(d.IndirmeURL)
}

// ozetTopla — durum sayfasının gösterdiği anlık özeti globallerden derler.
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
		Bagli:       d.Bagli,
		IsletmeAd:   yapilandirma.IsletmeAd,
		Sunucu:      yapilandirma.SunucuAdresi(),
		Surum:       Surum,
		Yazicilar:   yaziciAdlari,
		SonIsler:    sonIsSatirlari(20),
		SonHata:     d.SonHata,
		SonBaski:    sonBaski,
		GuncelSurum: d.GuncelSurum,
		IndirmeURL:  d.IndirmeURL,
	}
}

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

// ─────────────────────────── ortak yardımcılar ───────────────────────────

// bayrakVar — komut satırında verilen bayrak var mı (flag paketi kullanılmaz).
func bayrakVar(ad string) bool {
	for _, a := range os.Args[1:] {
		if a == ad {
			return true
		}
	}
	return false
}

// kaldirSunucudanCihazi — "--kaldir-sunucu": kayıtlı token varsa sunucudaki
// cihaz kaydını sessizce siler. HER hatada sessizce döner (çağıran os.Exit(0)).
func kaldirSunucudanCihazi() {
	dizin, err := ayar.Dizin()
	if err != nil {
		return
	}
	gunluk.Baslat(dizin)
	defer gunluk.Kapat()

	a, err := ayar.Yukle()
	if err != nil || a.Token == "" {
		gunluk.Yaz("--kaldir-sunucu: kayıtlı token yok")
		return
	}
	istemci := api.New(a.SunucuAdresi())
	istemci.Token = a.Token
	if err := istemci.Kaldir(); err != nil {
		gunluk.Yaz("--kaldir-sunucu: sunucu çağrısı yok sayıldı: %v", err)
		return
	}
	gunluk.Yaz("--kaldir-sunucu: cihaz kaydı sunucudan silindi")
}

// tarayicidaAc — verilen yerel/uzak adresi varsayılan tarayıcıda açar.
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

// panodanKodOner — panoda tam 6 haneli sayı varsa öner. Get-Clipboard yalnız
// PowerShell 5+'te (çoğu Win10+) vardır; Win7'nin PS2.0'ında yoksa boş döner.
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
