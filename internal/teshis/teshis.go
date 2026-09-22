// Package teshis — yazıcı sorunlarının PLATFORMDAN BAĞIMSIZ katalogu.
//
// NEDEN AYRI PAKET: buradaki her şey SAF fonksiyondur (build tag YOK). Windows'a
// özgü bit maskeleri, port adları ve kuyruk durumları Windows dosyalarında
// OKUNUR, ama KARAR burada verilir. Böylece "hangi bit hangi Türkçe cümleye
// dönüşür" sorusu macOS/Linux'ta `go test ./...` ile gerçekten sınanabilir —
// gerçek 32-bit Windows kasası beklemeden.
//
// DİL KURALI: kafe sahibi teknik terim OKUMAZ. Her cümle TEK satır, TEK cümle,
// en fazla 140 karakter ve "ne olmuş + ne yapmalıyım" içerir. metinGuvenli()
// panelde/işletmede kafa karıştıran birkaç kelimeyi (eski servis adı vb.)
// süzer; karşılaştırma ELLE harf haritasıyla yapılır çünkü strings.ToLower
// Türkçe'de 'I' harfini 'i' yapar ve "yapılandır"/"YAPILANDIR" eşleşmesini
// kaçırır (klasik I/ı tuzağı).
package teshis

import (
	"errors"
	"strings"
)

// Kod — makine tarafında taşınan sorun kimliği. Sunucuya `hata_kodu` olarak
// gider; kullanıcıya GÖSTERİLMEZ (kullanıcı Cumle'yi görür).
type Kod string

// Sorun kodları. Yeni kod eklendiğinde TumKodlar'a da eklenmeli (tablo testi
// tüm kodların cümlesini doğrular).
const (
	KAGIT_YOK           Kod = "KAGIT_YOK"
	KAPAK_ACIK          Kod = "KAPAK_ACIK"
	YAZICI_KAPALI       Kod = "YAZICI_KAPALI"
	KUYRUK_DURAKLATILDI Kod = "KUYRUK_DURAKLATILDI"
	CEVRIMDISI_ISARETLI Kod = "CEVRIMDISI_ISARETLI"
	SANAL_HEDEF         Kod = "SANAL_HEDEF"
	HEDEF_YOK           Kod = "HEDEF_YOK"
	HEDEF_BOS           Kod = "HEDEF_BOS"
	PORT_OLU            Kod = "PORT_OLU"
	VERI_TURU_RED       Kod = "VERI_TURU_RED"
	YETKI_YOK           Kod = "YETKI_YOK"
	DISK_DOLU           Kod = "DISK_DOLU"
	ASILDI              Kod = "ASILDI"
	YARIM_YAZILDI       Kod = "YARIM_YAZILDI"
	AG_ULASILAMIYOR     Kod = "AG_ULASILAMIYOR"
	AG_YANLIS_CIHAZ     Kod = "AG_YANLIS_CIHAZ"
	KUYRUK_SISTI        Kod = "KUYRUK_SISTI"
	BELIRSIZ_TESLIM     Kod = "BELIRSIZ_TESLIM"
	BILINMEYEN          Kod = "BILINMEYEN"
)

// TumKodlar — katalogdaki bütün kodlar (tablo testi bunları gezer).
func TumKodlar() []Kod {
	return []Kod{
		KAGIT_YOK, KAPAK_ACIK, YAZICI_KAPALI, KUYRUK_DURAKLATILDI,
		CEVRIMDISI_ISARETLI, SANAL_HEDEF, HEDEF_YOK, HEDEF_BOS, PORT_OLU,
		VERI_TURU_RED, YETKI_YOK, DISK_DOLU, ASILDI, YARIM_YAZILDI,
		AG_ULASILAMIYOR, AG_YANLIS_CIHAZ, KUYRUK_SISTI, BELIRSIZ_TESLIM,
		BILINMEYEN,
	}
}

// Eylem — durum penceresindeki TEK düğmenin ne yapacağı.
const (
	EylemOnar      = "ONAR"       // ajan geri alınabilir bir onarım deneyebilir
	EylemYaziciSec = "YAZICI_SEC" // kullanıcı panelden başka yazıcı seçmeli
	EylemYok       = "YOK"        // düğme gösterilmez (fiziksel müdahale gerekir)
)

// Bulgu — bir sorunun tam tanımı (katalog satırı).
type Bulgu struct {
	Kod   Kod
	Cumle string
	Eylem string
	// Onarilabilir — ajan bu sorunu KENDİ düzeltmeyi deneyebilir mi?
	Onarilabilir bool
	// Kalici — kullanıcı müdahalesi olmadan kendiliğinden geçmez mi?
	// (kağıt yok kalıcıdır; ağ zaman aşımı değildir.)
	Kalici bool
}

// OnarimKaydi — yapılan bir onarımın geri alınabilir kaydı (defter satırı).
type OnarimKaydi struct {
	Zaman     string
	Yazici    string
	NeYaptim  string
	EskiDeger string
	GeriAlKod string
}

// katalog — kod → (şablon, eylem, onarılabilir, kalıcı).
// Şablondaki %AD% yazıcı adıyla değiştirilir (fmt yerine düz değiştirme:
// kullanıcı yazıcı adında '%' kullanırsa fmt bozulmasın).
var katalog = map[Kod]Bulgu{
	KAGIT_YOK: {Cumle: "%AD% yazıcısında kağıt bitti — yeni rulo takıp kapağı kapatın, fiş kendiliğinden yeniden basılacak.",
		Eylem: EylemYok, Kalici: true},
	KAPAK_ACIK: {Cumle: "%AD% yazıcısının kapağı açık — kapağı tam kapatın, fiş kendiliğinden yeniden basılacak.",
		Eylem: EylemYok, Kalici: true},
	YAZICI_KAPALI: {Cumle: "%AD% yazıcısı kapalı görünüyor — açma düğmesini ve kablosunu kontrol edin.",
		Eylem: EylemYok, Kalici: true},
	KUYRUK_DURAKLATILDI: {Cumle: "%AD% baskı sırası duraklatılmış — duraklatmayı kaldırmayı deneyebiliriz.",
		Eylem: EylemOnar, Onarilabilir: true, Kalici: true},
	CEVRIMDISI_ISARETLI: {Cumle: "%AD% Windows'ta çevrimdışı kullanılıyor işaretli — bu işareti kaldırmayı deneyebiliriz.",
		Eylem: EylemOnar, Onarilabilir: true, Kalici: true},
	SANAL_HEDEF: {Cumle: "%AD% kağıda değil dosyaya yazıyor, fiş çıkmaz — panelden gerçek fiş yazıcısını seçin.",
		Eylem: EylemYaziciSec, Kalici: true},
	HEDEF_YOK: {Cumle: "%AD% adında bir yazıcı bu bilgisayarda yok — panelden doğru yazıcıyı seçin.",
		Eylem: EylemYaziciSec, Kalici: true},
	HEDEF_BOS: {Cumle: "Bu yazıcıya hedef atanmamış — panelden fiş yazıcısını seçin.",
		Eylem: EylemYaziciSec, Kalici: true},
	PORT_OLU: {Cumle: "%AD% yazıcısının USB girişi şu an boşta — kabloyu takıp yazıcıyı açın.",
		Eylem: EylemYok, Kalici: true},
	VERI_TURU_RED: {Cumle: "%AD% sürücüsü ham fiş verisini kabul etmedi — panelden başka bir yazıcı seçmeniz gerekebilir.",
		Eylem: EylemYaziciSec, Kalici: true},
	YETKI_YOK: {Cumle: "%AD% ayarlarını değiştirme yetkimiz yok — yazıcıya sağ tıklayıp Yazıcıyı Çevrimiçi Kullan deyin.",
		Eylem: EylemYok, Kalici: true},
	DISK_DOLU: {Cumle: "Baskı sırası için bilgisayarda yer kalmadı — biraz disk alanı açın.",
		Eylem: EylemYok, Kalici: true},
	ASILDI: {Cumle: "%AD% yanıt vermedi, baskı bitmedi — yazıcıyı kapatıp açın, kağıt çıkmadıysa panelden yeniden gönderin.",
		Eylem: EylemYok, Kalici: false},
	YARIM_YAZILDI: {Cumle: "%AD% yazıcısına fiş yarım gitti — çift fiş riskine karşı kendiliğinden yeniden denenmez, kağıt çıkmadıysa panelden gönderin.",
		Eylem: EylemYok, Kalici: false},
	AG_ULASILAMIYOR: {Cumle: "%AD% adresine ulaşılamıyor — yazıcı kapalı olabilir veya ağda görünmüyor.",
		Eylem: EylemYok, Kalici: false},
	AG_YANLIS_CIHAZ: {Cumle: "%AD% adresi cevap veriyor ama fiş girişi kapalı — adres başka bir cihaza ait olabilir.",
		Eylem: EylemYaziciSec, Kalici: true},
	KUYRUK_SISTI: {Cumle: "%AD% sırasında çok fazla bekleyen iş birikmiş — sırayı temizlemek gerekebilir.",
		Eylem: EylemYok, Kalici: true},
	BELIRSIZ_TESLIM: {Cumle: "%AD% fişi aldı ama kağıda döktüğünü doğrulamadı — kağıt çıkmadıysa panelden yeniden gönderin.",
		Eylem: EylemYok, Kalici: false},
	BILINMEYEN: {Cumle: "%AD% üzerinde beklenmeyen bir baskı sorunu oldu — ayrıntı için günlüğe bakın.",
		Eylem: EylemYok, Kalici: false},
}

// BulguAl — kod için katalog satırını döndürür (cümle doldurulmuş hâlde).
func BulguAl(kod Kod, yaziciAd, ek string) Bulgu {
	b, varmi := katalog[kod]
	if !varmi {
		b = katalog[BILINMEYEN]
		kod = BILINMEYEN
	}
	b.Kod = kod
	b.Cumle = Cumle(kod, yaziciAd, ek)
	return b
}

// Cumle — kullanıcıya gösterilecek TEK satırlık, TEK cümlelik Türkçe metin.
// En fazla 140 rune; satır sonu içermez; yasaklı kelimeler süzülür.
func Cumle(kod Kod, yaziciAd, ek string) string {
	b, varmi := katalog[kod]
	if !varmi {
		b = katalog[BILINMEYEN]
	}
	ad := strings.TrimSpace(yaziciAd)
	if ad == "" {
		ad = "Yazıcı"
	}
	metin := strings.ReplaceAll(b.Cumle, "%AD%", ad)
	if ek = strings.TrimSpace(ek); ek != "" {
		metin = strings.TrimSuffix(metin, ".") + "; " + ek + "."
	}
	metin = tekSatir(metin)
	metin = metinGuvenli(metin)
	return kisalt(metin, 140)
}

// ── Yasaklı kelime süzgeci ────────────────────────────────────────────────

// yasakli — süzülecek alt-dizgiler ve yerlerine konacak güvenli eşanlamlıları.
// Anahtarlar NORMALLEŞTİRİLMİŞ (kucukTR + i-ailesi 'i'ye indirgenmiş) hâlde
// yazılır; eşleşme de normalleştirilmiş metin üzerinde yapılır.
var yasakli = []struct {
	desen  string
	yerine string
}{
	{"printnode", "eski servis"},
	{"yapilandir", "ayarla"},
	{"bağlan", "ulaş"},
	{"bayat", "eski"},
}

// metinGuvenli — yasaklı alt-dizgileri güvenli eşanlamlıyla değiştirir.
//
// strings.ToLower KULLANILMAZ: Türkçe'de 'I' harfinin küçüğü 'ı'dır ama Go'nun
// ToLower'ı 'i' üretir; "YAPILANDIR" → "yapilandir" olur, "yapılandır" ise
// olduğu gibi kalır ve iki varyant BİRBİRİNE EŞLEŞMEZ. Bu yüzden elle harf
// haritası kullanıp i/ı/İ/I'yı TEK harfe indiriyoruz — üç varyant da yakalanır.
func metinGuvenli(s string) string {
	for _, y := range yasakli {
		for {
			bas, son := normalBul(s, y.desen)
			if bas < 0 {
				break
			}
			s = s[:bas] + y.yerine + s[son:]
		}
	}
	return s
}

// normalBul — s içinde (normalleştirilmiş karşılaştırmayla) desen'i arar;
// bulursa ORİJİNAL s üzerindeki bayt aralığını döndürür, yoksa (-1, -1).
func normalBul(s, desen string) (int, int) {
	runeler := []rune(s)
	// normal[i] = runeler[i]'nin normalleştirilmiş hâli; indeksler birebir eşleşir
	// (her rune tek runa iner), bu yüzden eşleşme aralığını geri çevirebiliriz.
	normal := make([]rune, len(runeler))
	baytBasi := make([]int, len(runeler)+1)
	sayac := 0
	for i, r := range runeler {
		normal[i] = normalRune(r)
		baytBasi[i] = sayac
		sayac += len(string(r))
	}
	baytBasi[len(runeler)] = sayac

	d := []rune(desen)
	if len(d) == 0 || len(d) > len(normal) {
		return -1, -1
	}
	for i := 0; i+len(d) <= len(normal); i++ {
		esit := true
		for j := 0; j < len(d); j++ {
			if normal[i+j] != d[j] {
				esit = false
				break
			}
		}
		if esit {
			return baytBasi[i], baytBasi[i+len(d)]
		}
	}
	return -1, -1
}

// normalRune — Türkçe duyarlı küçültme + i-ailesini tek harfe indirme.
func normalRune(r rune) rune {
	switch r {
	case 'İ', 'I', 'ı', 'i':
		return 'i'
	case 'Ş':
		return 'ş'
	case 'Ğ':
		return 'ğ'
	case 'Ü':
		return 'ü'
	case 'Ö':
		return 'ö'
	case 'Ç':
		return 'ç'
	}
	if r >= 'A' && r <= 'Z' {
		return r - 'A' + 'a'
	}
	return r
}

// ── Küçük yardımcılar (Go 1.20: builtin min/max YOK) ─────────────────────

// kisalt — n rune'dan uzunsa keser (bayt değil RUNE sayar: Türkçe harfler
// ortadan bölünmesin).
func kisalt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// enAz — iki tamsayının küçüğü. Go 1.20'de builtin min YOK (eski kanal
// derlemesi Go 1.20 ile yapılıyor, bkz. .github/workflows/ci.yml).
func enAz(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// enCok — iki tamsayının büyüğü (builtin max Go 1.21+).
func enCok(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func tekSatir(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

// ── Hata tipi ─────────────────────────────────────────────────────────────

// Hata — kodlu baskı hatası. yazdir paketi bunu döndürür, kopru döngüsü
// errors.As ile kodu çıkarıp sunucuya `hata_kodu` olarak bildirir.
type Hata struct {
	Kod   Kod
	Metin string
	Alt   error
}

func (h *Hata) Error() string { return h.Metin }
func (h *Hata) Unwrap() error { return h.Alt }

// Yeni — kodlu hata üretir. Cümle katalogdan gelir, ham hata Alt'ta saklanır
// (günlüğe teknik ayrıntı yazılabilsin diye).
func Yeni(kod Kod, yaziciAd, ek string, alt error) *Hata {
	return &Hata{Kod: kod, Metin: Cumle(kod, yaziciAd, ek), Alt: alt}
}

// KodunuAl — hatadan sorun kodunu çıkarır; kodlu değilse BILINMEYEN döner.
func KodunuAl(err error) Kod {
	if err == nil {
		return ""
	}
	var h *Hata
	if errors.As(err, &h) {
		return h.Kod
	}
	return BILINMEYEN
}

// ── JOB_STATUS bit maskesi → Kod (SAF; Windows'a bağımlı DEĞİL) ───────────

// Windows JOB_STATUS bitleri. Burada ELLE tanımlıdır ki bu dosya her platformda
// derlensin ve karar mantığı macOS'ta test edilebilsin (değerler winspool.h).
const (
	IsBitiPAUSED            uint32 = 0x00000001
	IsBitiERROR             uint32 = 0x00000002
	IsBitiDELETING          uint32 = 0x00000004
	IsBitiSPOOLING          uint32 = 0x00000008
	IsBitiPRINTING          uint32 = 0x00000010
	IsBitiOFFLINE           uint32 = 0x00000020
	IsBitiPAPEROUT          uint32 = 0x00000040
	IsBitiPRINTED           uint32 = 0x00000080
	IsBitiDELETED           uint32 = 0x00000100
	IsBitiBLOCKED_DEVQ      uint32 = 0x00000200
	IsBitiUSER_INTERVENTION uint32 = 0x00000400
	IsBitiCOMPLETE          uint32 = 0x00001000
)

// IsTeslimEdildi — bu bitler "kağıda döküldü/cihaza teslim edildi" diyor mu?
func IsTeslimEdildi(bitler uint32) bool {
	return bitler&(IsBitiPRINTED|IsBitiCOMPLETE) != 0
}

// IsHataKodu — iş durum bitlerinden GERÇEK hata kodunu çıkarır.
// İkinci dönüş false ise hata yok (iş yolunda veya bitmiş).
//
// Sıra ÖNEMLİ: en açıklayıcı fiziksel sebep önce gelir. Kağıt bittiğinde
// spooler genelde PAPEROUT|OFFLINE|ERROR üçünü birden set eder; kullanıcıya
// "bilinmeyen hata" değil "kağıt bitti" demeliyiz.
func IsHataKodu(bitler uint32) (Kod, bool) {
	switch {
	case bitler&IsBitiPAPEROUT != 0:
		return KAGIT_YOK, true
	case bitler&IsBitiUSER_INTERVENTION != 0:
		// "Kullanıcı müdahalesi gerekiyor" — fiş yazıcılarında bu neredeyse her
		// zaman açık kapak / sıkışmış kağıttır.
		return KAPAK_ACIK, true
	case bitler&IsBitiOFFLINE != 0:
		return YAZICI_KAPALI, true
	case bitler&IsBitiPAUSED != 0:
		return KUYRUK_DURAKLATILDI, true
	case bitler&IsBitiBLOCKED_DEVQ != 0:
		// Sürücü işi basamıyor — ham veri türü reddi ile aynı kullanıcı eylemi.
		return VERI_TURU_RED, true
	case bitler&IsBitiERROR != 0:
		return BILINMEYEN, true
	}
	return "", false
}

// IsKarari — teslim teyidi sonucu.
type IsKarari int

const (
	KararBekliyor IsKarari = iota // iş hâlâ sırada, yoklamaya devam
	KararBasarili                 // kağıda döküldüğüne yeterince kanıt var
	KararHata                     // gerçek hata; iş 'hata' bildirilmeli
	KararBelirsiz                 // teslim doğrulanamadı; 'basildi' denir ama günlüğe not düşülür
)

// TeslimKarari — EnumJobs sonucundan iş teslim edildi mi kararı (SAF).
//
//	listedeVar : bizim iş kimliğimiz kuyrukta hâlâ duruyor mu
//	bitler     : duruyorsa JOB_INFO_1.StatusCode
//	listeDoldu : jobsReturned istenen üst sınıra (255) dayandı mı
//
// "Listede yok = başarılı" kuralı KUYRUK DOLUYKEN UYGULANMAZ: liste kesilmiş
// olabilir, yani işimiz aslında sırada beklerken "basıldı" sanılırdı. Bu,
// sessiz kâğıtsızlığın en sinsi hâliydi.
func TeslimKarari(listedeVar bool, bitler uint32, listeDoldu bool) (IsKarari, Kod) {
	if !listedeVar {
		if listeDoldu {
			return KararBelirsiz, KUYRUK_SISTI
		}
		return KararBasarili, ""
	}
	if IsTeslimEdildi(bitler) {
		return KararBasarili, ""
	}
	if kod, hataVar := IsHataKodu(bitler); hataVar {
		return KararHata, kod
	}
	return KararBekliyor, ""
}
