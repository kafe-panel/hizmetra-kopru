package kopru

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

// sahteSunucu — hep AYNI işi veren ve sonucu kabul eden mini sunucu.
func sahteSunucu(t *testing.T, isID int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/kopru/isler":
			zarfYaz(w, 200, true, []map[string]any{
				{"is_id": isID, "hedef": "POS-80", "tip": "kasa",
					"icerik_b64": base64.StdEncoding.EncodeToString([]byte("FIS"))},
			})
		case "/api/kopru/isler/sonuc":
			zarfYaz(w, 200, true, map[string]any{"islenen": 1})
		default:
			zarfYaz(w, 200, true, map[string]any{})
		}
	}))
}

// TestKalkanYenidenBaslatmadaCiftBaskiyiOnler — ajan yeniden başlasa bile aynı
// iş İKİNCİ KEZ BASILMAZ. Kalkan yalnız bellekte olduğu sürece güncelleme/çökme
// sonrası aynı fiş iki kez çıkıyordu (mutfağa iki kez düşen sipariş).
func TestKalkanYenidenBaslatmadaCiftBaskiyiOnler(t *testing.T) {
	srv := sahteSunucu(t, 7)
	defer srv.Close()
	dizin := t.TempDir()

	var sayac int
	var kilit sync.Mutex
	bas := func(hedef string, veri []byte) error {
		kilit.Lock()
		sayac++
		kilit.Unlock()
		return nil
	}

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	isler, err := istemci.Isler(0)
	if err != nil {
		t.Fatalf("iş çekilemedi: %v", err)
	}

	ilk := Yeni(istemci, bas, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})
	ilk.KalkanDosyasiAyarla(KalkanYolu(dizin))
	ilk.isleriBas(isler)

	// YENİ bir ajan örneği (süreç yeniden başlamış gibi) aynı dizini yükler.
	ikinci := Yeni(istemci, bas, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})
	ikinci.KalkanDosyasiAyarla(KalkanYolu(dizin))
	ikinci.isleriBas(isler)

	kilit.Lock()
	defer kilit.Unlock()
	if sayac != 1 {
		t.Fatalf("yeniden başlatma sonrası %d kez basıldı, 1 olmalıydı", sayac)
	}
}

// TestKalkanEskiKayitlariDuser — 1 saatten eski kayıtlar yüklenmemeli.
func TestKalkanEskiKayitlariDuser(t *testing.T) {
	dizin := t.TempDir()
	yol := KalkanYolu(dizin)
	ham, _ := json.Marshal(kalkanDosya{Basildi: []kalkanKayit{
		{IsID: 1, Zaman: time.Now().Add(-2 * time.Hour)},
		{IsID: 2, Zaman: time.Now().Add(-5 * time.Minute)},
	}})
	if err := os.WriteFile(yol, ham, 0o600); err != nil {
		t.Fatal(err)
	}
	a := Yeni(nil, nil, nil, "test", &Durum{})
	a.KalkanDosyasiAyarla(yol)

	if a.zatenBasildi(1) {
		t.Error("2 saatlik kayıt düşmeliydi")
	}
	if !a.zatenBasildi(2) {
		t.Error("5 dakikalık kayıt korunmalıydı")
	}
}

// TestKalkanHaritasiSismez — 600 TAZE kayıt sonrası harita şişmemeli ama
// tazeler korunmalı; eski kod yalnız 500'ü aşınca ve YALNIZ eskileri siliyordu.
func TestKalkanHaritasiSismez(t *testing.T) {
	dizin := t.TempDir()
	a := Yeni(nil, nil, nil, "test", &Durum{})
	a.KalkanDosyasiAyarla(KalkanYolu(dizin))

	for i := int64(1); i <= 600; i++ {
		a.basildiIsaretle(i)
	}
	if !a.zatenBasildi(600) {
		t.Error("taze kayıt kaybolmamalı")
	}
	// Eski bir kaydı elle bayatlatıp yeni bir işaretleme yapınca düşmeli.
	a.kayitKilit.Lock()
	a.basildiKayit[1] = time.Now().Add(-2 * time.Hour)
	a.kayitKilit.Unlock()
	a.basildiIsaretle(601)
	if a.zatenBasildi(1) {
		t.Error("bayat kayıt her işaretlemede süzülmeliydi")
	}
}

// TestKalkanBozukDosyaPanikYapmaz — yarım JSON ajanı durdurmamalı.
func TestKalkanBozukDosyaPanikYapmaz(t *testing.T) {
	dizin := t.TempDir()
	yol := KalkanYolu(dizin)
	if err := os.WriteFile(yol, []byte("{bozuk"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := Yeni(nil, nil, nil, "test", &Durum{})
	a.KalkanDosyasiAyarla(yol)
	if a.zatenBasildi(1) {
		t.Error("bozuk dosyadan kayıt yüklenmemeli")
	}
	a.basildiIsaretle(5)
	if _, err := os.Stat(filepath.Join(dizin, "basildi.json.tmp")); !os.IsNotExist(err) {
		t.Error("geçici dosya kalmış (atomik yazım bozuk)")
	}
}

// TestBildirilemeyenSonucSiradakiTurdaGonderilir — sonuç POST'u başarısız
// olursa kuyruğa alınır ve bir sonraki turda tekrar gönderilir.
func TestBildirilemeyenSonucSiradakiTurdaGonderilir(t *testing.T) {
	var kilit sync.Mutex
	var gelenTurlar [][]api.Sonuc
	basarisiz := true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/kopru/isler/sonuc" {
			zarfYaz(w, 200, true, map[string]any{})
			return
		}
		var govde struct {
			Sonuclar []api.Sonuc `json:"sonuclar"`
		}
		_ = json.NewDecoder(r.Body).Decode(&govde)
		kilit.Lock()
		gelenTurlar = append(gelenTurlar, govde.Sonuclar)
		ilkTur := basarisiz
		basarisiz = false
		kilit.Unlock()
		if ilkTur {
			zarfYaz(w, 500, false, map[string]any{})
			return
		}
		zarfYaz(w, 200, true, map[string]any{"islenen": len(govde.Sonuclar)})
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	a := Yeni(istemci, func(string, []byte) error { return nil },
		func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})
	a.KalkanDosyasiAyarla(KalkanYolu(t.TempDir()))

	is1 := []api.Is{{IsID: 11, Hedef: "POS-80", IcerikB64: base64.StdEncoding.EncodeToString([]byte("A"))}}
	is2 := []api.Is{{IsID: 12, Hedef: "POS-80", IcerikB64: base64.StdEncoding.EncodeToString([]byte("B"))}}
	a.isleriBas(is1) // sonuç POST'u 500 → kuyruğa girer
	a.isleriBas(is2) // ikinci tur: 11 + 12 birlikte gitmeli

	kilit.Lock()
	defer kilit.Unlock()
	if len(gelenTurlar) != 2 {
		t.Fatalf("2 sonuç POST'u bekleniyordu, %d geldi", len(gelenTurlar))
	}
	if len(gelenTurlar[1]) != 2 {
		t.Fatalf("ikinci turda 2 sonuç bekleniyordu, %d geldi: %+v", len(gelenTurlar[1]), gelenTurlar[1])
	}
}
