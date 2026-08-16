package durum

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDurumSayfasiTokenIster(t *testing.T) {
	d := Yeni("Test Kafe", "0.2.0", func() Ozet { return Ozet{Bagli: true} }, func(int) []string { return nil }, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()
	r, _ := http.Get(srv.URL + "/") // token yok
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("tokensiz 403 beklenir, geldi %d", r.StatusCode)
	}
	r2, _ := http.Get(srv.URL + "/?t=" + d.Token)
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("tokenli 200 beklenir, geldi %d", r2.StatusCode)
	}
}

// TestSayfaIsletmeAdiVeYaziciGosterir — tokenli sayfa özet alanlarını render eder.
// Üst şeritteki uygulama adı GÖRÜNEN ad "Hizmetra Yazıcı"dır (v0.3.0 rename;
// teknik kimlik hizmetra-kopru/HizmetraKopru değişmez).
func TestSayfaIsletmeAdiVeYaziciGosterir(t *testing.T) {
	d := Yeni("Hizmetra Yazıcı", "0.3.0", func() Ozet {
		return Ozet{Bagli: true, IsletmeAd: "Çokluşubetemiz", Yazicilar: []string{"POS-80"}}
	}, func(int) []string { return []string{"15:04:05  iş #7 → POS-80 (12 bayt, tip=kasa)"} }, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/?t=" + d.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	govde, _ := io.ReadAll(r.Body)
	metin := string(govde)
	for _, beklenen := range []string{"Hizmetra Yazıcı", "Çokluşubetemiz", "POS-80", "iş #7"} {
		if !strings.Contains(metin, beklenen) {
			t.Errorf("sayfada %q yok", beklenen)
		}
	}
	if strings.Contains(metin, "Hizmetra Köprü") {
		t.Error("eski görünen ad 'Hizmetra Köprü' sayfada kalmamalı")
	}
}

// TestVeriJSONTokensizReddedilir — /veri.json de token ister ve JSON döner.
func TestVeriJSONTokensizReddedilir(t *testing.T) {
	d := Yeni("Test Kafe", "0.2.0", func() Ozet { return Ozet{Bagli: true, Sunucu: "https://api.hizmetra.com"} }, func(int) []string { return nil }, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, _ := http.Get(srv.URL + "/veri.json") // token yok
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("tokensiz /veri.json 403 beklenir, geldi %d", r.StatusCode)
	}

	r2, err := http.Get(srv.URL + "/veri.json?t=" + d.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("tokenli /veri.json 200 beklenir, geldi %d", r2.StatusCode)
	}
	var veri struct {
		Ozet     Ozet     `json:"ozet"`
		Gunluk   []string `json:"gunluk"`
		PanelURL string   `json:"panel_url"`
	}
	if err := json.NewDecoder(r2.Body).Decode(&veri); err != nil {
		t.Fatalf("JSON çözülemedi: %v", err)
	}
	if !veri.Ozet.Bagli {
		t.Error("ozet.bagli true beklenir")
	}
	// panel linki api. → panel. türetimi
	if veri.PanelURL != "https://panel.hizmetra.com" {
		t.Errorf("panel_url türetimi yanlış: %q", veri.PanelURL)
	}
}

func TestPanelURLTuret(t *testing.T) {
	if g := panelURLTuret("https://api.hizmetra.com"); g != "https://panel.hizmetra.com" {
		t.Errorf("api→panel türetimi: %q", g)
	}
	if g := panelURLTuret(""); g != "" {
		t.Errorf("boş girdi boş dönmeli: %q", g)
	}
}

// TestOdaklanTokensizReddedilirVeCallbackTetiklenmez — /odaklan de token
// ister; tokensiz istekte onOdaklan ÇAĞRILMAZ (v0.4.0, madde: ikinci kopya
// yalnız kayıtlı token'ı biliyorsa çalışan kopyayı öne getirebilmeli).
func TestOdaklanTokensizReddedilirVeCallbackTetiklenmez(t *testing.T) {
	cagrildi := false
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil },
		func() { cagrildi = true })
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/odaklan", "", nil) // token yok
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("tokensiz /odaklan 403 beklenir, geldi %d", r.StatusCode)
	}
	if cagrildi {
		t.Fatal("tokensiz istekte callback TETİKLENMEMELİ")
	}
}

// TestOdaklanTokenliCallbackTetikler — doğru token'lı POST /odaklan, enjekte
// edilen callback'i (main.go'da pencere.OneGetir()) tetikler ve 200 döner.
// Callback ASENKRON tetiklenir (gerçek Windows'ta ölçüldü: main.go'daki
// callback soğuk WebView2 açılışında saniyelerce sürebilir — 200 yanıtı bunu
// BEKLEMEMELİ, bkz. Handler yorumu) — bu yüzden test kanal + kısa bir zaman
// aşımıyla POLLAR, hemen ardından senkron kontrol ETMEZ.
func TestOdaklanTokenliCallbackTetikler(t *testing.T) {
	cagrildi := make(chan struct{}, 1)
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil },
		func() { cagrildi <- struct{}{} })
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/odaklan?t="+d.Token, "", nil)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("tokenli /odaklan 200 beklenir, geldi %d", r.StatusCode)
	}
	select {
	case <-cagrildi:
	case <-time.After(2 * time.Second):
		t.Fatal("callback zamanında (asenkron) tetiklenmedi")
	}
}

// TestOdaklanCallbackNilOlabilir — onOdaklan nil verilirse (main.go dışı
// kullanım/test) /odaklan yine de 200 döner, panic OLMAZ.
func TestOdaklanCallbackNilOlabilir(t *testing.T) {
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/odaklan?t="+d.Token, "", nil)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("200 beklenir, geldi %d", r.StatusCode)
	}
}

// TestOdaklanYalnizPostKabulEder — doğru token ama GET → 405 (metod yasağı
// token kontrolünden SONRA gelir; tokensiz istek her zaman 403 kalır).
func TestOdaklanYalnizPostKabulEder(t *testing.T) {
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/odaklan?t=" + d.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /odaklan 405 beklenir, geldi %d", r.StatusCode)
	}
}

// TestBaslatPortDondurur — Baslat artık dinlediği portu da döndürür (v0.4.0:
// ikinci kopyanın /odaklan'a ulaşabilmesi için bu port ayar dosyasına yazılır).
func TestBaslatPortDondurur(t *testing.T) {
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil)
	url, port, err := d.Baslat()
	if err != nil {
		t.Fatalf("Baslat hata verdi: %v", err)
	}
	if port <= 0 {
		t.Fatalf("port pozitif olmalı, geldi %d", port)
	}
	beklenenParca := fmt.Sprintf(":%d/?t=", port)
	if !strings.Contains(url, beklenenParca) {
		t.Fatalf("dönen URL %q port %d ile tutarsız", url, port)
	}
}
