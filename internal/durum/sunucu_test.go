package durum

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDurumSayfasiTokenIster(t *testing.T) {
	d := Yeni("Test Kafe", "0.2.0", func() Ozet { return Ozet{Bagli: true} }, func(int) []string { return nil })
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
	}, func(int) []string { return []string{"15:04:05  iş #7 → POS-80 (12 bayt, tip=kasa)"} })
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
	d := Yeni("Test Kafe", "0.2.0", func() Ozet { return Ozet{Bagli: true, Sunucu: "https://api.hizmetra.com"} }, func(int) []string { return nil })
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
