// Package yazdir — ESC/POS baytlarını fiziksel yazıcıya iletir.
//
// İki yol var, hedefin ŞEKLİNDEN seçilir:
//   - "192.168.1.50:9100" → ham TCP soketi (JetDirect). Sürücü GEREKMEZ.
//   - "POS-80"            → Windows spooler'a RAW iş. Sürücü kurulu olmalı ama
//     RAW datatype sürücünün render'ını BYPASS eder → ESC/POS bozulmadan gider.
//
// TEMEL KURAL (2026-09-22): "yazıcıya verdim" ≠ "kağıt çıktı". Bu paketteki her
// yol, hata döndürmeden önce elindeki EN GÜÇLÜ teslim kanıtını arar; kanıt yoksa
// bunu gizlemez (bkz. teshis.BELIRSIZ_TESLIM). Sunucu, 'hata' bildirilen işi 2
// dakika sonra kendiliğinden yeniden verir — yalan söylemediğimiz sürece kafe
// sahibinin hiçbir şey yapması gerekmez.
package yazdir

import (
	"errors"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// spoolerYaz — Windows spooler'a RAW yazan fonksiyon. Testte değiştirilebilsin
// diye değişken (windows dışı derlemede CUPS/lp yoluna bağlanır).
var spoolerYaz = spoolerYazPlatform

// ErrBosHedef — yazıcıya hedef atanmamış (panelde yapılandırma eksik).
var ErrBosHedef = errors.New("yazdir: hedef boş — panelden yazıcıya hedef atayın")

const (
	agBaglantiTimeout = 5 * time.Second
	agYazmaTimeout    = 10 * time.Second
	// Klon yazıcılar (Zjiang vb.) soketi kapatınca son baytları düşürebiliyor;
	// kapatmadan önce kısa bekleme veriyoruz.
	agKapatmaBeklemesi = 300 * time.Millisecond
	// agDurumOkumaSuresi — DLE EOT cevabını beklediğimiz süre. Cevap gelmezse
	// hata SAYILMAZ; yazıcı bu komutu desteklemiyor olabilir.
	agDurumOkumaSuresi = 800 * time.Millisecond
	// sessizHedefSuresi — DLE EOT'a cevap vermeyen hedefi bu süre boyunca bir
	// daha sorgulamayız; her fişe 1,6 saniye eklemesin.
	sessizHedefSuresi = 30 * time.Minute
)

// agDeneme — TEŞHİS/TEST sayacı: son Bas çağrısında kaç kez bağlantı denendi.
// Üretimde yalnız günlüğe bilgi verir; karar mantığı buna bakmaz.
var (
	agDenemeKilit sync.Mutex
	agDeneme      int
)

func agDenemeSifirla() {
	agDenemeKilit.Lock()
	agDeneme = 0
	agDenemeKilit.Unlock()
}

func agDenemeArtir() {
	agDenemeKilit.Lock()
	agDeneme++
	agDenemeKilit.Unlock()
}

// AgDenemeSayisi — son baskıdaki bağlantı denemesi sayısı (test/teşhis).
func AgDenemeSayisi() int {
	agDenemeKilit.Lock()
	defer agDenemeKilit.Unlock()
	return agDeneme
}

// sessizHedefler — DLE EOT'a cevap vermeyen hedefler ve işaretlenme zamanı.
var (
	sessizKilit sync.Mutex
	sessizler   = map[string]time.Time{}
)

func sessizMi(hedef string) bool {
	sessizKilit.Lock()
	defer sessizKilit.Unlock()
	t, varmi := sessizler[hedef]
	return varmi && time.Since(t) < sessizHedefSuresi
}

func sessizIsaretle(hedef string) {
	sessizKilit.Lock()
	sessizler[hedef] = time.Now()
	sessizKilit.Unlock()
}

// AgHedefiMi — "host:port" biçimi mi? Karar teshis paketindedir (saf + testli);
// buradaki sarmalayıcı geriye uyum içindir.
func AgHedefiMi(hedef string) bool { return teshis.AgHedefiMi(hedef) }

// Bas — baytları hedefe iletir. Hata dönerse iş 'hata' olarak bildirilir ve
// sunucu işi yeniden verir. Dönen hata KODLUDUR (teshis.Hata) — durum penceresi
// ve tepsi bu koddan tek cümlelik Türkçe açıklama ve tek düğme üretir.
func Bas(hedef string, veri []byte) error {
	agDenemeSifirla()
	temiz, portsuzIP, hedefHatasi := teshis.HedefNormalize(hedef)
	if hedefHatasi != nil {
		return teshis.Yeni(teshis.HEDEF_BOS, strings.TrimSpace(hedef), "", ErrBosHedef)
	}
	if teshis.AgHedefiMi(temiz) {
		return agaBas(temiz, veri)
	}
	// PORTSUZ IP: panele "192.168.1.50" yazılmış ama port unutulmuş. Aynı adda
	// yerel bir kuyruk YOKSA fiş yazıcılarının standart portunu (9100) ekleyip
	// TEK deneme yaparız. KALICI DEĞİŞİKLİK YAPILMAZ — yalnız günlüğe not düşer
	// ve hata metninde öneri olarak görünür; paneldeki hedefi kullanıcı düzeltir.
	if portsuzIP && !yerelKuyrukVar(temiz) {
		gunluk.Yaz("hedef '%s' portsuz IP — 9100 portu eklenerek denendi (panelde '%s:9100' yazmanız önerilir)", temiz, temiz)
		if err := agaBas(temiz+":9100", veri); err != nil {
			return err
		}
		return nil
	}
	return spoolerYaz(temiz, veri)
}

// yerelKuyrukVar — bu adda bir yazıcı kuyruğu var mı? (önbellekten okur)
func yerelKuyrukVar(ad string) bool {
	yazicilar, err := YazicilariOku()
	if err != nil || yazicilar == nil {
		return false
	}
	_, varmi := yazicilar[ad]
	return varmi
}

// agaBas — ağ yazıcısına baskı. YALNIZ "ulaşılamıyor" (zaman aşımı) sınıfında
// kısa yeniden deneme yapar: bağlantı kurulmadan önce başarısız olduğumuz için
// çift fiş riski YOKTUR. Reddedilen bağlantıda (yanlış cihaz/port) yeniden
// denemek anlamsızdır, denenmez.
func agaBas(hedef string, veri []byte) error {
	geciktirmeler := []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond}
	var son error
	for i := 0; ; i++ {
		son = agaBasTek(hedef, veri)
		if son == nil {
			return nil
		}
		if teshis.KodunuAl(son) != teshis.AG_ULASILAMIYOR || i >= len(geciktirmeler) {
			return son
		}
		time.Sleep(geciktirmeler[i])
	}
}

func agaBasTek(hedef string, veri []byte) error {
	agDenemeArtir()
	baglanti, err := net.DialTimeout("tcp", hedef, agBaglantiTimeout)
	if err != nil {
		return agBaglantiHatasi(hedef, err)
	}
	defer baglanti.Close()

	// BASKIDAN ÖNCE CİHAZA SOR: kağıt bittiyse fişi HİÇ gönderme. Yoksa yazıcı
	// fişleri tamponlar ve kullanıcı rulo takınca hepsi birden dökülür.
	if !sessizMi(hedef) {
		kagitYok, kapakAcik, cevapVar := cihazDurumunuSor(baglanti)
		if !cevapVar {
			sessizIsaretle(hedef)
			gunluk.YazSessiz("hedef %s durum sorgusuna cevap vermedi — cihaz onayı yok, baskı yine de denenecek", hedef)
		}
		if cevapVar && kagitYok {
			return teshis.Yeni(teshis.KAGIT_YOK, hedef, "", nil)
		}
		if cevapVar && kapakAcik {
			return teshis.Yeni(teshis.KAPAK_ACIK, hedef, "", nil)
		}
	}

	if err := baglanti.SetWriteDeadline(time.Now().Add(agYazmaTimeout)); err != nil {
		return teshis.Yeni(teshis.BILINMEYEN, hedef, "yazma süresi ayarlanamadı", err)
	}
	yazilan, err := baglanti.Write(veri)
	if err != nil {
		if yazilan > 0 {
			// Kısmi yazım: KÖR YENİDEN DENEME YASAK (yarım + tam fiş = çift fiş).
			return teshis.Yeni(teshis.YARIM_YAZILDI, hedef, "", err)
		}
		return teshis.Yeni(teshis.AG_ULASILAMIYOR, hedef, "", err)
	}
	if yazilan != len(veri) {
		return teshis.Yeni(teshis.YARIM_YAZILDI, hedef, "", nil)
	}
	// Yazıcı ACK vermez; baytların çıkması için kısa bekleme.
	time.Sleep(agKapatmaBeklemesi)
	return nil
}

// cihazDurumunuSor — DLE EOT n=4 (kağıt sensörü) ve n=2 (çevrimdışı sebebi)
// sorar. cevapVar=false → yazıcı bu komutu desteklemiyor; bu TEK BAŞINA hata
// DEĞİLDİR, yalnız "cihaz onayı yok" demektir.
func cihazDurumunuSor(baglanti net.Conn) (kagitYok, kapakAcik, cevapVar bool) {
	for _, n := range []byte{DleEotKagitSensoru, DleEotCevrimdisiSebep} {
		if err := baglanti.SetWriteDeadline(time.Now().Add(agDurumOkumaSuresi)); err != nil {
			return false, false, cevapVar
		}
		if _, err := baglanti.Write(DleEotSorgu(n)); err != nil {
			return false, false, cevapVar
		}
		if err := baglanti.SetReadDeadline(time.Now().Add(agDurumOkumaSuresi)); err != nil {
			return false, false, cevapVar
		}
		tampon := make([]byte, 1)
		if _, err := baglanti.Read(tampon); err != nil {
			return kagitYok, kapakAcik, cevapVar
		}
		kg, kp, _, gecerli := DurumBiti(n, tampon[0])
		if !gecerli {
			continue
		}
		cevapVar = true
		kagitYok = kagitYok || kg
		kapakAcik = kapakAcik || kp
	}
	// Okuma bitti: sonraki yazımlar için süre sınırlarını temizle.
	_ = baglanti.SetReadDeadline(time.Time{})
	return kagitYok, kapakAcik, cevapVar
}

// agBaglantiHatasi — bağlantı hatasını ÜÇ sınıfa ayırır. Sınıf, yeniden deneme
// kararını da belirler (yalnız AG_ULASILAMIYOR yeniden denenir).
func agBaglantiHatasi(hedef string, err error) error {
	var agHatasi net.Error
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		// Adres CANLI ama 9100 portu kapalı → büyük ihtimalle başka bir cihaz
		// (router, NAS, başka bir bilgisayar). Yeniden denemek anlamsız.
		return teshis.Yeni(teshis.AG_YANLIS_CIHAZ, hedef, "", err)
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return teshis.Yeni(teshis.AG_ULASILAMIYOR, hedef, "yazıcı farklı bir ağda olabilir", err)
	case errors.As(err, &agHatasi) && agHatasi.Timeout():
		return teshis.Yeni(teshis.AG_ULASILAMIYOR, hedef, "", err)
	}
	return teshis.Yeni(teshis.AG_ULASILAMIYOR, hedef, "", err)
}
