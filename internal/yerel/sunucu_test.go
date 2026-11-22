package yerel

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const koken = "https://panel.hizmetra.com"

func kur(t *testing.T) (*Sunucu, *[]string) {
	t.Helper()
	var basilan []string
	s := Yeni("0.17.0", func(h string, v []byte) error {
		basilan = append(basilan, h+":"+string(v))
		return nil
	})
	s.HedefGecerli = func(h string) bool { return h == "POS-80" }
	s.YetkiAyarla("gizli", []string{koken + "/"})
	return s, &basilan
}

func istek(s *Sunucu, yontem, yol, govde, anahtar, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(yontem, yol, strings.NewReader(govde))
	if anahtar != "" {
		r.Header.Set("X-Yerel-Anahtar", anahtar)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func basGovde(hedef, veri, kimlik string) string {
	return `{"hedef":"` + hedef + `","veri_b64":"` + base64.StdEncoding.EncodeToString([]byte(veri)) + `","kimlik":"` + kimlik + `"}`
}

func TestDogruAnahtarlaBasar(t *testing.T) {
	s, basilan := kur(t)
	w := istek(s, "POST", "/yerel/bas", basGovde("POS-80", "FIS", "k1"), "gizli", koken)
	if w.Code != 200 || len(*basilan) != 1 || (*basilan)[0] != "POS-80:FIS" {
		t.Fatalf("kod=%d basilan=%v", w.Code, *basilan)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != koken {
		t.Fatal("CORS başlığı yok")
	}
}

func TestAyniKimlikIkinciKezBasilmaz(t *testing.T) {
	s, basilan := kur(t)
	istek(s, "POST", "/yerel/bas", basGovde("POS-80", "FIS", "k1"), "gizli", koken)
	w := istek(s, "POST", "/yerel/bas", basGovde("POS-80", "FIS", "k1"), "gizli", koken)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "tekrar") || len(*basilan) != 1 {
		t.Fatalf("çift baskı: kod=%d basilan=%d", w.Code, len(*basilan))
	}
}

func TestYanlisAnahtarYanlisKokenYabanciHedefReddedilir(t *testing.T) {
	s, basilan := kur(t)
	if w := istek(s, "POST", "/yerel/bas", basGovde("POS-80", "X", "a"), "yanlis", koken); w.Code != 401 {
		t.Fatalf("yanlış anahtar: %d", w.Code)
	}
	if w := istek(s, "POST", "/yerel/bas", basGovde("POS-80", "X", "b"), "gizli", "https://kotu.site"); w.Code != 403 {
		t.Fatalf("yabancı köken: %d", w.Code)
	}
	if w := istek(s, "POST", "/yerel/bas", basGovde("8.8.8.8:9100", "X", "c"), "gizli", koken); w.Code != 400 {
		t.Fatalf("dış ağ hedefi: %d", w.Code)
	}
	if w := istek(s, "POST", "/yerel/bas", basGovde("192.168.1.50:9100", "X", "d"), "gizli", koken); w.Code != 200 {
		t.Fatalf("özel ağ yazıcısı kabul edilmeli: %d", w.Code)
	}
	if len(*basilan) != 1 {
		t.Fatalf("yalnız özel ağ fişi basılmalıydı: %v", *basilan)
	}
}

func TestAnahtarYokkenHicbirSeyKabulEdilmez(t *testing.T) {
	s := Yeni("0.17.0", func(string, []byte) error { return nil })
	if w := istek(s, "GET", "/yerel/durum", "", "", ""); w.Code != 401 {
		t.Fatalf("anahtarsız sunucu: %d", w.Code)
	}
}

func TestOnIstekYerelAgIzniVerir(t *testing.T) {
	s, _ := kur(t)
	w := istek(s, "OPTIONS", "/yerel/bas", "", "", koken)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Private-Network") != "true" {
		t.Fatalf("ön-istek: %d %v", w.Code, w.Header())
	}
}

func TestBaskiHatasiKalkaniBosaltir(t *testing.T) {
	s, _ := kur(t)
	hata := true
	s.Bas = func(string, []byte) error {
		if hata {
			return errors.New("kağıt yok")
		}
		return nil
	}
	if w := istek(s, "POST", "/yerel/bas", basGovde("POS-80", "X", "k9"), "gizli", koken); w.Code != 502 {
		t.Fatalf("hata kodu: %d", w.Code)
	}
	hata = false
	w := istek(s, "POST", "/yerel/bas", basGovde("POS-80", "X", "k9"), "gizli", koken)
	if w.Code != 200 || strings.Contains(w.Body.String(), "tekrar") {
		t.Fatalf("yeniden deneme basmalıydı: %d %s", w.Code, w.Body.String())
	}
}
