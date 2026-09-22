package yazdir

import (
	"errors"
	"sort"
	"strings"
	"unicode"
)

var (
	errAdGecersiz = errors.New("yazıcı adı geçersiz — \\ , ! ve kontrol karakteri kullanılamaz")
	errPortBos    = errors.New("port boş")
)

// FİZİKSEL OLARAK TAKILI AMA KURULMAMIŞ YAZICILARI BULMA.
//
// NEDEN VAR (2026-09-22): Windows, bir USB fiş yazıcısı takıldığında HER ZAMAN
// kuyruk açmaz. Sürücüsünü tanımadığında cihaz "Aygıtlar ve Yazıcılar" altında
// "Belirtilmemiş" bölümünde öylece durur; yazıcı olarak GÖRÜNMEZ, panele hiç
// düşmez. Kafe sahibinin bunu fark etmesi ve elle kuyruk açması imkânsız —
// sahada bir kullanıcı tam olarak burada takıldı ve yazıcıyı bağlayamadı.
//
// Bu dosya KARAR mantığını tutar (platformdan bağımsız, macOS/Linux'ta test
// edilir). Gerçek kurulum çağrısı kurulum_windows.go'dadır.

// KurulabilirYazici — kablosu takılı, açık, ama Windows'ta kuyruğu OLMAYAN yazıcı.
type KurulabilirYazici struct {
	Port       string // spooler port adı, ör "USB002"
	Donanim    string // kayıt defteri donanım kimliği, ör "ZiJiangZJ-80D6E4"
	OnerilenAd string // açılacak kuyruğun adı, ör "ZJ-80"
}

// KurulabilirleriBul — canlı USB portlarını kurulu kuyruklarla karşılaştırır ve
// kuyruğu olmayanları döner. Sonuç port adına göre SIRALIDIR (deterministik).
//
// KAPALI BAŞARISIZLIK: tamListe=false ise (kayıt defteri okuması eksik kaldı)
// BOŞ döner. Eksik bir listeyle "bu yazıcı kurulmamış" demek, aslında vendor
// port monitörüyle çalışan sağlam bir kuruluma ikinci bir kuyruk açmak
// demektir — kullanıcının fişleri iki yere bölünür.
func KurulabilirleriBul(canliPortlar map[string]string, tamListe bool, kurulu map[string]YaziciDurumu) []KurulabilirYazici {
	if !tamListe || len(canliPortlar) == 0 {
		return nil
	}

	// Kuyruğu olan portlar + zaten kullanılan adlar.
	doluPortlar := make(map[string]bool, len(kurulu))
	adlar := make(map[string]bool, len(kurulu))
	for ad, d := range kurulu {
		if p := strings.TrimSpace(d.Port); p != "" {
			doluPortlar[esitlemeIcinPort(p)] = true
		}
		adlar[strings.TrimSpace(ad)] = true
	}

	portlar := make([]string, 0, len(canliPortlar))
	for p := range canliPortlar {
		portlar = append(portlar, p)
	}
	sort.Strings(portlar)

	var out []KurulabilirYazici
	for _, port := range portlar {
		if doluPortlar[esitlemeIcinPort(port)] {
			continue // bu porta zaten bir kuyruk bakıyor
		}
		ad := benzersizAd(DonanimdanAd(canliPortlar[port]), adlar)
		adlar[ad] = true // aynı turda iki yazıcı aynı adı almasın
		out = append(out, KurulabilirYazici{Port: port, Donanim: canliPortlar[port], OnerilenAd: ad})
	}
	return out
}

// esitlemeIcinPort — port adlarını karşılaştırmaya hazırlar. Windows port
// adları harf duyarsızdır ("usb001" ile "USB001" AYNI porttur) ve bazı
// sürücüler sonuna boşluk bırakır.
func esitlemeIcinPort(p string) string { return kucukHarf(strings.TrimSpace(p)) }

// DonanimdanAd — kayıt defteri donanım kimliğinden okunabilir bir kuyruk adı
// üretir. "ZiJiangZJ-80D6E4" → "ZiJiangZJ-80", "Zjiang_POS-80" → "Zjiang POS-80".
//
// Kimlikler üretici+model+sağlama toplamı biçimindedir; sondaki sağlama
// (4-8 onaltılık karakter) atılır. Tanıyamazsak genel bir ada düşeriz —
// kullanıcı zaten panelden istediği adı verebilir.
func DonanimdanAd(donanim string) string {
	s := donanim
	// Kayıt defteri kimliklerinde alt çizgi ve & ayraç olarak kullanılır.
	s = strings.NewReplacer("_", " ", "&", " ", "#", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	s = sondakiSaglamayiAt(s)
	s = strings.TrimSpace(s)
	s = kisalt(s, 40)
	// Windows kuyruk adlarında YASAK karakterler: \ ve , ve !
	s = strings.NewReplacer(`\`, " ", ",", " ", "!", " ").Replace(s)
	s = strings.Join(strings.Fields(s), " ")
	if s == "" || anlamsizAdMi(s) {
		return "Fiş Yazıcısı"
	}
	return s
}

// anlamsizAdlar — Windows'un gerçek model adı yerine koyduğu yer tutucular.
//
// Sahada görüldü (2026-09-22): iki fiş yazıcısı için de kayıt defterinde
// "UnknownPrinter" yazıyordu ve kullanıcıya "UnknownPrinter" / "UnknownPrinter 2"
// diye iki kart gösterildi. 50 yaşında bir kafe sahibine bu hiçbir şey
// anlatmaz; hangi kartın hangi yazıcı olduğunu da ayırt edemez. Böyle
// durumlarda "Fiş Yazıcısı" diyoruz — kart zaten altında "USB002 girişi"
// yazdığı için ayırt etmek mümkün, ve kullanıcı panelden istediği adı verebilir.
var anlamsizAdlar = []string{
	"unknownprinter", "unknown", "unknown device", "printer", "usbprinter",
	"usb printer", "localprint", "local printer", "generic", "default",
	"dot4prt", "dot4usb", "bilinmeyen",
}

func anlamsizAdMi(ad string) bool {
	k := kucukHarf(strings.TrimSpace(ad))
	for _, a := range anlamsizAdlar {
		if k == a {
			return true
		}
	}
	return false
}

// sondakiSaglamayiAt — "ZiJiangZJ-80D6E4" → "ZiJiangZJ-80".
//
// Üç koruma birden aranır, biri bile tutmazsa ad AYNEN kalır:
//   - son 4-8 karakterin TAMAMI onaltılık,
//   - içinde en az bir onaltılık HARF (a-f) VE en az bir RAKAM var,
//   - kesildikten sonra geriye en az 3 karakter kalıyor.
//
// EN KISA eşleşme kazanır (4'ten 8'e doğru aranır): "ZJ-80D6E4" içinde
// "80D6E4" de geçerli bir onaltılık dizidir ve en uzunu alsaydık modelin
// "80" numarasını yerdik. Harf şartı da "POS-8000" gibi saf rakamlı model
// numaralarını korur — gerçek sağlama toplamları neredeyse her zaman harf içerir.
func sondakiSaglamayiAt(s string) string {
	r := []rune(s)
	for n := 4; n <= 8; n++ {
		if len(r)-n < 3 {
			break
		}
		kuyruk := r[len(r)-n:]
		if tumuOnaltilik(kuyruk) && enAzBirRakam(kuyruk) && enAzBirOnaltilikHarf(kuyruk) {
			return strings.TrimRight(string(r[:len(r)-n]), " -")
		}
	}
	return s
}

func enAzBirOnaltilikHarf(r []rune) bool {
	for _, c := range r {
		if c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' {
			return true
		}
	}
	return false
}

func tumuOnaltilik(r []rune) bool {
	for _, c := range r {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return len(r) > 0
}

func enAzBirRakam(r []rune) bool {
	for _, c := range r {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

// benzersizAd — kullanılan adlarla çakışmayan bir ad üretir: "ZJ-80", "ZJ-80 2", ...
func benzersizAd(taban string, kullanilan map[string]bool) string {
	if !kullanilan[taban] {
		return taban
	}
	for i := 2; i < 100; i++ {
		aday := taban + " " + sayiMetni(i)
		if !kullanilan[aday] {
			return aday
		}
	}
	return taban + " yeni"
}

func sayiMetni(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func kisalt(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n]))
}

// kucukHarf — ELLE küçük harfe indirger.
//
// strings.ToLower KULLANILMAZ: Türkçe yerelinde 'I' → 'ı' dönüşümü port
// karşılaştırmasını bozabilir. Burada yalnız ASCII harfler çevrilir, geri
// kalan rune'lar AYNEN korunur.
func kucukHarf(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			b.WriteRune(c + ('a' - 'A'))
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// AdGecerliMi — Windows kuyruk adı olarak kullanılabilir mi?
// Boş olamaz, 220 karakteri geçemez, \ , ! ve kontrol karakteri içeremez.
func AdGecerliMi(ad string) bool {
	ad = strings.TrimSpace(ad)
	if ad == "" || len([]rune(ad)) > 220 {
		return false
	}
	for _, c := range ad {
		if c == '\\' || c == ',' || c == '!' || c == 0 || unicode.IsControl(c) {
			return false
		}
	}
	return true
}

// Kurulabilirler — TAKILI ama Windows'ta kuyruğu olmayan yazıcıları döner.
// Windows dışında her zaman boş döner (canlı port okuması yalnız orada var).
//
// İkinci dönüş taramaGuvenilir: false ise tarama EKSİK kaldı ve öneriler bu
// yüzden susturuldu — v0.15.2'de bu sessizce oluyordu ve GERÇEKTEN takılı
// yazıcı için bile kart çıkmayınca sebebini görmenin hiçbir yolu yoktu.
// Çağıran bunu günlüğe düşer ki destek tek bakışta anlasın.
func Kurulabilirler() (liste []KurulabilirYazici, taramaGuvenilir bool) {
	canli, tam := CanliUsbPortlari()
	kurulu, err := YazicilariOku()
	if err != nil {
		// Kurulu kuyrukları okuyamadıysak ÖNERİ YAPMAYIZ: boş listeyle
		// karşılaştırmak, var olan her kuyruğu "yok" sayıp ikinci bir kuyruk
		// açmaya kalkmak demektir.
		return nil, false
	}
	return KurulabilirleriBul(canli, tam, kurulu), tam
}

// Kur — verilen porta bakan yeni bir yazıcı kuyruğu açar.
//
// Kullanıcı ONAYIYLA çağrılır (arayüzdeki "Kur" düğmesi); kendiliğinden
// çalışmaz. Kafe sahibinin bilgisayarına habersiz yazıcı eklemek, panelde
// beklenmedik hedefler ve iki yere bölünmüş fişler demektir.
func Kur(ad, port string) error {
	ad = strings.TrimSpace(ad)
	port = strings.TrimSpace(port)
	if !AdGecerliMi(ad) {
		return errAdGecersiz
	}
	if port == "" {
		return errPortBos
	}
	return kurPlatform(ad, port)
}

// KurCocukSurec — YÜKSELTİLMİŞ çocuk süreçten çağrılır (--yazici-kur bayrağı).
// Kur ile aynı doğrulamadan geçer: yükseltilmiş süreç, doğrulanmamış girdiyle
// çalıştırılabilecek en yanlış yerdir.
func KurCocukSurec(ad, port string) error {
	ad = strings.TrimSpace(ad)
	port = strings.TrimSpace(port)
	if !AdGecerliMi(ad) {
		return errAdGecersiz
	}
	if port == "" {
		return errPortBos
	}
	return kurCocukSurecPlatform(ad, port)
}
