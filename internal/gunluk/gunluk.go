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

var (
	kilit  sync.Mutex
	kayitci *log.Logger
	dosya  *os.File
	yolu   string
)

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

// Yaz — biçimli günlük satırı.
func Yaz(bicim string, arg ...any) {
	kilit.Lock()
	defer kilit.Unlock()
	if kayitci == nil {
		kayitci = log.New(os.Stderr, "", log.LstdFlags)
	}
	devret()
	kayitci.Output(2, fmt.Sprintf(bicim, arg...)) //nolint:errcheck
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
