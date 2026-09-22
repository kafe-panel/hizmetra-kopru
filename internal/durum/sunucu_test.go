package durum

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// TestYenidenEslestirTokensizReddedilir — /yeniden-eslestir token ister; tokensiz
// istekte callback ÇAĞRILMAZ (başkası token'ı temizleyip yeniden başlatamasın).
func TestYenidenEslestirTokensizReddedilir(t *testing.T) {
	cagrildi := false
	d := Yeni("Test Kafe", "0.7.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
	d.YenidenEslestirAyarla(func() { cagrildi = true })
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/yeniden-eslestir", "", nil) // token yok
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusForbidden {
		t.Fatalf("tokensiz /yeniden-eslestir 403 beklenir, geldi %d", r.StatusCode)
	}
	if cagrildi {
		t.Fatal("tokensiz istekte yeniden-eşleştir callback'i TETİKLENMEMELİ")
	}
}

// TestYenidenEslestirTokenliCallbackTetikler — doğru token'lı POST, setter ile
// bağlanan callback'i (main.go: yenidenEslestirGovde) ASENKRON tetikler; 200 döner.
func TestYenidenEslestirTokenliCallbackTetikler(t *testing.T) {
	cagrildi := make(chan struct{}, 1)
	d := Yeni("Test Kafe", "0.7.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
	d.YenidenEslestirAyarla(func() { cagrildi <- struct{}{} })
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Post(srv.URL+"/yeniden-eslestir?t="+d.Token, "", nil)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatalf("tokenli /yeniden-eslestir 200 beklenir, geldi %d", r.StatusCode)
	}
	select {
	case <-cagrildi:
	case <-time.After(2 * time.Second):
		t.Fatal("yeniden-eşleştir callback'i zamanında (asenkron) tetiklenmedi")
	}
}

// TestYenidenEslestirYalnizPost — doğru token ama GET → 405; callback set
// edilmese de (nil) tokenli POST panic olmadan 200 döner.
func TestYenidenEslestirYalnizPost(t *testing.T) {
	d := Yeni("Test Kafe", "0.7.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
	srv := httptest.NewServer(d.Handler())
	defer srv.Close()

	r, err := http.Get(srv.URL + "/yeniden-eslestir?t=" + d.Token)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /yeniden-eslestir 405 beklenir, geldi %d", r.StatusCode)
	}
	r2, err := http.Post(srv.URL+"/yeniden-eslestir?t="+d.Token, "", nil)
	if err != nil {
		t.Fatalf("istek hatası: %v", err)
	}
	if r2.StatusCode != http.StatusOK {
		t.Fatalf("nil callback ile tokenli POST 200 beklenir, geldi %d", r2.StatusCode)
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

// ── Sorun şeridi uçları (2026-09-22) ────────────────────────────────────────

func sorunluSunucu(eylemID string, tetiklendi *bool) *Sunucu {
	s := Yeni("Test", "0.0.0", func() Ozet {
		return Ozet{Bagli: true, BaskiSorunu: "Kağıt bitti.", BaskiKodu: "KAGIT_YOK",
			OnarimEylemi: "ONAR", EylemID: eylemID}
	}, func(int) []string { return nil }, nil, nil)
	s.OnarAyarla(func() { *tetiklendi = true })
	return s
}

// TestOnarTokensizReddedilir — /guncelle ile AYNI güvenlik deseni.
func TestOnarTokensizReddedilir(t *testing.T) {
	tetik := false
	s := sorunluSunucu("E1", &tetik)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/onar", strings.NewReader(`{"eylem_id":"E1"}`)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("token'sız /onar reddedilmeliydi, durum %d", w.Code)
	}
	if tetik {
		t.Fatal("token'sız istek callback'i TETİKLEMEMELİ")
	}
}

// TestOnarGETReddedilir — yalnız POST.
func TestOnarGetReddedilir(t *testing.T) {
	tetik := false
	s := sorunluSunucu("E1", &tetik)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/onar?t="+s.Token, nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET reddedilmeliydi, durum %d", w.Code)
	}
	if tetik {
		t.Fatal("GET callback'i TETİKLEMEMELİ")
	}
}

// TestOnarYanlisEylemIDTetiklemez — açık kalmış eski bir sekme, çoktan geçmiş
// bir sorunu "onaramaz".
func TestOnarYanlisEylemIDTetiklemez(t *testing.T) {
	tetik := false
	s := sorunluSunucu("YENI", &tetik)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/onar?t="+s.Token,
		strings.NewReader(`{"eylem_id":"ESKI"}`)))
	if w.Code != http.StatusConflict {
		t.Fatalf("eski eylem_id çakışma dönmeliydi, durum %d", w.Code)
	}
	time.Sleep(50 * time.Millisecond)
	if tetik {
		t.Fatal("yanlış eylem_id ile callback TETİKLENMEMELİ")
	}
}

// TestOnarDogruEylemIDTetikler.
func TestOnarDogruEylemIDTetikler(t *testing.T) {
	var kilit sync.Mutex
	tetik := false
	s := Yeni("Test", "0.0.0", func() Ozet {
		return Ozet{BaskiSorunu: "x", BaskiKodu: "KAGIT_YOK", EylemID: "E9"}
	}, func(int) []string { return nil }, nil, nil)
	s.OnarAyarla(func() { kilit.Lock(); tetik = true; kilit.Unlock() })

	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/onar?t="+s.Token,
		strings.NewReader(`{"eylem_id":"E9"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("doğru eylem_id 200 dönmeliydi, durum %d", w.Code)
	}
	for i := 0; i < 100; i++ {
		kilit.Lock()
		oldu := tetik
		kilit.Unlock()
		if oldu {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("callback tetiklenmedi")
}

// TestGunlukAcUcu — token + POST + callback.
func TestGunlukAcUcu(t *testing.T) {
	var kilit sync.Mutex
	tetik := false
	s := Yeni("Test", "0.0.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
	s.GunlukAcAyarla(func() { kilit.Lock(); tetik = true; kilit.Unlock() })

	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/gunluk-ac", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("token'sız /gunluk-ac reddedilmeliydi, durum %d", w.Code)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/gunluk-ac?t="+s.Token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("200 bekleniyordu, durum %d", w.Code)
	}
	for i := 0; i < 100; i++ {
		kilit.Lock()
		oldu := tetik
		kilit.Unlock()
		if oldu {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("günlük açma callback'i tetiklenmedi")
}

// TestGeriAlUcu — token'sız reddedilir, GET reddedilir.
func TestGeriAlUcu(t *testing.T) {
	s := Yeni("Test", "0.0.0", func() Ozet { return Ozet{} }, func(int) []string { return nil }, nil, nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/geri-al", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("token'sız /geri-al reddedilmeliydi, durum %d", w.Code)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/geri-al?t="+s.Token, nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET reddedilmeliydi, durum %d", w.Code)
	}
}

// TestVeriJsonYeniAlanlariTasir — sayfa sorun şeridini bu alanlardan çizer.
func TestVeriJsonYeniAlanlariTasir(t *testing.T) {
	s := Yeni("Test", "0.0.0", func() Ozet {
		return Ozet{BaskiSorunu: "Kağıt bitti.", BaskiKodu: "KAGIT_YOK",
			OnarimEylemi: "ONAR", EylemID: "E1", SonOnarim: "14:32 · Mutfak — x",
			GeriAlKod: "3", GunlukYol: "/tmp/kopru.log"}
	}, func(int) []string { return nil }, nil, nil)

	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/veri.json?t="+s.Token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("durum %d", w.Code)
	}
	govde := w.Body.String()
	for _, anahtar := range []string{"baski_sorunu", "baski_kodu", "onarim_eylemi", "eylem_id", "son_onarim", "geri_al_kod", "gunluk_yolu"} {
		if !strings.Contains(govde, anahtar) {
			t.Errorf("%q anahtarı /veri.json'da yok", anahtar)
		}
	}
}

// ── /yazici-kur (2026-09-22) ─────────────────────────────────────────────

func kurulumSunucusu(t *testing.T, kurulabilir []KurulabilirYazici, fn func(ad, port string) error) *Sunucu {
	t.Helper()
	s := Yeni("Hizmetra Yazıcı", "0.0.0",
		func() Ozet { return Ozet{Bagli: true, KurulabilirYazicilar: kurulabilir} },
		func(int) []string { return nil }, nil, nil)
	s.Token = "tok"
	if fn != nil {
		s.YaziciKurAyarla(fn)
	}
	return s
}

func kurIstegi(t *testing.T, s *Sunucu, token, govde string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/yazici-kur?t="+token, strings.NewReader(govde))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func TestYaziciKurMutluYol(t *testing.T) {
	var gelenAd, gelenPort string
	s := kurulumSunucusu(t, []KurulabilirYazici{{Ad: "ZJ-80", Port: "USB002"}},
		func(ad, port string) error { gelenAd, gelenPort = ad, port; return nil })

	w := kurIstegi(t, s, "tok", `{"ad":"ZJ-80","port":"USB002"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("200 bekleniyordu, %d geldi: %s", w.Code, w.Body.String())
	}
	if gelenAd != "ZJ-80" || gelenPort != "USB002" {
		t.Errorf("callback'e yanlış değer gitti: %q @ %q", gelenAd, gelenPort)
	}
}

// Listelenmeyen bir porta kurulum YAPILMAMALI: açık kalmış eski bir sekme,
// artık takılı olmayan bir porta kuyruk açtırabilirdi.
func TestYaziciKurListeDisiPortuReddeder(t *testing.T) {
	cagrildi := false
	s := kurulumSunucusu(t, []KurulabilirYazici{{Ad: "ZJ-80", Port: "USB002"}},
		func(string, string) error { cagrildi = true; return nil })

	w := kurIstegi(t, s, "tok", `{"ad":"ZJ-80","port":"USB009"}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("409 bekleniyordu, %d geldi", w.Code)
	}
	if cagrildi {
		t.Error("liste dışı port için kurulum çağrılmamalıydı")
	}
}

func TestYaziciKurYetkiVeYontem(t *testing.T) {
	s := kurulumSunucusu(t, []KurulabilirYazici{{Ad: "ZJ-80", Port: "USB002"}},
		func(string, string) error { return nil })

	if w := kurIstegi(t, s, "yanlis", `{"ad":"ZJ-80","port":"USB002"}`); w.Code != http.StatusForbidden {
		t.Errorf("yanlış token 403 vermeliydi, %d geldi", w.Code)
	}
	r := httptest.NewRequest(http.MethodGet, "/yazici-kur?t=tok", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET 405 vermeliydi, %d geldi", w.Code)
	}
}

// Windows dışı: callback bağlı değil → 501, panik yok.
func TestYaziciKurDesteklenmeyenPlatform(t *testing.T) {
	s := kurulumSunucusu(t, []KurulabilirYazici{{Ad: "ZJ-80", Port: "USB002"}}, nil)
	if w := kurIstegi(t, s, "tok", `{"ad":"ZJ-80","port":"USB002"}`); w.Code != http.StatusNotImplemented {
		t.Errorf("501 bekleniyordu, %d geldi", w.Code)
	}
}

// Kurulum hatası kullanıcıya AYNEN dönmeli — düğmeye basıp sessizlik görmesin.
func TestYaziciKurHatayiKullaniciyaDoner(t *testing.T) {
	s := kurulumSunucusu(t, []KurulabilirYazici{{Ad: "ZJ-80", Port: "USB002"}},
		func(string, string) error { return errors.New("yönetici yetkisi gerekiyor") })

	w := kurIstegi(t, s, "tok", `{"ad":"ZJ-80","port":"USB002"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("400 bekleniyordu, %d geldi", w.Code)
	}
	if !strings.Contains(w.Body.String(), "yönetici yetkisi") {
		t.Errorf("hata metni kullanıcıya dönmeli, gelen: %q", w.Body.String())
	}
}
