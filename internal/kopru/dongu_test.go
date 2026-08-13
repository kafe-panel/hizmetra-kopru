package kopru

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

func zarfYaz(w http.ResponseWriter, kod int, basarili bool, veri any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(kod)
	ham, _ := json.Marshal(veri)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": basarili, "data": json.RawMessage(ham), "message": "",
	})
}

// TestCiftBaskiKalkani — SUNUCU AYNI İŞİ İKİ KEZ VERİRSE ajan ikinci kez BASMAZ.
// Bu, sunucudaki 300sn yeniden-sahiplenme penceresinin ajan-tarafı ikizidir:
// ajan yavaş sanılıp iş geri verilse bile fiş iki kez çıkmaz.
func TestCiftBaskiKalkani(t *testing.T) {
	var baskiSayisi int
	var kilit sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/kopru/isler":
			// Her seferinde AYNI işi ver (sunucu işi geri vermiş gibi).
			zarfYaz(w, 200, true, []map[string]any{
				{"is_id": 7, "hedef": "POS-80", "tip": "kasa", "icerik_b64": base64.StdEncoding.EncodeToString([]byte("FIS"))},
			})
		case "/api/kopru/isler/sonuc":
			zarfYaz(w, 200, true, map[string]any{"islenen": 1})
		default:
			zarfYaz(w, 200, true, map[string]any{})
		}
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	ajan := Yeni(istemci, func(hedef string, veri []byte) error {
		kilit.Lock()
		baskiSayisi++
		kilit.Unlock()
		return nil
	}, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})
	ajan.direktifAyarla(0, 1)

	// Aynı işi üç tur boyunca işlet.
	isler, err := istemci.Isler(0)
	if err != nil {
		t.Fatalf("iş çekilemedi: %v", err)
	}
	ajan.isleriBas(isler)
	ajan.isleriBas(isler)
	ajan.isleriBas(isler)

	kilit.Lock()
	defer kilit.Unlock()
	if baskiSayisi != 1 {
		t.Fatalf("aynı iş %d kez basıldı, 1 olmalıydı (çift baskı kalkanı çalışmadı)", baskiSayisi)
	}
}

func TestBaskiHatasiSunucuyaBildirilir(t *testing.T) {
	var gelenSonuc []api.Sonuc
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/kopru/isler/sonuc" {
			var govde struct {
				Sonuclar []api.Sonuc `json:"sonuclar"`
			}
			_ = json.NewDecoder(r.Body).Decode(&govde)
			gelenSonuc = govde.Sonuclar
			zarfYaz(w, 200, true, map[string]any{"islenen": 1})
			return
		}
		zarfYaz(w, 200, true, map[string]any{})
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	ajan := Yeni(istemci, func(hedef string, veri []byte) error {
		return errors.New("kağıt yok")
	}, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})

	ajan.isleriBas([]api.Is{{IsID: 9, Hedef: "POS-80", Tip: "kasa", IcerikB64: "RklT"}})

	if len(gelenSonuc) != 1 || gelenSonuc[0].Durum != "hata" {
		t.Fatalf("hata sonucu bildirilmedi: %+v", gelenSonuc)
	}
	if gelenSonuc[0].Hata == "" {
		t.Error("hata mesajı boş gönderildi")
	}
	// Başarısız iş kalkana YAZILMAMALI — yeniden denenebilmeli.
	if ajan.zatenBasildi(9) {
		t.Error("başarısız iş 'basıldı' olarak işaretlendi")
	}
}

func TestBozukIcerikHataOlarakBildirilir(t *testing.T) {
	var gelenSonuc []api.Sonuc
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/kopru/isler/sonuc" {
			var govde struct {
				Sonuclar []api.Sonuc `json:"sonuclar"`
			}
			_ = json.NewDecoder(r.Body).Decode(&govde)
			gelenSonuc = govde.Sonuclar
		}
		zarfYaz(w, 200, true, map[string]any{"islenen": 1})
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	basildi := false
	ajan := Yeni(istemci, func(hedef string, veri []byte) error {
		basildi = true
		return nil
	}, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})

	ajan.isleriBas([]api.Is{{IsID: 3, Hedef: "POS-80", IcerikB64: "bu-base64-degil!!!"}})

	if basildi {
		t.Error("bozuk içerik yazıcıya gönderildi")
	}
	if len(gelenSonuc) != 1 || gelenSonuc[0].Durum != "hata" {
		t.Fatalf("bozuk içerik hata olarak bildirilmedi: %+v", gelenSonuc)
	}
}

// TestNabizDirektifiUygulanir — sunucu bekle_sn=0/poll_sn=5 derse ajan UYAR.
// Bu, 1000 cihaza ajan güncellemeden ölçeklenmenin tek mekanizması.
func TestNabizDirektifiUygulanir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 200, true, map[string]any{"ok": true, "poll_sn": 5, "bekle_sn": 0})
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-x"
	ajan := Yeni(istemci, func(string, []byte) error { return nil },
		func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})

	if bekle, _ := ajan.direktif(); bekle != 25 {
		t.Fatalf("başlangıç bekle_sn 25 olmalıydı: %d", bekle)
	}
	ajan.nabizAt()
	bekle, poll := ajan.direktif()
	if bekle != 0 || poll != 5 {
		t.Fatalf("sunucu direktifi uygulanmadı: bekle=%d poll=%d", bekle, poll)
	}
}

func TestYetkisizDurumaYansir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zarfYaz(w, 401, false, nil)
	}))
	defer srv.Close()

	istemci := api.New(srv.URL)
	istemci.Token = "k1-0-iptal"
	durum := &Durum{}
	ajan := Yeni(istemci, func(string, []byte) error { return nil },
		func() ([]api.Yazici, error) { return nil, nil }, "test", durum)

	dur := make(chan struct{})
	bitti := make(chan struct{})
	go func() { ajan.IsDongusu(dur); close(bitti) }()

	time.Sleep(200 * time.Millisecond)
	close(dur)
	select {
	case <-bitti:
	case <-time.After(2 * time.Second):
		t.Fatal("döngü durdurulamadı")
	}

	d := durum.Oku()
	if d.Bagli {
		t.Error("401 sonrası bağlı görünüyor")
	}
	if d.SonHata == "" {
		t.Error("401 sonrası kullanıcıya mesaj yok")
	}
}
