// Package kopru — ajanın çekirdek çalışma döngüsü (tray'den bağımsız, test edilebilir).
package kopru

import (
	"encoding/base64"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
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
	kayitKilit   sync.Mutex

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

func (a *Ajan) basildiIsaretle(isID int64) {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	a.basildiKayit[isID] = time.Now()
	// 1 saatten eski kayıtları temizle (bellek sınırlı tut).
	if len(a.basildiKayit) > 500 {
		sinir := time.Now().Add(-time.Hour)
		for id, t := range a.basildiKayit {
			if t.Before(sinir) {
				delete(a.basildiKayit, id)
			}
		}
	}
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

func (a *Ajan) isleriBas(isler []api.Is) {
	sonuclar := make([]api.Sonuc, 0, len(isler))
	for _, is := range isler {
		// ÇİFT BASKI KALKANI: sunucu aynı işi tekrar verdiyse basma, sadece bildir.
		if a.zatenBasildi(is.IsID) {
			gunluk.Yaz("iş #%d zaten basılmıştı — tekrar basılmadı, sonuç yeniden bildiriliyor", is.IsID)
			sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "basildi"})
			continue
		}
		veri, err := base64.StdEncoding.DecodeString(is.IcerikB64)
		if err != nil {
			gunluk.Yaz("iş #%d içerik çözülemedi: %v", is.IsID, err)
			sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "hata", Hata: "içerik çözülemedi"})
			continue
		}
		// PII: yalnız is_id/hedef/bayt sayısı loglanır, İÇERİK ASLA.
		gunluk.Yaz("iş #%d → %s (%d bayt, tip=%s)", is.IsID, is.Hedef, len(veri), is.Tip)
		if err := a.Bas(is.Hedef, veri); err != nil {
			gunluk.Yaz("iş #%d BASILAMADI: %v", is.IsID, err)
			sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "hata", Hata: kisalt(err.Error(), 300)})
			a.Durum.Ayarla(func(d *Durum) { d.SonHata = err.Error() })
			continue
		}
		a.basildiIsaretle(is.IsID)
		sonuclar = append(sonuclar, api.Sonuc{IsID: is.IsID, Durum: "basildi"})
		a.Durum.Ayarla(func(d *Durum) { d.SonBaski = time.Now(); d.SonHata = "" })
	}
	if _, err := a.Istemci.SonucBildir(sonuclar); err != nil {
		// Sonuç gitmezse iş sunucuda 'gonderiliyor' kalır, 300sn sonra yeniden
		// verilir — ama kalkan sayesinde İKİNCİ KEZ BASILMAZ.
		gunluk.Yaz("sonuç bildirilemedi (iş yeniden verilirse tekrar basılmayacak): %v", err)
	}
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
