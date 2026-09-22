package kopru

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

// sonucToplayici — gönderilen sonuçları biriktiren mini sunucu.
func sonucToplayici(t *testing.T, kilit *sync.Mutex, kutu *[]api.Sonuc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/kopru/isler/sonuc" {
			var govde struct {
				Sonuclar []api.Sonuc `json:"sonuclar"`
			}
			_ = json.NewDecoder(r.Body).Decode(&govde)
			kilit.Lock()
			*kutu = append(*kutu, govde.Sonuclar...)
			kilit.Unlock()
		}
		zarfYaz(w, 200, true, map[string]any{"islenen": 1})
	}))
}

func testIs(id int64) api.Is {
	return api.Is{IsID: id, Hedef: "POS-80", Tip: "kasa",
		IcerikB64: base64.StdEncoding.EncodeToString([]byte("FIS"))}
}

// TestAsiliBaskiZamanAsiminaDuser — 60 saniye uyuyan bir yazıcı yolu işi
// KİLİTLEMEMELİ: 30 saniyede (testte kısaltılmış) kesilir.
//
// SÖZLEŞME DEĞİŞTİ (2026-09-22): asılı iş artık 'hata' DEĞİL 'basildi'
// bildirilir. Sebep: zaman aşımına uğrayan baskı goroutine'i iptal edilemiyor,
// arkada spooler'a yazmaya devam ediyor; kağıt takılınca o fiş basıyor.
// 'hata' deseydik sunucu aynı işi 2 dakika sonra yeniden verir ve mutfağa
// İKİNCİ fiş düşerdi. İş şüpheli deftere yazılır ve bir daha ASLA basılmaz.
func TestAsiliBaskiZamanAsiminaDuser(t *testing.T) {
	var kilit sync.Mutex
	var sonuclar []api.Sonuc
	srv := sonucToplayici(t, &kilit, &sonuclar)
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	a := Yeni(istemci, func(string, []byte) error {
		time.Sleep(60 * time.Second) // asılı yazıcı
		return nil
	}, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})

	eski := basZamanAsimiTest
	basZamanAsimiTest = 300 * time.Millisecond
	defer func() { basZamanAsimiTest = eski }()

	basla := time.Now()
	a.isleriBas([]api.Is{testIs(21)})
	if gecen := time.Since(basla); gecen > 10*time.Second {
		t.Fatalf("iş döngüsü kilitlendi (%v)", gecen)
	}

	kilit.Lock()
	defer kilit.Unlock()
	if len(sonuclar) != 1 {
		t.Fatalf("1 sonuç bekleniyordu, %d geldi", len(sonuclar))
	}
	if sonuclar[0].Durum != "basildi" {
		t.Fatalf("asılı baskı yeniden verilmemeli ('basildi' beklenir), %q geldi", sonuclar[0].Durum)
	}
	if a.zatenBasildi(21) {
		t.Error("zaman aşımına uğrayan iş 'basıldı' işaretlenmemeli (kör yeniden deneme yok)")
	}
	if !a.supheliMi(21) {
		t.Error("zaman aşımına uğrayan iş ŞÜPHELİ deftere yazılmalı")
	}
	// Sunucu aynı işi yine de verirse (sonuç POST'u kaybolmuş olabilir) iş
	// TEKRAR BASILMAMALI.
	basildi := 0
	a.Bas = func(string, []byte) error { basildi++; return nil }
	a.isleriBas([]api.Is{testIs(21)})
	if basildi != 0 {
		t.Errorf("şüpheli iş yeniden verilince basılmamalıydı (%d kez basıldı)", basildi)
	}
	if d := a.Durum.Oku(); d.SonBaskiSorunu == "" {
		t.Error("kullanıcı uyarılmalı: durum kartında baskı sorunu cümlesi olmalı")
	}
}

// TestPanikDonguyuOldurmez — panik atan bir Basici ajanı öldürmemeli.
func TestPanikDonguyuOldurmez(t *testing.T) {
	var kilit sync.Mutex
	var sonuclar []api.Sonuc
	srv := sonucToplayici(t, &kilit, &sonuclar)
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	a := Yeni(istemci, func(string, []byte) error {
		panic("printers.Open paniği")
	}, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})

	a.isleriBas([]api.Is{testIs(31)}) // panik yayılırsa test çöker

	kilit.Lock()
	defer kilit.Unlock()
	if len(sonuclar) != 1 || sonuclar[0].Durum != "hata" {
		t.Fatalf("panikte 'hata' bildirilmeliydi: %+v", sonuclar)
	}
}

// TestNabizPanigiDonguyuOldurmez — keşif panik atarsa nabız yutmalı.
func TestNabizPanigiDonguyuOldurmez(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 200, true, map[string]any{"ok": true, "poll_sn": 30})
	}))
	defer srv.Close()
	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	a := Yeni(istemci, func(string, []byte) error { return nil },
		func() ([]api.Yazici, error) { panic("keşif paniği") }, "test", &Durum{})
	a.nabizAt() // panik yayılırsa test çöker
}

// TestBasariliNabizBaskiSorununuSILMEZ — bağlantının sağlam olması fişin
// çıktığı anlamına gelmez. Eski kod nabızda SonHata'yı siliyordu ve kullanıcı
// "✓ Bağlı" görüp çıkmayan fişi fark etmiyordu.
func TestBasariliNabizBaskiSorununuSilmez(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/kopru/isler":
			zarfYaz(w, 200, true, []map[string]any{})
		default:
			zarfYaz(w, 200, true, map[string]any{"ok": true, "poll_sn": 30})
		}
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	d := &Durum{}
	a := Yeni(istemci, func(string, []byte) error { return nil },
		func() ([]api.Yazici, error) { return nil, nil }, "test", d)

	a.baskiSorunuYaz("Kağıt bitti.", "KAGIT_YOK", "Mutfak")

	a.nabizAt()
	if d.Oku().SonBaskiSorunu == "" {
		t.Fatal("başarılı nabız baskı sorununu SİLMEMELİ")
	}
	// Başarılı iş çekme de silmemeli.
	if _, err := istemci.Isler(0); err != nil {
		t.Fatal(err)
	}
	d.Ayarla(func(x *Durum) { x.Bagli = true; x.SonHata = "" })
	if d.Oku().SonBaskiSorunu == "" {
		t.Fatal("başarılı iş çekme baskı sorununu SİLMEMELİ")
	}
	if d.Oku().SonBaskiKodu != "KAGIT_YOK" {
		t.Fatalf("kod kayboldu: %q", d.Oku().SonBaskiKodu)
	}
}

// TestHataSonucundaHataMetniBosOlmaz — panelde boş "Son Hata" görünmemeli.
func TestHataSonucundaHataMetniBosOlmaz(t *testing.T) {
	a := Yeni(nil, nil, nil, "test", &Durum{})
	s := a.hataSonucu(1, "   ", "")
	if strings.TrimSpace(s.Hata) == "" {
		t.Fatal("durum=='hata' iken Hata alanı ASLA boş kalmamalı")
	}
	if s.Durum != "hata" {
		t.Fatalf("durum yanlış: %q", s.Durum)
	}
}

// TestPartiButcesiKalanIsleriHataBildirir — bütçe dolunca kalan işler sessizce
// düşürülmez, 'hata' bildirilir ki sunucu yeniden versin.
func TestPartiButcesiKalanIsleriHataBildirir(t *testing.T) {
	var kilit sync.Mutex
	var sonuclar []api.Sonuc
	srv := sonucToplayici(t, &kilit, &sonuclar)
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	a := Yeni(istemci, func(string, []byte) error {
		time.Sleep(120 * time.Millisecond)
		return nil
	}, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})

	eskiButce := partiButcesiTest
	partiButcesiTest = 50 * time.Millisecond
	defer func() { partiButcesiTest = eskiButce }()

	a.isleriBas([]api.Is{testIs(41), testIs(42), testIs(43)})

	kilit.Lock()
	defer kilit.Unlock()
	if len(sonuclar) != 3 {
		t.Fatalf("3 sonuç bekleniyordu (hiçbiri düşürülmemeli), %d geldi", len(sonuclar))
	}
	hataSayisi := 0
	for _, s := range sonuclar {
		if s.Durum == "hata" {
			hataSayisi++
			if strings.TrimSpace(s.Hata) == "" {
				t.Error("bütçe aşımı sonucunda hata metni boş")
			}
		}
	}
	if hataSayisi == 0 {
		t.Fatal("bütçe aşımında kalan işler 'hata' bildirilmeliydi")
	}
}
