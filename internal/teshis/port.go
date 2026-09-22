package teshis

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// Bu dosya SAF karar fonksiyonlarıdır: Windows'a HİÇ bağımlı değildir, veriyi
// (port adı, canlı port kümesi, hedef metni) Windows dosyaları toplar, KARARI
// burası verir. Böylece "USB001 ölü mü", "nul: sanal mı", "192.168.1.50 portsuz
// IP mi" soruları macOS'ta birim testiyle sınanır.

// Port sınıfları.
const (
	PortUSB   = "usb"
	PortDosya = "dosya" // dosyaya/sanal aygıta yazan port: kağıt ASLA çıkmaz
	PortAg    = "ag"
	PortDiger = "diger"
)

// sanalPortOnekleri — bu portlara giden iş spooler'da "basıldı" görünür ama
// KAĞIT ÇIKMAZ. En sinsisi 'nul:' — iş anında tamamlanmış sayılıp buharlaşır.
var sanalPortOnekleri = []string{"file:", "portprompt:", "nul:", "nul", "xpsport:", "shrfax:", "microsoft.office"}

// PortSinifi — kuyruğun port adını sınıflandırır (harf duyarsız).
func PortSinifi(port string) string {
	p := strings.TrimSpace(port)
	if p == "" {
		return PortDiger
	}
	n := normalMetin(p)
	for _, onek := range sanalPortOnekleri {
		if n == onek || strings.HasPrefix(n, onek) {
			return PortDosya
		}
	}
	if usbPortuMu(n) {
		return PortUSB
	}
	// Ağ portları: "IP_192.168.1.50", "WSD-...", düz IP veya host:port.
	if strings.HasPrefix(n, "ip_") || strings.HasPrefix(n, "wsd") {
		return PortAg
	}
	if net.ParseIP(p) != nil {
		return PortAg
	}
	if AgHedefiMi(p) {
		return PortAg
	}
	return PortDiger
}

// bilinenYaziciPortlari — "host:port" biçiminin gerçekten bir AĞ YAZICISI
// olduğuna karar verirken güvendiğimiz port numaraları (JetDirect 9100 ve
// kardeşleri, LPD 515, IPP 631).
var bilinenYaziciPortlari = map[int]bool{9100: true, 9101: true, 9102: true, 515: true, 631: true}

// AgHedefiMi — hedef "host:port" biçiminde bir AĞ yazıcısı mı?
//
// Yalnız "iki nokta var + sayı var" YETMEZ: Windows'ta yazıcı adında iki nokta
// olabilir ("Kasa:2") ve o ad spooler kuyruğudur, ağ adresi DEĞİL. Bu yüzden
// host ya gerçek bir IP olmalı, ya nokta içeren bir alan adı olmalı, ya da port
// bilinen bir yazıcı portu olmalı. Aksi halde yerel kuyruk kabul edilir —
// yanlış dalda "bağlanılamıyor" hatası vermektense spooler'ı denemek doğrudur.
func AgHedefiMi(hedef string) bool {
	host, portMetni, err := net.SplitHostPort(strings.TrimSpace(hedef))
	if err != nil || host == "" {
		return false
	}
	no, err := strconv.Atoi(portMetni)
	if err != nil || no <= 0 || no > 65535 {
		return false
	}
	if net.ParseIP(host) != nil || strings.Contains(host, ".") {
		return true
	}
	return bilinenYaziciPortlari[no]
}

// usbPortuMu — tam olarak USB + rakamlar mı ("USB001", "usb3")?
func usbPortuMu(normal string) bool {
	if !strings.HasPrefix(normal, "usb") {
		return false
	}
	kalan := normal[3:]
	if kalan == "" {
		return false
	}
	for _, r := range kalan {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// UsbPortOlu — kuyruğun USB portu artık sistemde var mı?
//
// KAPALI BAŞARISIZLIK: canlı port kümesi BOŞSA (kayıt defteri okunamadı, ya da
// sürücü vendor'a özgü bir port monitörü kullanıyor) ASLA "ölü" denmez —
// kanitVar=false döner. Sahte "USB girişi boşta" uyarısı, sessiz hatadan daha
// kötüdür: kullanıcı çalışan bir kurulumu bozmaya kalkar.
func UsbPortOlu(kuyrukPortu string, canliPortlar map[string]string) (olu bool, kanitVar bool) {
	n := normalMetin(strings.TrimSpace(kuyrukPortu))
	if !usbPortuMu(n) {
		return false, false // USB portu değil → bu kural konuşmaz
	}
	if len(canliPortlar) == 0 {
		return false, false // kanıt yok
	}
	for canli := range canliPortlar {
		if normalMetin(strings.TrimSpace(canli)) == n {
			return false, true
		}
	}
	return true, true
}

// ErrHedefBozuk — hedef metni kullanılamaz (NUL/kontrol karakteri, aşırı uzun).
var ErrHedefBozuk = errors.New("teshis: hedef metni geçersiz")

// HedefNormalize — panelden gelen hedefi temizler ve doğrular.
//
// NUL kontrolü KRİTİK: godoes/printers Open() adı UTF16'ya çevirirken hatayı
// yutuyor ve NUL içeren adda PANİKLİYOR. Panik ajanı komple öldürürdü.
//
// portsuzIP=true → hedef düz bir IP ("192.168.1.50"); fiş yazıcıları 9100
// portunda dinler, çağıran ':9100' ekleyip TEK deneme yapabilir.
func HedefNormalize(hedef string) (temiz string, portsuzIP bool, err error) {
	temiz = strings.TrimSpace(hedef)
	if temiz == "" {
		return "", false, ErrHedefBozuk
	}
	if len([]rune(temiz)) > 220 {
		return "", false, ErrHedefBozuk
	}
	for _, r := range temiz {
		if r == 0 || (r < 0x20 && r != 0x20) || r == 0x7f {
			return "", false, ErrHedefBozuk
		}
	}
	if net.ParseIP(temiz) != nil {
		return temiz, true, nil
	}
	return temiz, false, nil
}

// AdEsitMi — iki yazıcı/kuyruk adı AYNI hedefi mi gösteriyor?
//
// Windows'un OpenPrinter'ı harf duyarsızdır ve baştaki/sondaki boşluğu
// önemsemez; Go map'i ise duyarlıdır. Panelde "POS-80" yazıp kuyruk adı
// "Pos-80" ya da "POS-80 " olduğunda düz map araması TUTMAZ ve eskiden
// çalışan kurulum "bu bilgisayarda böyle bir yazıcı yok" diye fişsiz kalırdı.
// Karşılaştırma Türkçe duyarlı küçültmeyle yapılır (I/ı/İ/i tuzağı).
func AdEsitMi(a, b string) bool {
	return normalMetin(strings.TrimSpace(a)) == normalMetin(strings.TrimSpace(b))
}

// normalMetin — metnin tamamını Türkçe duyarlı küçültür (bkz. normalRune).
func normalMetin(s string) string {
	r := []rune(s)
	for i := range r {
		r[i] = normalRune(r[i])
	}
	return string(r)
}
