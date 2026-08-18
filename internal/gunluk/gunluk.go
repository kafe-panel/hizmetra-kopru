// Package gunluk — dosyaya dönen basit günlük.
//
// GİZLİLİK KURALI: fiş İÇERİĞİ (icerik_b64 / ESC/POS baytları) ASLA yazılmaz.
// Fişte müşteri adı, adres, telefon olabilir (KVKK). Yalnız is_id, hedef ve
// bayt SAYISI loglanır. Token da loglanmaz.
package gunluk

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maksBoyut = 2 << 20 // 2MB → devret
const halkaBoyut = 200    // durum penceresi fiş şeridi için bellekte tutulan son satır sayısı

var (
	kilit   sync.Mutex
	kayitci *log.Logger
	dosya   *os.File
	yolu    string
)

// Durum penceresinin "fiş şeridi" son N satırı bellekte okur (dosyayı açmadan).
// Ayrı kilit: Yaz'ın dosya kilidini bloklamaz. PII yok — fiş İÇERİĞİ zaten
// hiçbir zaman loglanmaz (yalnız is_id/hedef/bayt sayısı), o kural burada da geçerli.
var (
	halkaKilit sync.Mutex
	halka      [halkaBoyut]string
	halkaBas   int // bir sonraki yazılacak indeks
	halkaDolu  int // yazılmış satır sayısı (≤ halkaBoyut)
)

func halkayaYaz(satir string) {
	halkaKilit.Lock()
	halka[halkaBas] = satir
	halkaBas = (halkaBas + 1) % halkaBoyut
	if halkaDolu < halkaBoyut {
		halkaDolu++
	}
	halkaKilit.Unlock()
}

// SonSatirlar — bellekteki son n günlük satırını ESKİDEN YENİYE döndürür
// (fiş şeridi altta en yeni satırı gösterir). n dolu satırdan büyükse kırpılır.
func SonSatirlar(n int) []string {
	halkaKilit.Lock()
	defer halkaKilit.Unlock()
	if n > halkaDolu {
		n = halkaDolu
	}
	if n <= 0 {
		return nil
	}
	bas := (halkaBas - n + halkaBoyut) % halkaBoyut
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = halka[(bas+i)%halkaBoyut]
	}
	return out
}

// Baslat — günlüğü dizinde açar. Hata olursa yalnız stderr'e yazılır.
func Baslat(dizin string) {
	kilit.Lock()
	defer kilit.Unlock()
	yolu = filepath.Join(dizin, "kopru.log")
	ac()
}

func ac() {
	f, err := os.OpenFile(yolu, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		kayitci = log.New(os.Stderr, "", log.LstdFlags)
		return
	}
	dosya = f
	kayitci = log.New(io.MultiWriter(f, os.Stderr), "", log.LstdFlags)
}

// devret — dosya büyüdüyse .1'e taşı (tek yedek yeter).
func devret() {
	if dosya == nil {
		return
	}
	bilgi, err := dosya.Stat()
	if err != nil || bilgi.Size() < maksBoyut {
		return
	}
	dosya.Close()
	_ = os.Rename(yolu, yolu+".1")
	ac()
}

// Yaz — biçimli günlük satırı. Dosyaya (tam damga) ve bellek halkasına
// (durum penceresi için HH:MM:SS damgalı) yazar.
func Yaz(bicim string, arg ...any) {
	mesaj := fmt.Sprintf(bicim, arg...)
	kilit.Lock()
	if kayitci == nil {
		kayitci = log.New(os.Stderr, "", log.LstdFlags)
	}
	devret()
	kayitci.Output(2, mesaj) //nolint:errcheck
	kilit.Unlock()
	halkayaYaz(time.Now().Format("15:04:05") + "  " + mesaj)
}

// YazSessiz — YALNIZ dosyaya yazar, durum penceresinin fiş günlüğü halkasına
// YAZMAZ. Kullanıcıyı ilgilendirmeyen ama teşhis için tutulması iyi olan gürültü
// içindir (ör. long-poll sırasında Cloudflare/Render'ın ara ara döndürdüğü geçici
// 502/503/504 — otomatik yeniden deneniyor, fiş yine basılıyor). Böylece kafe
// sahibi fiş günlüğünde "hata" gibi görünen satırlarla korkutulmaz; durum kartı
// gerçek bağlantı durumunu zaten doğru gösterir (emre 2026-08-18).
func YazSessiz(bicim string, arg ...any) {
	mesaj := fmt.Sprintf(bicim, arg...)
	kilit.Lock()
	if kayitci == nil {
		kayitci = log.New(os.Stderr, "", log.LstdFlags)
	}
	devret()
	kayitci.Output(2, mesaj) //nolint:errcheck
	kilit.Unlock()
	// halkayaYaz YOK — bilerek: bu satır fiş günlüğü feed'ine girmez.
}

// Yolu — günlük dosyasının tam yolu (tray "Günlüğü Aç" için).
func Yolu() string { return yolu }

// Kapat — dosyayı kapatır.
func Kapat() {
	kilit.Lock()
	defer kilit.Unlock()
	if dosya != nil {
		_ = dosya.Close()
		dosya = nil
	}
}

// Zaman — insan-okunur damga (rapor satırlarında).
func Zaman() string { return time.Now().Format("15:04:05") }
