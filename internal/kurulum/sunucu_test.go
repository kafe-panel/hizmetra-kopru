package kurulum

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

// TestKurulumSayfasiTokenIster — internal/durum'daki TestDurumSayfasiTokenIster
// ile AYNI desen: "/" token ister.
func TestKurulumSayfasiTokenIster(t *testing.T) {
	s := Yeni("0.4.0", "", func(string) (*api.EslestirCevap, string, error) {
		return nil, "", api.ErrKodGecersiz
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, _ := http.Get(srv.URL + "/") // token yok
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("tokensiz 403 beklenir, geldi %d", r.StatusCode)
	}
	r2, _ := http.Get(srv.URL + "/?t=" + s.Token)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("tokenli 200 beklenir, geldi %d", r2.StatusCode)
	}
}

// TestKurulumSayfasiKodOneriGosterir — panodan önerilen kod (main.go:
// panodanKodOner) sayfaya önceden dolduruluyor.
func TestKurulumSayfasiKodOneriGosterir(t *testing.T) {
	s := Yeni("0.4.0", "482913", func(string) (*api.EslestirCevap, string, error) {
		return nil, "", api.ErrKodGecersiz
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/?t=" + s.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	govde, _ := io.ReadAll(r.Body)
	metin := string(govde)
	for _, beklenen := range []string{"482913", "Hoş geldiniz", "0.4.0"} {
		if !strings.Contains(metin, beklenen) {
			t.Errorf("sayfada %q yok", beklenen)
		}
	}
}

// TestEslestirDeneTokenGerektirmez — /eslestir-dene KASITLI OLARAK token
// istemez (henüz eşleşme yok); başarılı denemede Sonuc() kanalına da düşer.
func TestEslestirDeneTokenGerektirmez(t *testing.T) {
	cevapDon := &api.EslestirCevap{Token: "sunucu-tokeni", CihazID: 5, IsletmeAd: "Test Kafe"}
	s := Yeni("0.4.0", "", func(kod string) (*api.EslestirCevap, string, error) {
		if kod != "123456" {
			t.Fatalf("beklenmeyen kod: %q", kod)
		}
		return cevapDon, "https://api.hizmetra.com", nil
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/eslestir-dene", "text/plain", strings.NewReader("123456")) // token YOK
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("200 beklenir, geldi %d", r.StatusCode)
	}
	var yanit eslestirYanit
	if err := json.NewDecoder(r.Body).Decode(&yanit); err != nil {
		t.Fatalf("JSON çözülemedi: %v", err)
	}
	if !yanit.Basarili || yanit.IsletmeAd != "Test Kafe" || yanit.Hata != "" {
		t.Errorf("beklenmeyen yanıt: %+v", yanit)
	}

	select {
	case es := <-s.Sonuc():
		if es.Cevap.IsletmeAd != "Test Kafe" || es.Sunucu != "https://api.hizmetra.com" {
			t.Errorf("beklenmeyen sonuç: %+v", es)
		}
	default:
		t.Error("Sonuc() kanalına başarı bildirilmedi")
	}
}

// TestEslestirDeneGecersizKodHataDoner — sunucu ErrKodGecersiz döndürünce
// sayfa içi Türkçe hata döner ve Sonuc() DOLMAZ (main.go beklemeye devam eder).
func TestEslestirDeneGecersizKodHataDoner(t *testing.T) {
	s := Yeni("0.4.0", "", func(string) (*api.EslestirCevap, string, error) {
		return nil, "", api.ErrKodGecersiz
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/eslestir-dene", "text/plain", strings.NewReader("000000"))
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	var yanit eslestirYanit
	if err := json.NewDecoder(r.Body).Decode(&yanit); err != nil {
		t.Fatalf("JSON çözülemedi: %v", err)
	}
	if yanit.Basarili || yanit.Hata == "" {
		t.Errorf("hata bekleniyordu: %+v", yanit)
	}
	select {
	case es := <-s.Sonuc():
		t.Errorf("başarısız denemede Sonuc() dolmamalı: %+v", es)
	default:
	}
}

// TestEslestirDeneBaglantiHatasiFarkliMesaj — ErrKodGecersiz DIŞINDAKİ
// hatalarda (ör. ağ hatası) "sunucuya ulaşılamadı" mesajı dönmeli (main.go'nun
// eski zenity dallarıyla AYNI ayrım: kod-geçersiz vs. ulaşılamama).
func TestEslestirDeneBaglantiHatasiFarkliMesaj(t *testing.T) {
	s := Yeni("0.4.0", "", func(string) (*api.EslestirCevap, string, error) {
		return nil, "", errAglantiYok
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/eslestir-dene", "text/plain", strings.NewReader("111111"))
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	var yanit eslestirYanit
	_ = json.NewDecoder(r.Body).Decode(&yanit)
	if yanit.Basarili || !strings.Contains(yanit.Hata, "ulaşılamadı") {
		t.Errorf("bağlantı hatası mesajı bekleniyordu: %+v", yanit)
	}
}

// TestEslestirDeneKisaKodSunucuyaGitmez — 6 haneden kısa/uzun kod esle()
// fonksiyonuna HİÇ gitmez (main.go'nun eski "Kod 6 haneli olmalı" kontrolü
// artık sunucu tarafında da var — savunma katmanı, JS tarafı zaten engeller).
func TestEslestirDeneKisaKodSunucuyaGitmez(t *testing.T) {
	cagrildi := false
	s := Yeni("0.4.0", "", func(string) (*api.EslestirCevap, string, error) {
		cagrildi = true
		return nil, "", api.ErrKodGecersiz
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/eslestir-dene", "text/plain", strings.NewReader("123"))
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	var yanit eslestirYanit
	_ = json.NewDecoder(r.Body).Decode(&yanit)
	if cagrildi {
		t.Error("6 haneden kısa kod esle() fonksiyonuna gitmemeli")
	}
	if yanit.Hata == "" {
		t.Error("kısa kod için hata mesajı bekleniyordu")
	}
}

// TestEslestirDeneSadecePost — GET (ya da başka metot) 405 döner.
func TestEslestirDeneSadecePost(t *testing.T) {
	s := Yeni("0.4.0", "", func(string) (*api.EslestirCevap, string, error) {
		return nil, "", api.ErrKodGecersiz
	})
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()

	r, _ := http.Get(srv.URL + "/eslestir-dene")
	if r.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET'te 405 beklenir, geldi %d", r.StatusCode)
	}
}

// errAglantiYok — testte "ağa ulaşılamadı" gibi ErrKodGecersiz DIŞI bir hata
// simüle eder (gerçek koddaki http/dns hatalarının yerine geçer).
type aglantiHatasi struct{}

func (aglantiHatasi) Error() string { return "bağlantı zaman aşımı" }

var errAglantiYok error = aglantiHatasi{}
