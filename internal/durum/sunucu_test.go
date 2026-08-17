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
	d := Yeni("Test Kafe", "0.2.0", func() Ozet { return Ozet{Bagli: true} }, func(int) []string { return nil }, nil, nil)
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
	}, func(int) []string { return []string{"15:04:05  iş #7 → POS-80 (12 bayt, tip=kasa)"} }, nil, nil)
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
	d := Yeni("Test Kafe", "0.2.0", func() Ozet { return Ozet{Bagli: true, Sunucu: "https://api.hizmetra.com"} }, func(int) []string { return nil }, nil, nil)
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
		func() { cagrildi = true }, nil)
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
		func() { cagrildi <- struct{}{} }, nil)
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
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
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
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
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

// TestGuncelleTokensizReddedilirVeCallbackTetiklenmez — /guncelle token ister;
// tokensiz istekte onGuncelle ÇAĞRILMAZ (installer'ı başkası tetikleyemesin).
func TestGuncelleTokensizReddedilirVeCallbackTetiklenmez(t *testing.T) {
	cagrildi := false
	d := Yeni("Test Kafe", "0.5.0", func() Ozet { return Ozet{} }, func(int) []string { return nil },
		nil, func() { cagrildi = true })
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/guncelle", "", nil) // token yok
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("tokensiz /guncelle 403 beklenir, geldi %d", r.StatusCode)
	}
	if cagrildi {
		t.Fatal("tokensiz istekte güncelle callback'i TETİKLENMEMELİ")
	}
}

// TestGuncelleTokenliCallbackTetikler — doğru token'lı POST /guncelle, enjekte
// edilen callback'i (main.go: guncelle() — indir+kur) tetikler ve 200 döner.
// /odaklan gibi callback ASENKRON tetiklenir (indirme uzun sürebilir; 200 hemen
// dönmeli) — bu yüzden kanalla POLLANIR.
func TestGuncelleTokenliCallbackTetikler(t *testing.T) {
	cagrildi := make(chan struct{}, 1)
	d := Yeni("Test Kafe", "0.5.0", func() Ozet { return Ozet{} }, func(int) []string { return nil },
		nil, func() { cagrildi <- struct{}{} })
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/guncelle?t="+d.Token, "", nil)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("tokenli /guncelle 200 beklenir, geldi %d", r.StatusCode)
	}
	select {
	case <-cagrildi:
	case <-time.After(2 * time.Second):
		t.Fatal("güncelle callback'i zamanında (asenkron) tetiklenmedi")
	}
}

// TestGuncelleYalnizPostKabulEder — doğru token ama GET → 405; onGuncelle nil
// verilse de (test) panic olmadan 405/200 döner.
func TestGuncelleYalnizPostKabulEder(t *testing.T) {
	d := Yeni("Test Kafe", "0.5.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/guncelle?t=" + d.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /guncelle 405 beklenir, geldi %d", r.StatusCode)
	}
	// onGuncelle nil iken tokenli POST yine 200 dönmeli (panic yok).
	r2, err := http.Post(srv.URL+"/guncelle?t="+d.Token, "", nil)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("nil callback ile tokenli POST /guncelle 200 beklenir, geldi %d", r2.StatusCode)
	}
}

// TestGuncelSurumVeriJSONdaGorunur — Ozet.GuncelSurum/IndirmeURL alanları
// /veri.json'da guncel_surum/indirme_url olarak görünür (sayfa "Güncelle"
// şeridini bu alanlardan gösterir).
func TestGuncelSurumVeriJSONdaGorunur(t *testing.T) {
	d := Yeni("Test Kafe", "0.5.0", func() Ozet {
		return Ozet{Bagli: true, Surum: "0.5.0", GuncelSurum: "0.6.0", IndirmeURL: "https://x/HizmetraYaziciKurulum.exe"}
	}, func(int) []string { return nil }, nil, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/veri.json?t=" + d.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	defer r.Body.Close()
	var veri struct {
		Ozet Ozet `json:"ozet"`
	}
	if err := json.NewDecoder(r.Body).Decode(&veri); err != nil {
		t.Fatalf("JSON çözülemedi: %v", err)
	}
	if veri.Ozet.GuncelSurum != "0.6.0" {
		t.Errorf("guncel_surum JSON'da yok/yanlış: %q", veri.Ozet.GuncelSurum)
	}
	if veri.Ozet.IndirmeURL != "https://x/HizmetraYaziciKurulum.exe" {
		t.Errorf("indirme_url JSON'da yok/yanlış: %q", veri.Ozet.IndirmeURL)
	}
}

// TestBaslatPortDondurur — Baslat artık dinlediği portu da döndürür (v0.4.0:
// ikinci kopyanın /odaklan'a ulaşabilmesi için bu port ayar dosyasına yazılır).
func TestBaslatPortDondurur(t *testing.T) {
	d := Yeni("Test Kafe", "0.4.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
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
