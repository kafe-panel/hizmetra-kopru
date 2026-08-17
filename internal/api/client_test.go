package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// sunucuKur — api_response(...) zarfını taklit eden test sunucusu.
func sunucuKur(t *testing.T, isleyici func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(isleyici))
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

func zarfYaz(w http.ResponseWriter, kod int, basarili bool, veri any, mesaj string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(kod)
	ham, _ := json.Marshal(veri)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": basarili,
		"data":    json.RawMessage(ham),
		"message": mesaj,
	})
}

func TestEslestirTokenDonerVeSaklar(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/kopru/eslestir" || r.Method != http.MethodPost {
			t.Errorf("beklenmeyen istek: %s %s", r.Method, r.URL.Path)
		}
		var govde map[string]string
		_ = json.NewDecoder(r.Body).Decode(&govde)
		if govde["kod"] != "482917" {
			t.Errorf("kod gönderilmedi: %v", govde)
		}
		zarfYaz(w, 200, true, map[string]any{
			"token": "k1-0-abc", "cihaz_id": 7, "isletme_ad": "Test Kafe",
		}, "")
	})
	cevap, err := c.Eslestir("482917", "Kasa PC", "PC-1", "0.1.0", "windows")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if cevap.Token != "k1-0-abc" || cevap.IsletmeAd != "Test Kafe" {
		t.Errorf("cevap yanlış: %+v", cevap)
	}
	if c.Token != "k1-0-abc" {
		t.Error("token istemciye saklanmadı")
	}
}

func TestEslestirGecersizKod404(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 404, false, nil, "Kurulum kodu geçersiz")
	})
	if _, err := c.Eslestir("000000", "", "", "", ""); err != ErrKodGecersiz {
		t.Fatalf("ErrKodGecersiz bekleniyordu, gelen: %v", err)
	}
}

func TestTokenBasligiGonderilir(t *testing.T) {
	var gorulen string
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		gorulen = r.Header.Get("X-Kopru-Token")
		zarfYaz(w, 200, true, map[string]any{"ok": true, "poll_sn": 25}, "")
	})
	c.Token = "k1-0-gizli"
	if _, err := c.Nabiz(nil, "0.1.0"); err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if gorulen != "k1-0-gizli" {
		t.Errorf("X-Kopru-Token gönderilmedi: %q", gorulen)
	}
}

func TestNabizYazicilariGonderirVePollSnOkur(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		var govde struct {
			Yazicilar []Yazici `json:"yazicilar"`
			Surum     string   `json:"surum"`
		}
		_ = json.NewDecoder(r.Body).Decode(&govde)
		if len(govde.Yazicilar) != 1 || govde.Yazicilar[0].Hedef != "POS-80" {
			t.Errorf("yazıcı listesi yanlış: %+v", govde.Yazicilar)
		}
		zarfYaz(w, 200, true, map[string]any{"ok": true, "poll_sn": 5}, "")
	})
	c.Token = "k1-0-x"
	cevap, err := c.Nabiz([]Yazici{{Ad: "POS-80", Hedef: "POS-80", Tip: "windows", Durum: "online"}}, "0.1.0")
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	// Ölçek düğmesi: sunucu poll_sn'i değiştirince ajan UYMAK ZORUNDA.
	if cevap.PollSn != 5 {
		t.Errorf("poll_sn okunmadı: %d", cevap.PollSn)
	}
}

func TestIslerBekleParametresiVeListe(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("bekle") != "25" {
			t.Errorf("bekle parametresi yanlış: %q", r.URL.Query().Get("bekle"))
		}
		zarfYaz(w, 200, true, []map[string]any{
			{"is_id": 42, "hedef": "POS-80", "tip": "mutfak", "icerik_b64": "SGVsbG8="},
		}, "")
	})
	c.Token = "k1-0-x"
	isler, err := c.Isler(25)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if len(isler) != 1 || isler[0].IsID != 42 || isler[0].IcerikB64 != "SGVsbG8=" {
		t.Errorf("iş listesi yanlış: %+v", isler)
	}
}

func TestIslerBosListe(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 200, true, []any{}, "")
	})
	c.Token = "k1-0-x"
	isler, err := c.Isler(0)
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if len(isler) != 0 {
		t.Errorf("boş liste bekleniyordu: %+v", isler)
	}
}

func TestYetkisiz401(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 401, false, nil, "Yetkisiz")
	})
	c.Token = "k1-0-iptal"
	if _, err := c.Isler(0); err != ErrYetkisiz {
		t.Fatalf("ErrYetkisiz bekleniyordu, gelen: %v", err)
	}
	if _, err := c.Nabiz(nil, "0.1.0"); err != ErrYetkisiz {
		t.Fatalf("ErrYetkisiz bekleniyordu, gelen: %v", err)
	}
}

// TestGeciciAgGecidiHatasi — 502/503/504 → ErrGecici. Cloudflare/Render bu
// hatalarda çoğu zaman JSON DEĞİL HTML gövde döner; istemci gövdeyi çözmeye
// ÇALIŞMADAN (yoksa "cevap çözümlenemedi" gürültüsü) doğrudan ErrGecici döner.
func TestGeciciAgGecidiHatasi(t *testing.T) {
	for _, kod := range []int{http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(kod)
			_, _ = w.Write([]byte("<html><head><title>Bad Gateway</title></head><body>502</body></html>"))
		})
		c.Token = "k1-0-x"
		if _, err := c.Isler(0); !errors.Is(err, ErrGecici) {
			t.Errorf("HTTP %d için ErrGecici bekleniyordu, gelen: %v", kod, err)
		}
		if _, err := c.Nabiz(nil, "0.1.0"); !errors.Is(err, ErrGecici) {
			t.Errorf("nabız HTTP %d için ErrGecici bekleniyordu, gelen: %v", kod, err)
		}
	}
}

func TestSonucBildir(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		var govde struct {
			Sonuclar []Sonuc `json:"sonuclar"`
		}
		_ = json.NewDecoder(r.Body).Decode(&govde)
		if len(govde.Sonuclar) != 2 || govde.Sonuclar[0].Durum != "basildi" {
			t.Errorf("sonuç gövdesi yanlış: %+v", govde.Sonuclar)
		}
		if govde.Sonuclar[1].Hata != "kağıt yok" {
			t.Errorf("hata mesajı gitmedi: %+v", govde.Sonuclar[1])
		}
		zarfYaz(w, 200, true, map[string]any{"islenen": 2}, "")
	})
	c.Token = "k1-0-x"
	n, err := c.SonucBildir([]Sonuc{
		{IsID: 1, Durum: "basildi"},
		{IsID: 2, Durum: "hata", Hata: "kağıt yok"},
	})
	if err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if n != 2 {
		t.Errorf("işlenen sayısı yanlış: %d", n)
	}
}

func TestSonucBildirBosListeIstekAtmaz(t *testing.T) {
	cagrildi := false
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		cagrildi = true
		zarfYaz(w, 200, true, map[string]any{"islenen": 0}, "")
	})
	c.Token = "k1-0-x"
	if _, err := c.SonucBildir(nil); err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if cagrildi {
		t.Error("boş sonuç listesi için sunucuya istek atılmamalı")
	}
}

func TestKaldirBasarili(t *testing.T) {
	var gorulenYol, gorulenMetot, gorulenToken string
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		gorulenYol = r.URL.Path
		gorulenMetot = r.Method
		gorulenToken = r.Header.Get("X-Kopru-Token")
		zarfYaz(w, 200, true, nil, "Cihaz kaydı silindi")
	})
	c.Token = "k1-0-x"
	if err := c.Kaldir(); err != nil {
		t.Fatalf("beklenmeyen hata: %v", err)
	}
	if gorulenYol != "/api/kopru/kaldir" || gorulenMetot != http.MethodPost {
		t.Errorf("beklenmeyen istek: %s %s", gorulenMetot, gorulenYol)
	}
	if gorulenToken != "k1-0-x" {
		t.Errorf("X-Kopru-Token gönderilmedi: %q", gorulenToken)
	}
}

func TestKaldirYetkisiz401(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 401, false, nil, "Yetkisiz")
	})
	c.Token = "k1-0-iptal"
	if err := c.Kaldir(); err != ErrYetkisiz {
		t.Fatalf("ErrYetkisiz bekleniyordu, gelen: %v", err)
	}
}

// TestKaldir404UcYok — production'da (api.hizmetra.com) bu uç henüz yayında
// olmayabilir (main branch'e gitmedi). Canlı doğrulandı: uygulama genelindeki
// 404 handler'ı da api_response zarfı döner ({success:false,message:"Kaynak
// bulunamadı."}), düz HTML DEĞİL — bu yüzden burada da AYNI zarf taklit
// edilir. İstemci bunu ErrKodGecersiz'e çevirir (anlamca "kaldırma" ile
// örtüşmese de, o eşleme `istek()`'te uç-bağımsız) — kaldirSunucudanCihazi
// hangi sentinel döndüğüne bakmaksızın HER hatayı sessizce yutar.
func TestKaldir404UcYok(t *testing.T) {
	c := sunucuKur(t, func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 404, false, nil, "Kaynak bulunamadı.")
	})
	c.Token = "k1-0-x"
	if err := c.Kaldir(); err == nil {
		t.Fatal("uç yokken (404) bir hata dönmeli")
	}
}

// TestIndirmeURLIcin — platform/mimari → doğru paket seçimi (hasım-review KRİTİK-1
// takip). Yeni sunucu indirme_urls haritası döner; ajan kendi OS/arch'ına göre seçer.
func TestIndirmeURLIcin(t *testing.T) {
	b := &SurumBilgi{
		IndirmeURL: "https://x/HizmetraYaziciKurulum.exe",
		IndirmeURLler: map[string]string{
			"windows":     "https://x/HizmetraYaziciKurulum.exe",
			"darwin":      "https://x/HizmetraYazici.dmg",
			"linux_amd64": "https://x/k_linux_amd64.deb",
			"linux_arm64": "https://x/k_linux_arm64.deb",
		},
	}
	testler := []struct{ os, arch, bek string }{
		{"windows", "amd64", "https://x/HizmetraYaziciKurulum.exe"},
		{"darwin", "arm64", "https://x/HizmetraYazici.dmg"}, // os_arch yok → os anahtarı
		{"darwin", "amd64", "https://x/HizmetraYazici.dmg"},
		{"linux", "amd64", "https://x/k_linux_amd64.deb"},
		{"linux", "arm64", "https://x/k_linux_arm64.deb"},
	}
	for _, tt := range testler {
		if g := b.IndirmeURLIcin(tt.os, tt.arch); g != tt.bek {
			t.Errorf("IndirmeURLIcin(%q,%q)=%q, beklenen %q", tt.os, tt.arch, g, tt.bek)
		}
	}
	// Legacy (eski sunucu: yalnız IndirmeURL) → Windows'a düşer, mac/linux BOŞ
	// (installer .exe mac/linux'a verilmemeli; boşsa güncelle şeridi hiç çıkmaz).
	eski := &SurumBilgi{IndirmeURL: "https://x/HizmetraYaziciKurulum.exe"}
	if eski.IndirmeURLIcin("windows", "amd64") == "" {
		t.Error("legacy windows IndirmeURL'e düşmeli")
	}
	if g := eski.IndirmeURLIcin("darwin", "arm64"); g != "" {
		t.Errorf("legacy'de darwin BOŞ dönmeli, geldi %q", g)
	}
	if g := eski.IndirmeURLIcin("linux", "amd64"); g != "" {
		t.Errorf("legacy'de linux BOŞ dönmeli, geldi %q", g)
	}
}
