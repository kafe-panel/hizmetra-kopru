// Hizmetra Yazıcı — kafe bilgisayarındaki fiş yazıcılarını Hizmetra Panel'e bağlar.
//
// Çalışma: panelden alınan 6 haneli kurulum kodunu bir kez girersiniz; ajan
// bundan sonra panelden gelen fişleri çekip yazıcıya basar. Buluttaki sunucu
// kafenin yerel ağına ERİŞMEZ — bağlantıyı hep bu program başlatır (giden
// HTTPS), bu yüzden router/firewall ayarı GEREKMEZ.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
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
	"github.com/kafe-panel/hizmetra-kopru/internal/kurulum"
	"github.com/kafe-panel/hizmetra-kopru/internal/pencere"
	"github.com/kafe-panel/hizmetra-kopru/internal/surum"
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
	// "--kaldir-sunucu": Inno Setup installer'ın [UninstallRun] adımı çağırır
	// (dosyalar silinmeden HEMEN ÖNCE). Mutex/pencere/tray mantığından ÖNCE
	// kontrol edilir ki kaldırma sırasında tepsi ikonu/pencere HİÇ açılmasın —
	// bkz. kurulum.go:kaldirSunucudanCihazi (sessiz, hızlı, her hatayı yutar).
	if bayrakVar("--kaldir-sunucu") {
		kaldirSunucudanCihazi()
		os.Exit(0)
	}

	dizin, err := ayar.Dizin()
	if err != nil {
		_ = zenity.Error("Ayar klasörü oluşturulamadı: "+err.Error(), zenity.Title("Hizmetra Yazıcı"))
		return
	}
	gunluk.Baslat(dizin)
	defer gunluk.Kapat()

	// Aynı PC'de İKİ KOPYA çalışırsa aynı fiş iki kez basılabilir → tek kopya kilidi.
	// v0.4.0: kilit tutuluysa artık SORU SORULMAZ (eski "zaten çalışıyor,
	// Güncelle/Onar/Kaldır?" zenity akışı Inno Setup installer'a devredildi —
	// bkz. plan Track C4). Çalışan kopyaya yalnızca "pencereni öne getir"
	// sinyali gönderilir (digerKopyayaOdaklanDene) ve bu süreç sessizce çıkar.
	if !tekKopyaKilidi() {
		digerKopyayaOdaklanDene()
		return
	}
	gunluk.Yaz("=== Hizmetra Yazıcı %s başladı === (%s)", Surum, ortamBilgisi())
	otomatikBaslatKur() // macOS: login'de otomatik başlat; diğer OS'lerde no-op

	yapilandirma, err = ayar.Yukle()
	if err != nil {
		gunluk.Yaz("ayar okunamadı: %v", err)
		yapilandirma = &ayar.Ayar{}
	}
	istemci = api.New(yapilandirma.SunucuAdresi())
	istemci.Token = yapilandirma.Token

	// Durum penceresi sunucusunu ERKEN başlat: kurulum bitince "Tamam"dan sonra
	// otomatik açılabilsin (ozet globalleri canlı okur, ajan sonra başlasa da olur).
	baslatDurumSunucusu()

	// Token yoksa ilk kurulum: kullanıcıdan 6 haneli kodu iste.
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
	go surumKontrolDongusu()

	// Arayüz görünürlüğü (emre 2026-08-18: "elle açtım ama arayüz gelmedi"):
	// Kullanıcı uygulamayı ELLE açtıysa (kısayol/Uygulamayı Aç, ya da kurulum/güncelleme
	// sonrası installer [Run]) durum penceresini göster. Windows açılışında OTOMATİK
	// başlatmada (installer Run anahtarı "--autostart" verir; macOS LaunchAgent /
	// Linux autostart .desktop de öyle) sessiz tepside kal — her login'de pencere
	// pop-up olmasın. Fresh eşleşmede kurulumTamamlandi zaten açtığından bu yol
	// yalnız zaten-eşleşmiş başlangıçlarda pencereyi açar (çift açılış yok: pencere
	// paketi zaten-açıksa yenisini açmaz, öne getirir — bkz. internal/pencere).
	if !bayrakVar("--autostart") {
		go durumPenceresiniAc()
	}

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
	kurulumIptal kurulumSonuc = iota // kullanıcı vazgeçti → çık
	kurulumDevam                     // eşleşti; bu süreç ajan olarak devam eder
)

// ilkKurulum — 6 haneli kurulum kodunu, WebView penceresinde açılan gömülü
// bir sihirbaz sayfasında sorar (v0.4.0: zenity.Entry'nin yerini alır — eski
// diyaloğun metni "Kuruluma hoş geldiniz!1) Hizmetra Panel'i a…" gibi
// OKUNAMAYACAK KADAR kırpılıyordu, emre 2026-08-16 ekran görüntüsü kanıtlı).
// Eşleşme sonrası adımlar (kayıt, durum penceresine geçiş) AYNI kalır (bkz.
// kurulumTamamlandi) — yalnız kodun NEREDEN geldiği değişti.
func ilkKurulum() kurulumSonuc {
	makineAdi, _ := os.Hostname()
	sihirbaz := kurulum.Yeni(Surum, panodanKodOner(), func(kod string) (*api.EslestirCevap, string, error) {
		// TEK exe hem staging hem production'a bağlanabilsin diye kodu bilinen
		// sunucuların HEPSİNDE dene; KABUL eden (token dönen) sunucuyu kullan.
		return eslestirmeDene(kod, makineAdi)
	})
	kurulumURL, err := sihirbaz.Baslat()
	if err != nil {
		gunluk.Yaz("kurulum sihirbazı başlatılamadı: %v", err)
		_ = zenity.Error("Kurulum ekranı başlatılamadı: "+err.Error(), zenity.Title("Hizmetra Yazıcı"))
		return kurulumIptal
	}

	// WebView açılamazsa (WebView2 Runtime kurulu değil vb.) TEK düşüş dalı:
	// sistem tarayıcısı — durumPenceresiniAc ile AYNI desen. Bu yolda "pencere
	// kapatıldı" algılaması YOK (aşağıya bkz.): tarayıcı sekmesinin kapanışını
	// izleyecek bir API yok, tıpkı durum penceresinin tarayıcı düşüşünde de
	// olmadığı gibi — kabul edilebilir, nadir yol (bkz. internal/pencere).
	webviewAcik := pencere.Ac(kurulumURL, 480, 560) == nil
	if !webviewAcik {
		gunluk.Yaz("kurulum penceresi açılamadı, tarayıcıya düşülüyor")
		tarayicidaAc(kurulumURL)
	}

	// pencere.OneGetir() paketi DEĞİŞTİRMEDEN (bkz. internal/pencere) "pencere
	// kapatıldı mı" sorusuna cevap verebildiğimiz TEK yol: pencere açıkken nil,
	// kapalıyken hata döner. Kullanıcı sihirbazı X'e basıp kapatırsa (eşleşme
	// olmadan) bu yoklama olmasaydı süreç tepside görünmeden sonsuza dek asılı
	// kalırdı — eski zenity.Entry'nin "Vazgeç" düğmesinin yerini bu alır.
	var tikCh <-chan time.Time
	if webviewAcik {
		tik := time.NewTicker(500 * time.Millisecond)
		defer tik.Stop()
		tikCh = tik.C
	}
	for {
		select {
		case es := <-sihirbaz.Sonuc():
			return kurulumTamamlandi(es, makineAdi)
		case <-tikCh:
			if pencere.OneGetir() != nil {
				gunluk.Yaz("kurulum penceresi kapatıldı, kurulum iptal edildi")
				return kurulumIptal
			}
		}
	}
}

// kurulumTamamlandi — eşleşme başarılı olduktan SONRAKİ adımlar: ayarı
// kaydet, durum penceresine geç. sayfa.html zaten "Bağlandı: {işletme}"
// ekranını gösterdi; kullanıcı bunu okusun diye kısa bir bekleme payı
// bırakılır (eski zenity.Info diyaloğunun yerini alır — artık pencere
// İÇİNDE, ayrı bir OS diyaloğu YOK).
//
// v0.4.0: kurulu konuma kopyalama + Run anahtarı kurma + gerekiyorsa kurulu
// kopyaya devretme adımları BURADAN KALKTI — Inno Setup installer sabit bir
// konuma kurar ve kendi autostart/Uninstall kaydını sağlar (bkz. plan Track
// C4); bu exe artık "nerede kurulu olduğumu" hiç bilmez/yönetmez.
func kurulumTamamlandi(es kurulum.EslesmeSonucu, makineAdi string) kurulumSonuc {
	// Kazanan sunucuya kilitlen: nabız/işler bundan sonra hep oraya gider.
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

	time.Sleep(2 * time.Second) // sayfadaki "Bağlandı" ekranını okuyacak süre
	durumPenceresiniAc()
	return kurulumDevam
}

// yenidenEslestirBir — 401-otomatik yeniden eşleşme yalnız BİR kez çalışsın (iki
// döngü aynı anda 401 alıp iki kez tetikleyebilir; süreç zaten yeniden başlıyor).
var yenidenEslestirBir sync.Once

// yenidenEslestir — token KALICI geçersiz (art arda 401: cihaz panelden silinmiş).
// ajan.YetkisizGeldi olarak bağlanır (dongu.go). sync.Once ile tek sefer; çekirdek
// iş yenidenEslestirGovde'dedir.
func yenidenEslestir() {
	yenidenEslestirBir.Do(func() { yenidenEslestirGovde("token kalıcı geçersiz (401)") })
}

// kullaniciYenidenEslestir — kullanıcı tray menüsünden "Yeniden Eşleştir (kod
// gir)" seçince: ÖNCE onay iste (kafede yanlış tıklama çalışan bağlantıyı
// koparmasın), sonra çekirdeği çağır. Token GEÇERLİYKEN farklı bir hesaba/kafeye
// geçmenin tek yolu budur — 401-otomatik yol yalnız token geçersizleşince tetikler
// (emre 2026-08-17: başka hesap açıldı, token geçerli olduğu için bağlanamıyordu).
// Durum penceresi butonu onayı sayfada (confirm()) aldığı için çekirdeği DOĞRUDAN
// çağırır; bu tray yolu ise ayrıca zenity onayı gösterir. sync.Once KULLANMAZ:
// kullanıcı bilinçli tıklıyor (401 yarışı değil).
func kullaniciYenidenEslestir() {
	onay := zenity.Question(
		"Farklı bir hesaba/kafeye bağlanmak için yeniden eşleştirilsin mi?\n\n"+
			"Mevcut bağlantı kesilir ve yeni bir 6 haneli kurulum kodu istenir.",
		zenity.Title("Hizmetra Yazıcı — Yeniden Eşleştir"),
		zenity.OKLabel("Yeniden Eşleştir"),
		zenity.CancelLabel("Vazgeç"),
	)
	if onay != nil {
		return // Vazgeç veya hata → değişiklik yapma
	}
	yenidenEslestirGovde("kullanıcı yeniden eşleştirmeyi seçti")
}

// yenidenEslestirGovde — ÇEKİRDEK: config'teki token'ı temizler ve süreci YENİDEN
// BAŞLATIR: yeni süreç boş token görüp ilkKurulum'u (eşleştirme penceresini) açar,
// kullanıcı yeni 6 haneli kodu girer. Eski davranış "Eşleştirme geçersiz" gösterip
// çıkmazda kalıyordu; kod girecek yer yoktu (emre 2026-08-17).
//
// Süreç değiştirme (in-process ilkKurulum yerine): kurulumTamamlandi YENİ bir
// api.Client yaratıyor ama çalışan ajan ESKİ istemciyi tutuyor — canlı döngüde
// güvenli değiştirmek yarış/karmaşa demek. Temiz yeniden başlatma, KANITLANMIŞ
// "boş token → ilkKurulum" yolunu aynen kullanır. Kilit önce bırakılır ki yeni
// süreç tek-kopya kilidini alabilsin (bkz. tekKopyaKilidiBirak / "Güncelle" deseni).
func yenidenEslestirGovde(sebep string) {
	gunluk.Yaz("%s → token temizlenip yeniden eşleştirme için yeniden başlatılıyor", sebep)
	yapilandirma.Token = ""
	if err := ayar.Kaydet(yapilandirma); err != nil {
		gunluk.Yaz("token temizlenemedi: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		gunluk.Yaz("executable yolu alınamadı, yeniden başlatılamıyor: %v", err)
		return
	}
	tekKopyaKilidiBirak() // yeni süreç kilidi alabilsin (aksi halde /odaklan'a düşer)
	if err := exec.Command(exe).Start(); err != nil {
		gunluk.Yaz("yeniden başlatma başarısız: %v", err)
		return
	}
	systray.Quit() // bu süreç temiz kapansın (trayBitti → çıkış)
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
		bekleme := 12 * time.Hour
		bilgi, err := istemci.Surum()
		switch {
		case err != nil:
			// Kontrol başarısız (çoğunlukla geçici 502): 12 SAAT değil, KISA süre
			// sonra yeniden dene — yeni kurulan/başlatılan ajan güncellemeyi 12 saat
			// geç görmesin (emre 2026-08-18: açılışta kontrol 502 yiyince "Güncelle"
			// şeridi hiç çıkmıyordu). YazSessiz: fiş günlüğünü kirletme.
			bekleme = 10 * time.Minute
			gunluk.YazSessiz("sürüm kontrolü başarısız (%v) — 10 dk sonra yeniden denenecek", err)
		// SEMANTİK kıyas (surum.YeniMi): yalnız sunucu sürümü çalışandan KESİNLİKLE
		// ileriyse güncelle şeridi gösterilir (0.10.0 > 0.9.0 doğru; downgrade değil).
		case bilgi.Surum != "" && surum.YeniMi(bilgi.Surum, Surum):
			// Bu OS/mimari için DOĞRU paketi seç (Windows installer, macOS .dmg,
			// Linux .deb). Adres yoksa şeridi HİÇ gösterme (yanlış dosya inmesin).
			indirmeURL := bilgi.IndirmeURLIcin(runtime.GOOS, runtime.GOARCH)
			if indirmeURL == "" {
				gunluk.Yaz("yeni sürüm %s var ama %s/%s için indirme adresi yok — güncelle şeridi gösterilmiyor",
					bilgi.Surum, runtime.GOOS, runtime.GOARCH)
			} else {
				gunluk.Yaz("yeni sürüm var: %s (mevcut %s)", bilgi.Surum, Surum)
				systray.SetTooltip("Hizmetra Yazıcı — güncelleme var: " + bilgi.Surum)
				durum.Ayarla(func(d *kopru.Durum) {
					d.GuncelSurum = bilgi.Surum
					d.IndirmeURL = indirmeURL
				})
			}
		}
		select {
		case <-dur:
			return
		case <-time.After(bekleme):
		}
	}
}

// guncelle — durum penceresindeki "Güncelle" butonuna basılınca (POST /guncelle,
// durum sunucusunun onGuncelle callback'i olarak bağlanır) tetiklenir. Kullanıcı
// artık panelden elle indirip kurmaz.
//
// Ortak kısım BURADA: indirme adresi doğrulaması. Platform-özgü indir+kur+yeniden-aç
// akışı guncellePlatform'dadır (guncelle_windows.go / guncelle_darwin.go /
// guncelle_linux.go) — her biri o platformun "altın standart" sessiz güncellemesini
// yapar. Hatalar gunluk'a yazılır; satırlar durum penceresinin fiş şeridinde görünür.
func guncelle() {
	d := durum.Oku()
	if d.IndirmeURL == "" {
		gunluk.Yaz("güncelle hatası: indirme adresi yok, güncelleme iptal edildi")
		return
	}
	guncellePlatform(d.IndirmeURL, d.GuncelSurum)
}

// dosyaIndir — indirmeURL'deki paketi hedef yola indirir. Dosya KAPATILARAK
// döner: aksi halde exec.Command aynı dosyayı "kullanımda" bulurdu (Windows açık
// handle'ı kilitler). Windows installer'ı, macOS .dmg'si ve Linux .deb'i için
// ortaktır (bkz. guncelle_*.go). 5 dk zaman aşımı: yavaş kafe hattında bile yeter.
func dosyaIndir(indirmeURL, hedef string) error {
	istemciHTTP := &http.Client{Timeout: 5 * time.Minute}
	cevap, err := istemciHTTP.Get(indirmeURL)
	if err != nil {
		return err
	}
	defer cevap.Body.Close()
	if cevap.StatusCode != http.StatusOK {
		return fmt.Errorf("beklenmeyen HTTP durumu: %d", cevap.StatusCode)
	}
	f, err := os.Create(hedef)
	if err != nil {
		return err
	}
	yazilan, kopyaHata := io.Copy(f, cevap.Body)
	kapatHata := f.Close()
	if kopyaHata != nil {
		_ = os.Remove(hedef) // kısmi dosya kalmasın
		return kopyaHata
	}
	if kapatHata != nil {
		_ = os.Remove(hedef)
		return kapatHata
	}
	// Content-Length verildiyse tam indiğini DOĞRULA (hasım-review YÜKSEK-3):
	// kesik indirme (proxy/kopuk hat) io.Copy'de temiz EOF ile "başarı" görünüp
	// bozuk paketin kurulmasına yol açabilir. -1 (bilinmiyor) ise atlanır.
	if cevap.ContentLength >= 0 && yazilan != cevap.ContentLength {
		_ = os.Remove(hedef)
		return fmt.Errorf("eksik indirme: %d/%d bayt", yazilan, cevap.ContentLength)
	}
	return nil
}

func trayHazir() {
	systray.SetIcon(simgeVerisi)
	systray.SetTitle("")
	systray.SetTooltip("Hizmetra Yazıcı")

	// Tepsi simgesine SOL TIK → doğrudan uygulama arayüzünü aç (kullanıcı en sık
	// bunu ister). SAĞ TIK menüyü açar — tappedRight'ı BİLEREK set etmiyoruz;
	// set edilmeyince systray sağ tıkta menüyü kendi gösterir (systrayRightClick
	// → showMenu). SetOnTapped callback'i Windows mesaj döngüsü (wndProc) iş
	// parçacığında koşar; durumPenceresiniAc pencere açılana dek bloklayabildiği
	// için ayrı goroutine'e alıyoruz — yoksa tek tık boyunca tepsi donar.
	systray.SetOnTapped(func() { go durumPenceresiniAc() })

	mDurum := systray.AddMenuItem("Bağlanıyor…", "")
	mDurum.Disable()
	systray.AddSeparator()
	mGoster := systray.AddMenuItem("Uygulamayı Aç", "Hizmetra Yazıcı arayüzünü açar — bağlantı, yazıcılar ve fiş günlüğü")
	mPanel := systray.AddMenuItem("Yönetim Panelini Aç", "Hizmetra yönetim panelini tarayıcıda açar")
	mYeniden := systray.AddMenuItem("Yeniden Eşleştir (kod gir)", "Farklı bir hesaba/kafeye bağlan — yeni 6 haneli kurulum kodu girer")
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
			case <-mYeniden.ClickedCh:
				// Ayrı goroutine: zenity onay diyaloğu menü döngüsünü bloklamasın.
				go kullaniciYenidenEslestir()
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
	// 940×660 CSS px: kompakt sayfa.html buna sığar (güncelle şeridi + footer
	// butonları dahil); pencere.Ac DPI ölçeğini uygular → her ekranda tam görünür.
	if err := pencere.Ac(durumURL, 940, 660); err != nil {
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
//
// v0.4.0: sunucunun bağlandığı port + loopback token'ı ayar.Kaydet ile diske
// yazılır — aynı bilgisayarda açılan ikinci bir kopya (tek-kopya kilidini
// alamayan) buradan çalışan kopyanın /odaklan ucuna ulaşır (bkz.
// digerKopyayaOdaklanDene). Kayıt başarısız olsa da ajan çalışmaya devam eder;
// yalnız ikinci kopyanın "öne getir" sinyali işe yaramaz (sessizce çıkar).
func baslatDurumSunucusu() {
	s := durumsrv.Yeni("Hizmetra Yazıcı", Surum, ozetTopla, gunluk.SonSatirlar, odaklanGeldi, guncelle)
	// "Yeniden Eşleştir" butonu (sayfa.html) confirm()'i sayfada aldığı için
	// çekirdeği DOĞRUDAN çağırır (tray yolu ayrıca zenity onayı gösterir).
	s.YenidenEslestirAyarla(func() { yenidenEslestirGovde("durum penceresinden yeniden eşleştir") })
	u, port, err := s.Baslat()
	if err != nil {
		gunluk.Yaz("durum penceresi başlatılamadı: %v", err)
		return
	}
	durumURL = u
	yapilandirma.SonBilinenPort = port
	yapilandirma.SonBilinenToken = s.Token
	if err := ayar.Kaydet(yapilandirma); err != nil {
		gunluk.Yaz("durum sunucusu adresi kaydedilemedi: %v", err)
	}
	gunluk.Yaz("durum penceresi yerelde hazır") // token'lı URL LOGLANMAZ (gizlilik)
}

// odaklanGeldi — durum sunucusunun /odaklan ucuna POST geldiğinde (aynı
// bilgisayarda ikinci bir kopya açılmaya çalışıldı, bkz. digerKopyayaOdaklanDene)
// tetiklenir: açık bir durum penceresi varsa öne getirir; pencere kapalıysa
// (ajan sessizce tepside çalışıyordu) normal açılıştaki gibi açar — Start Menu
// kısayoluna tekrar tıklamak HER ZAMAN uygulamayı görünür kılar, eski "zaten
// çalışıyor" sorusunu SORMAZ.
func odaklanGeldi() {
	gunluk.Yaz("/odaklan alındı: pencere öne getiriliyor")
	if err := pencere.OneGetir(); err != nil {
		gunluk.Yaz("açık pencere yok (%v), durum penceresi açılıyor", err)
		durumPenceresiniAc()
	}
}

// digerKopyayaOdaklanDene — tek-kopya kilidi alınamadı: başka bir kopya zaten
// çalışıyor demektir. v0.4.0: eski "Zaten çalışıyor. Ne yapalım?" zenity
// diyaloğu YOK artık — kayıtlı porttan/token'dan çalışan kopyanın durum
// sunucusuna POST /odaklan denenir (kısa zaman aşımı: kafede kasiyer
// beklerken donmasın). Başarılı da olsa (öne getirildi) başarısız da olsa
// (port değişmiş, çalışan kopya port kaydetmeden çökmüş — nadir kenar durum)
// bu süreç SESSİZCE çıkar: kilit zaten başkasında, zorlamaya/eski mesajı
// göstermeye gerek yok.
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
		// Kilit BAŞKASINDA ama o kopyaya ULAŞILAMIYOR (port değişmiş, kopya
		// donmuş/zombi). Eskiden burada SESSİZCE çıkılıyordu: kullanıcı kısayola
		// çift tıklıyor, ekranda hiçbir şey olmuyor → "uygulama açılmıyor"
		// (2026-08-20 canlı olay). Artık ne olduğunu ve ne yapacağını söylüyoruz.
		gunluk.Yaz("çalışan kopyaya ulaşılamadı: %v", err)
		_ = zenity.Warning(
			"Hizmetra Yazıcı bu bilgisayarda zaten çalışıyor ama yanıt vermiyor.\n\n"+
				"Saatin yanındaki gizli simgeler (^) arasından Hizmetra Yazıcı'ya sağ tıklayıp "+
				"Çıkış deyin, sonra programı yeniden açın.\n\n"+
				"Sorun sürerse bilgisayarı yeniden başlatın.",
			zenity.Title("Hizmetra Yazıcı"),
		)
		return
	}
	_ = r.Body.Close()
	gunluk.Yaz("çalışan kopya öne getirildi (durum %d)", r.StatusCode)
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
