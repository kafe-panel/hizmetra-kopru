// Package kopru — ajanın çekirdek çalışma döngüsü (tray'den bağımsız, test edilebilir).
package kopru

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// Durum — tray'in gösterdiği canlı durum.
type Durum struct {
	sync.Mutex
	Bagli      bool
	SonHata    string
	SonBaski   time.Time
	IsletmeAd  string
	YaziciSayi int
	// GuncelSurum/IndirmeURL — sunucudaki daha yeni ajan sürümü + installer
	// indirme adresi (main.go surumKontrolDongusu doldurur). Boşsa güncelleme
	// yok. Durum penceresi bunları okuyup "Güncelle" şeridini gösterir.
	GuncelSurum string
	IndirmeURL  string

	// SonBaskiSorunu/SonBaskiKodu — EN SON BASKI sorununun tek cümlelik Türkçe
	// açıklaması ve makine kodu (teshis paketi).
	//
	// SonHata'dan AYRI TUTULUR, çünkü baskı hatasında BAĞLANTI sağlamdır:
	// nabız başarılı olduğu an eski kod SonHata'yı siliyordu ve kullanıcı
	// "✓ Bağlı — son fiş 14:32" görüp fişin hiç çıkmadığını fark etmiyordu.
	// Bu alanları YALNIZ teyitli başarılı bir baskı temizler; nabız ve iş
	// çekme bunlara DOKUNMAZ.
	SonBaskiSorunu string
	SonBaskiKodu   string
	// SonBaskiHedef — sorunun YAŞANDIĞI yazıcı (onarım bu hedefe uygulanır).
	SonBaskiHedef    string
	SonBaskiSorunuAt time.Time
	// UstUsteHata — art arda kaç baskı başarısız oldu (tek seferlik blip ile
	// süregelen arızayı ayırt etmek için).
	UstUsteHata int
}

func (d *Durum) Ayarla(f func(*Durum)) {
	d.Lock()
	defer d.Unlock()
	f(d)
}

func (d *Durum) Oku() Durum {
	d.Lock()
	defer d.Unlock()
	return Durum{
		Bagli: d.Bagli, SonHata: d.SonHata, SonBaski: d.SonBaski,
		IsletmeAd: d.IsletmeAd, YaziciSayi: d.YaziciSayi,
		GuncelSurum: d.GuncelSurum, IndirmeURL: d.IndirmeURL,
		SonBaskiSorunu: d.SonBaskiSorunu, SonBaskiKodu: d.SonBaskiKodu,
		SonBaskiHedef:    d.SonBaskiHedef,
		SonBaskiSorunuAt: d.SonBaskiSorunuAt, UstUsteHata: d.UstUsteHata,
	}
}

// Basici — baskı fonksiyonu (test için değiştirilebilir).
type Basici func(hedef string, veri []byte) error

// Kesifci — yazıcı listeleme fonksiyonu (test için değiştirilebilir).
type Kesifci func() ([]api.Yazici, error)

// Ajan — çalışma döngüsünü yürütür.
type Ajan struct {
	Istemci *api.Client
	Bas     Basici
	Kesfet  Kesifci
	Surum   string
	Durum   *Durum

	// basildiKayit — ÇİFT BASKI KALKANI. Sunucu bir işi (ajan yavaş sanılıp)
	// yeniden verirse, bu ajan onu ZATEN basmışsa TEKRAR BASMAZ; doğrudan
	// 'basildi' bildirir. Sunucu tarafındaki 300sn yeniden-sahiplenme
	// penceresinin ajan-tarafı ikizi.
	basildiKayit map[int64]time.Time
	// supheliKayit — baskısı YARIDA KALAN işler (ASILDI / YARIM_YAZILDI).
	// Bu işlerin kağıda dökülüp dökülmediği BİLİNMİYOR: tıkanan spooler
	// açıldığı an eski iş kendiliğinden basabilir. Bu yüzden sunucuya
	// 'hata' DEĞİL 'basildi' bildirilir (hata deseydik sunucu 2 dakika sonra
	// aynı fişi yeniden verir, mutfağa iki fiş düşerdi) ve iş bir daha
	// ASLA basılmaz. Kullanıcı durum kartındaki uyarıyı görür; kağıt
	// gerçekten çıkmadıysa panelden kendi eliyle yeniden gönderir.
	supheliKayit map[int64]time.Time
	kayitKilit   sync.Mutex

	// kalkanYolu — çift baskı kalkanının DİSKTEKİ hâli (bkz. kalkan.go).
	// Boşsa kalkan yalnız bellekte yaşar (eski davranış; testler böyle kullanır).
	kalkanYolu string
	// bildirilmedi — sunucuya ULAŞMAYAN sonuçlar; sıradaki turda yeniden POST edilir.
	bildirilmedi []api.Sonuc

	// bekleSn/pollSn — SUNUCU DİREKTİFİ (nabız cevabı). Ajan bunlara uyar;
	// böylece sunucu 1000 cihazda ajanı güncellemeden kısa-poll'a geçirir.
	bekleSn   int
	pollSn    int
	ayarKilit sync.Mutex

	// YetkisizGeldi — token KALICI geçersizleşince (art arda 401: cihaz panelden
	// silinmiş) çağrılır. main tarafı bunu sync.Once ile yakalar: config'teki
	// token'ı temizler + süreci yeniden başlatır ki boş token'la ilkKurulum
	// (eşleştirme penceresi) açılsın — kullanıcı yeni kodu girer. nil ise eski
	// davranış: yalnız "Eşleştirme geçersiz" gösterilir (çıkmaz). Bu, emre'nin
	// 2026-08-17 yaşadığı "cihaz silindi, kod girecek yer yok" bug'ının çözümü.
	YetkisizGeldi func()
	yetkisizSayac int
	yetkisizKilit sync.Mutex
}

// yetkisizArtir — art arda 401 sayar; EŞİK (2) aşılınca YetkisizGeldi'yi tetikler.
// Eşik 2: tek gelip-geçici bir 401 (ör. sunucu kısa kesinti) yeniden eşleştirmeyi
// (token temizleme + yeniden başlatma) TETİKLEMESİN. Başarılı nabız/işte sıfırlanır.
func (a *Ajan) yetkisizArtir() {
	a.yetkisizKilit.Lock()
	a.yetkisizSayac++
	tetik := a.yetkisizSayac >= 2
	a.yetkisizKilit.Unlock()
	if tetik && a.YetkisizGeldi != nil {
		a.YetkisizGeldi()
	}
}

func (a *Ajan) yetkisizSifirla() {
	a.yetkisizKilit.Lock()
	a.yetkisizSayac = 0
	a.yetkisizKilit.Unlock()
}

// Yeni — ajan kurar.
func Yeni(istemci *api.Client, bas Basici, kesfet Kesifci, surum string, durum *Durum) *Ajan {
	return &Ajan{
		Istemci: istemci, Bas: bas, Kesfet: kesfet, Surum: surum, Durum: durum,
		basildiKayit: map[int64]time.Time{},
		supheliKayit: map[int64]time.Time{},
		bekleSn:      25,
		pollSn:       25,
	}
}

func (a *Ajan) direktif() (bekle, poll int) {
	a.ayarKilit.Lock()
	defer a.ayarKilit.Unlock()
	return a.bekleSn, a.pollSn
}

func (a *Ajan) direktifAyarla(bekle, poll int) {
	a.ayarKilit.Lock()
	defer a.ayarKilit.Unlock()
	if poll > 0 {
		a.pollSn = poll
	}
	if bekle >= 0 {
		a.bekleSn = bekle
	}
}

// zatenBasildi — bu iş bu oturumda basıldı mı? (çift baskı kalkanı)
func (a *Ajan) zatenBasildi(isID int64) bool {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	_, var_ := a.basildiKayit[isID]
	return var_
}

// supheliMi — bu işin baskısı daha önce YARIDA KALDI mı? (kör yeniden baskı
// kalkanı)
func (a *Ajan) supheliMi(isID int64) bool {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	_, var_ := a.supheliKayit[isID]
	return var_
}

// supheliIsaretle — işi "uçuşta kalmış" olarak deftere yazar ve diske kaydeder.
func (a *Ajan) supheliIsaretle(isID int64) {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	a.supheliKayit[isID] = time.Now()
	sinir := time.Now().Add(-kalkanSaklamaSuresi)
	for id, t := range a.supheliKayit {
		if t.Before(sinir) {
			delete(a.supheliKayit, id)
		}
	}
	a.kalkanKaydet()
}

// supheliKod — bu hata kodunda iş spooler'a GİRMİŞ ve orada kalmış olabilir mi?
// Böyle işlerde sunucuya 'hata' bildirmek ÇİFT FİŞ demektir.
func supheliKod(kod teshis.Kod) bool {
	return kod == teshis.ASILDI || kod == teshis.YARIM_YAZILDI
}

func (a *Ajan) basildiIsaretle(isID int64) {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	a.basildiKayit[isID] = time.Now()
	// SÜRE TABANLI süzme — HER işaretlemede. Eski kod yalnız harita 500'ü
	// AŞINCA temizliyordu; o yüzden harita hiç 500'ün altına inmiyor ve yoğun
	// bir günde 1 saatten yeni 500+ iş varsa HİÇ temizlenmiyordu.
	sinir := time.Now().Add(-kalkanSaklamaSuresi)
	for id, t := range a.basildiKayit {
		if t.Before(sinir) {
			delete(a.basildiKayit, id)
		}
	}
	a.kalkanKaydet()
}

// NabizDongusu — periyodik "hayattayım" + yazıcı listesi. dur kapanınca çıkar.
func (a *Ajan) NabizDongusu(dur <-chan struct{}) {
	for {
		a.nabizAt()
		_, poll := a.direktif()
		bekleme := time.Duration(maks(poll, 30)) * time.Second
		select {
		case <-dur:
			return
		case <-time.After(bekleme):
		}
	}
}

func (a *Ajan) nabizAt() {
	// PANİK AĞI: keşif/HTTP yolunda beklenmedik bir panik (ör. printers.Open'ın
	// NUL'lu adda paniklemesi) tüm ajanı öldürüyordu. Artık günlüğe yazılır ve
	// döngü yaşamaya devam eder.
	defer func() {
		if p := recover(); p != nil {
			gunluk.Yaz("nabız sırasında beklenmeyen hata (döngü sürüyor): %v", p)
		}
	}()

	yazicilar, err := a.Kesfet()
	if err != nil {
		gunluk.Yaz("keşif hatası (nabız yine de atılıyor): %v", err)
		yazicilar = nil
	}
	cevap, err := a.Istemci.Nabiz(yazicilar, a.Surum)
	if err != nil {
		a.hataIsle("nabız", err)
		return
	}
	bekle := cevap.PollSn
	if cevap.BekleSn != nil {
		bekle = *cevap.BekleSn
	}
	a.direktifAyarla(bekle, cevap.PollSn)
	a.yetkisizSifirla() // başarılı nabız → 401 sayacı sıfırlanır
	a.Durum.Ayarla(func(d *Durum) {
		d.Bagli = true
		d.SonHata = ""
		d.YaziciSayi = len(yazicilar)
		// SonBaskiSorunu'na DOKUNULMAZ: bağlantının sağlam olması fişin
		// çıktığı anlamına GELMEZ.
	})
}

// IsDongusu — işleri çeker, basar, sonucu bildirir. dur kapanınca çıkar.
func (a *Ajan) IsDongusu(dur <-chan struct{}) {
	geriCekilme := time.Second
	// Geçici ağ-geçidi (502/503/504) gürültü kısması: long-poll sırasında
	// Cloudflare/Render araya girip bunu döndürebilir; iş yine gelir. Kendi
	// (kısa) geri çekilmesi + sayaçla kısılan logu vardır — kullanıcıya
	// "kopuk" GÖSTERİLMEZ (nabız Bagli'yi zaten doğru yönetir).
	geciciGeri := time.Second
	var geciciSayac int
	var geciciSonLog time.Time
	for {
		select {
		case <-dur:
			return
		default:
		}

		bekle, poll := a.direktif()
		isler, err := a.Istemci.Isler(bekle)
		if err != nil {
			if errors.Is(err, api.ErrYetkisiz) {
				a.Durum.Ayarla(func(d *Durum) {
					d.Bagli = false
					d.SonHata = "Eşleştirme geçersiz — panelden yeni kurulum kodu alın"
				})
				gunluk.Yaz("yetkisiz: cihaz panelden kaldırılmış olabilir")
				a.yetkisizArtir() // art arda 401 → main yeniden eşleştirme penceresi açar
				// Yetki hatasında hızlı döngüye girme.
				if bekleVeyaDur(dur, 60*time.Second) {
					return
				}
				continue
			}
			if errors.Is(err, api.ErrGecici) {
				// Geçici ağ-geçidi hatası (502/503/504): Cloudflare/Render
				// long-poll'a araya girdi ama iş yine gelir; nabız Bagli'yi
				// yönetir. hataIsle ÇAĞIRMA (kullanıcıya "kopuk"/kırmızı hata
				// yazma) — SESSİZCE, kısa geri çekilmeyle yeniden dene ve logu
				// KIS: art arda aynı hatayı en fazla ~5 dakikada bir yaz (sayaçla).
				geciciSayac++
				if geciciSonLog.IsZero() || time.Since(geciciSonLog) > 5*time.Minute {
					// YazSessiz: yalnız dosya loguna (teşhis). Kullanıcının fiş
					// günlüğünde GÖSTERİLMEZ — geçici, otomatik toparlanan bir blip;
					// fiş yine basılıyor, durum kartı yeşil kalıyor (emre 2026-08-18:
					// "hatasız çalışması gerek" — aslında çalışıyor, sadece korkutmasın).
					gunluk.YazSessiz("geçici ağ-geçidi hatası (502/503/504) ×%d — sessiz yeniden deneniyor", geciciSayac)
					geciciSonLog = time.Now()
					geciciSayac = 0
				}
				if bekleVeyaDur(dur, geciciGeri) {
					return
				}
				geciciGeri = min(geciciGeri*2, 15*time.Second)
				continue
			}
			a.hataIsle("iş çekme", err)
			// Üstel geri çekilme + jitter (uyanma/ağ kopması sonrası sürü etkisi yok).
			uyku := geriCekilme + time.Duration(rand.Int63n(int64(geriCekilme/5+1)))
			if bekleVeyaDur(dur, uyku) {
				return
			}
			geriCekilme = min(geriCekilme*2, 60*time.Second)
			continue
		}
		geriCekilme = time.Second
		geciciGeri = time.Second // başarılı çekme → geçici-hata geri çekilmesi de sıfırlanır
		a.yetkisizSifirla()      // başarılı iş çekme → 401 sayacı sıfırlanır
		// SonBaskiSorunu'na DOKUNULMAZ (bkz. Durum alan açıklaması): başarılı bir
		// iş çekme, çıkmayan fişi çıkmış yapmaz.
		a.Durum.Ayarla(func(d *Durum) { d.Bagli = true; d.SonHata = "" })

		if len(isler) == 0 {
			// bekle=0 (kısa poll) modunda sunucunun verdiği aralık kadar bekle.
			if bekle == 0 {
				if bekleVeyaDur(dur, time.Duration(maks(poll, 1))*time.Second) {
					return
				}
			}
			continue
		}
		a.isleriBas(isler)
	}
}

// basZamanAsimiTest — TEK bir işin baskısı için üst sınır. Aşılırsa iş ASLA
// 'basildi' sayılmaz ve KÖR YENİDEN DENENMEZ (yarım fiş + tam fiş = çift fiş).
// DEĞİŞKEN: testler kısaltıp gerçekten asılan bir yazıcıyı 30 saniye beklemeden
// doğrulayabilsin (üretim değeri 30 sn).
var basZamanAsimiTest = 30 * time.Second

// partiButcesiTest — bir turda çekilen işlerin TAMAMI için üst sınır. Tek asılı
// yazıcı bütün kuyruğu kilitlemesin: kalan işler basılmadan 'hata' bildirilir,
// sunucu onları 2 dakika sonra yeniden verir (üretim değeri 150 sn).
var partiButcesiTest = 150 * time.Second

// basZamanAsimiyla — a.Bas'ı AYRI goroutine'de çalıştırır ve süre sınırı koyar.
//
// NEDEN GOROUTINE: baskı yolu (CUPS/spooler) BLOKE olabilir ve iptal edilemez.
// Süre dolunca goroutine arkada kalmaya devam eder — ama iş döngüsü kurtulur;
// nabız zaten ayrı goroutine'de olduğu için eskiden panel YEŞİL kalıp fiş hiç
// basılmıyordu.
func (a *Ajan) basZamanAsimiyla(hedef string, veri []byte) error {
	// Bas ALANI goroutine'e GİRMEDEN yerele kopyalanır: zaman aşımında goroutine
	// arkada kalmaya devam ettiği için, alan sonradan değişirse (testte sahte
	// Basici takılması) terk edilmiş goroutine ile veri yarışı oluşurdu.
	bas := a.Bas
	bitti := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				bitti <- fmt.Errorf("yazdırma sırasında beklenmeyen hata: %v", p)
			}
		}()
		bitti <- bas(hedef, veri)
	}()
	select {
	case err := <-bitti:
		return err
	case <-time.After(basZamanAsimiTest):
		return teshis.Yeni(teshis.ASILDI, hedef, "", nil)
	}
}

func (a *Ajan) isleriBas(isler []api.Is) {
	// PANİK AĞI: tek bir bozuk iş tüm ajanı öldürmesin.
	defer func() {
		if p := recover(); p != nil {
			gunluk.Yaz("baskı sırasında beklenmeyen hata (döngü sürüyor): %v", p)
			a.baskiSorunuYaz(teshis.Cumle(teshis.BILINMEYEN, "", ""), string(teshis.BILINMEYEN), "")
		}
	}()

	// Önceki turda bildirilemeyen sonuçlar önce kuyruğa girer.
	sonuclar := a.bekleyenSonuclar()
	partiBitis := time.Now().Add(partiButcesiTest)

	for _, is := range isler {
		// ÇİFT BASKI KALKANI: sunucu aynı işi tekrar verdiyse basma, sadece bildir.
		if a.zatenBasildi(is.IsID) {
			gunluk.Yaz("iş #%d zaten basılmıştı — tekrar basılmadı, sonuç yeniden bildiriliyor", is.IsID)
			sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "basildi"})
			continue
		}
		// ŞÜPHELİ İŞ KALKANI: baskısı yarıda kalmış bir iş sunucu tarafından
		// yine de yeniden verilirse (ör. sonuç POST'u kaybolduysa) KÖR OLARAK
		// yeniden basmayız — o fiş spooler'da basılmış olabilir.
		if a.supheliMi(is.IsID) {
			gunluk.Yaz("iş #%d daha önce yarıda kalmıştı — çift fiş riskine karşı tekrar basılmadı", is.IsID)
			sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "basildi"})
			continue
		}
		if time.Now().After(partiBitis) {
			// Parti bütçesi doldu: KALAN işleri basmadan 'hata' bildir ki sunucu
			// onları yeniden versin. Sessizce düşürmek fişin kaybolması demekti.
			gunluk.Yaz("iş #%d bu turda sıraya yetişemedi — sunucu birazdan yeniden verecek", is.IsID)
			sonuclar = append(sonuclar, a.hataSonucu(is.IsID, teshis.Cumle(teshis.KUYRUK_SISTI, is.Hedef, ""), string(teshis.KUYRUK_SISTI)))
			continue
		}
		veri, err := base64.StdEncoding.DecodeString(is.IcerikB64)
		if err != nil {
			gunluk.Yaz("iş #%d içerik çözülemedi: %v", is.IsID, err)
			sonuclar = append(sonuclar, a.hataSonucu(is.IsID, "Fiş içeriği çözülemedi.", string(teshis.BILINMEYEN)))
			continue
		}
		// PII: yalnız is_id/hedef/bayt sayısı loglanır, İÇERİK ASLA.
		gunluk.Yaz("iş #%d → %s (%d bayt, tip=%s)", is.IsID, is.Hedef, len(veri), is.Tip)

		if err := a.basZamanAsimiyla(is.Hedef, veri); err != nil {
			kod := teshis.KodunuAl(err)
			gunluk.Yaz("iş #%d BASILAMADI [%s]: %v", is.IsID, string(kod), err)
			a.baskiSorunuYaz(err.Error(), string(kod), is.Hedef)
			if supheliKod(kod) {
				// ÇİFT FİŞ TUZAĞI (2026-09-22'de kapatıldı): baskı 30 saniyede
				// bitmediğinde goroutine İPTAL EDİLEMİYOR, arkada yazmaya devam
				// ediyor; kağıt takılınca o iş basıyor. Eskiden sunucuya 'hata'
				// diyorduk, sunucu 2 dakika sonra AYNI işi yeniden veriyordu ve
				// mutfağa iki fiş düşüyordu. Artık iş 'basildi' bildirilir
				// (yeniden verilmez), şüpheli deftere yazılır ve bir daha ASLA
				// basılmaz; kullanıcı durum kartındaki uyarıyı görür.
				gunluk.Yaz("iş #%d yarıda kaldı — yeniden gönderilmeyecek (çift fiş riski); kağıt çıkmadıysa panelden yeniden gönderin", is.IsID)
				a.supheliIsaretle(is.IsID)
				sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "basildi"})
				continue
			}
			sonuclar = append(sonuclar, a.hataSonucu(is.IsID, err.Error(), string(kod)))
			continue
		}
		a.basildiIsaretle(is.IsID)
		sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "basildi"})
		// YALNIZ teyitli başarılı baskı sorun alanlarını temizler.
		a.Durum.Ayarla(func(d *Durum) {
			d.SonBaski = time.Now()
			d.SonHata = ""
			d.SonBaskiSorunu = ""
			d.SonBaskiKodu = ""
			d.SonBaskiHedef = ""
			d.SonBaskiSorunuAt = time.Time{}
			d.UstUsteHata = 0
		})
	}

	if _, err := a.Istemci.SonucBildir(sonuclar); err != nil {
		// Sonuç gitmezse iş sunucuda 'gonderiliyor' kalır, 300sn sonra yeniden
		// verilir — kalkan sayesinde İKİNCİ KEZ BASILMAZ. Ayrıca sonucu kuyruğa
		// alıp bir sonraki turda tekrar göndeririz (diske de yazılır).
		gunluk.Yaz("sonuç bildirilemedi (iş yeniden verilirse tekrar basılmayacak): %v", err)
		a.bildirilmedigineEkle(sonuclar)
	}
}

// hataSonucu — 'hata' sonucu üretir. Hata metni ASLA BOŞ bırakılmaz: boş metin
// panelde "Son Hata" sütununu boş gösteriyor ve müdür neyin yanlış olduğunu
// göremiyordu.
func (a *Ajan) hataSonucu(isID int64, metin, kod string) api.Sonuc {
	if strings.TrimSpace(metin) == "" {
		metin = "Bilinmeyen yazdırma hatası"
	}
	return api.Sonuc{IsID: isID, Durum: "hata", Hata: kisalt(metin, 300), HataKodu: kod}
}

// baskiSorunuYaz — durum kartının/tepsinin göstereceği baskı sorununu işler.
func (a *Ajan) baskiSorunuYaz(metin, kod, hedef string) {
	a.Durum.Ayarla(func(d *Durum) {
		d.SonBaskiSorunu = kisalt(metin, 300)
		d.SonBaskiKodu = kod
		d.SonBaskiHedef = hedef
		d.SonBaskiSorunuAt = time.Now()
		d.UstUsteHata++
	})
}

func (a *Ajan) hataIsle(nerede string, err error) {
	gunluk.Yaz("%s hatası: %v", nerede, err)
	a.Durum.Ayarla(func(d *Durum) {
		d.Bagli = false
		d.SonHata = err.Error()
	})
	if errors.Is(err, api.ErrYetkisiz) {
		a.yetkisizArtir() // nabız 401'i de art arda sayılır → yeniden eşleştirme
	}
}

// bekleVeyaDur — süre kadar bekler; dur kapanırsa true döner.
func bekleVeyaDur(dur <-chan struct{}, sure time.Duration) bool {
	select {
	case <-dur:
		return true
	case <-time.After(sure):
		return false
	}
}

func kisalt(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func maks(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
