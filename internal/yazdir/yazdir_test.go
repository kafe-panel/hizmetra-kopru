package yazdir

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

func TestAgHedefiMi(t *testing.T) {
	durumlar := []struct {
		hedef  string
		bekler bool
	}{
		{"192.168.1.50:9100", true},
		{"printer.local:9100", true},
		{"10.0.0.5:515", true},
		{"POS-80", false}, // Windows yazıcı adı — tire var ama port yok
		{"Kasa:2", false}, // iki nokta var ama yazıcı adı — ağ adresi DEĞİL
		{"EPSON TM-T20 Receipt", false},
		{"", false},
		{"192.168.1.50:", false},    // port boş
		{"192.168.1.50:abc", false}, // port sayısal değil
		{"192.168.1.50:99999", false},
	}
	for _, d := range durumlar {
		if got := AgHedefiMi(d.hedef); got != d.bekler {
			t.Errorf("AgHedefiMi(%q) = %v, beklenen %v", d.hedef, got, d.bekler)
		}
	}
}

func TestBasBosHedef(t *testing.T) {
	if err := Bas("   ", []byte("x")); !errors.Is(err, ErrBosHedef) {
		t.Fatalf("ErrBosHedef bekleniyordu, gelen: %v", err)
	}
}

func TestBasAgHedefineBaytlariAynenYazar(t *testing.T) {
	dinleyici, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dinleyici kurulamadı: %v", err)
	}
	defer dinleyici.Close()

	alinan := make(chan []byte, 1)
	go func() {
		baglanti, err := dinleyici.Accept()
		if err != nil {
			alinan <- nil
			return
		}
		defer baglanti.Close()
		_ = baglanti.SetReadDeadline(time.Now().Add(3 * time.Second))
		veri, _ := io.ReadAll(baglanti)
		alinan <- veri
	}()

	// Gerçek ESC/POS öneki: ESC @ (init) + metin + GS V (kes)
	fis := []byte{0x1B, 0x40, 'T', 'e', 's', 't', 0x0A, 0x1D, 0x56, 0x00}
	// Baskıdan ÖNCE cihaza durum sorulur (DLE EOT n=4). Bu sunucu cevap
	// vermediği için sorgu bir kez gider, sonra baskı NORMAL tamamlanır —
	// "cevap yok" tek başına hata DEĞİLDİR.
	beklenen := append(DleEotSorgu(DleEotKagitSensoru), fis...)
	if err := Bas(dinleyici.Addr().String(), fis); err != nil {
		t.Fatalf("Bas hata verdi: %v", err)
	}
	select {
	case veri := <-alinan:
		if !bytes.Equal(veri, beklenen) {
			t.Errorf("baytlar bozuldu:\ngelen  = % x\nbeklenen = % x", veri, beklenen)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sunucu baytları zamanında almadı")
	}
}

func TestBasKapaliAgHedefiHataDoner(t *testing.T) {
	// Kapalı port — yazıcı kapalı senaryosu. Hata DÖNMELİ ki iş 'hata'
	// olarak bildirilsin (sessizce yutulmamalı).
	dinleyici, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dinleyici kurulamadı: %v", err)
	}
	adres := dinleyici.Addr().String()
	dinleyici.Close() // hemen kapat → bağlantı reddedilecek

	err = Bas(adres, []byte("x"))
	if err == nil {
		t.Fatal("kapalı hedefte hata bekleniyordu, nil geldi")
	}
	// Reddedilen bağlantı = adres canlı ama fiş girişi kapalı (başka cihaz).
	// Yeniden denemek anlamsızdır: deneme sayacı 1'de kalmalı.
	if kod := teshis.KodunuAl(err); kod != teshis.AG_YANLIS_CIHAZ {
		t.Errorf("AG_YANLIS_CIHAZ bekleniyordu, %q geldi (%v)", kod, err)
	}
	if n := AgDenemeSayisi(); n != 1 {
		t.Errorf("reddedilen bağlantıda yeniden deneme YAPILMAMALI, deneme=%d", n)
	}
}

// TestAgKagitYokkenFisGONDERILMEZ — yazıcı "kağıt bitti" derse fiş baytları
// HİÇ gönderilmemeli. Yoksa yazıcı fişleri tamponlar ve kullanıcı rulo takınca
// biriken fişler topluca dökülür.
func TestAgKagitYokkenFisGonderilmez(t *testing.T) {
	dinleyici, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("dinleyici kurulamadı: %v", err)
	}
	defer dinleyici.Close()

	alinan := make(chan []byte, 1)
	go func() {
		baglanti, err := dinleyici.Accept()
		if err != nil {
			alinan <- nil
			return
		}
		defer baglanti.Close()
		_ = baglanti.SetReadDeadline(time.Now().Add(3 * time.Second))
		// Sorguyu oku, "kağıt bitti" cevabı ver (taban 0x12 + bit5|bit6).
		sorgu := make([]byte, 3)
		if _, err := io.ReadFull(baglanti, sorgu); err != nil {
			alinan <- nil
			return
		}
		_, _ = baglanti.Write([]byte{0x12 | 0x60})
		kalan, _ := io.ReadAll(baglanti)
		alinan <- append(sorgu, kalan...)
	}()

	fis := []byte{0x1B, 0x40, 'K', 'A', 'G', 'I', 'T'}
	err = Bas(dinleyici.Addr().String(), fis)
	if kod := teshis.KodunuAl(err); kod != teshis.KAGIT_YOK {
		t.Fatalf("KAGIT_YOK bekleniyordu, %q geldi (%v)", kod, err)
	}
	select {
	case veri := <-alinan:
		if bytes.Contains(veri, fis) {
			t.Errorf("kağıt yokken fiş GÖNDERİLMİŞ: % x", veri)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sunucu okumayı bitirmedi")
	}
}

// TestBasBozukHedefReddedilir — NUL içeren hedef spooler'a HİÇ ulaşmamalı
// (printers.Open o adda panikliyor).
func TestBasBozukHedefReddedilir(t *testing.T) {
	eskisi := spoolerYaz
	defer func() { spoolerYaz = eskisi }()
	cagrildi := false
	spoolerYaz = func(ad string, veri []byte) error { cagrildi = true; return nil }

	err := Bas("POS-80\x00X", []byte("x"))
	if err == nil {
		t.Fatal("bozuk hedefte hata bekleniyordu")
	}
	if cagrildi {
		t.Error("bozuk hedef spooler'a iletilmemeliydi")
	}
	if kod := teshis.KodunuAl(err); kod != teshis.HEDEF_BOS {
		t.Errorf("HEDEF_BOS bekleniyordu, %q geldi", kod)
	}
}

func TestBasYereldeSpoolerCagirir(t *testing.T) {
	eskisi := spoolerYaz
	defer func() { spoolerYaz = eskisi }()

	var gorulenAd string
	var gorulenVeri []byte
	spoolerYaz = func(ad string, veri []byte) error {
		gorulenAd, gorulenVeri = ad, veri
		return nil
	}

	veri := []byte{0x1B, 0x40, 'A'}
	if err := Bas("POS-80", veri); err != nil {
		t.Fatalf("Bas hata verdi: %v", err)
	}
	if gorulenAd != "POS-80" {
		t.Errorf("yazıcı adı yanlış: %q", gorulenAd)
	}
	if !bytes.Equal(gorulenVeri, veri) {
		t.Errorf("baytlar bozuldu: % x", gorulenVeri)
	}
}

func TestBasSpoolerHatasiniIletir(t *testing.T) {
	eskisi := spoolerYaz
	defer func() { spoolerYaz = eskisi }()

	spoolerYaz = func(ad string, veri []byte) error { return errors.New("kağıt yok") }
	if err := Bas("POS-80", []byte("x")); err == nil {
		t.Fatal("spooler hatası iletilmedi")
	}
}
