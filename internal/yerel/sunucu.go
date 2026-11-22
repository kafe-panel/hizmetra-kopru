// Package yerel — ÇEVRİMDIŞI FİŞ (v0.17.0). İnternet kesilince panel fişi sunucuya
// gönderemez; bunun yerine AYNI bilgisayardaki bu uca (127.0.0.1:YerelPort)
// doğrudan gönderir, ajan yazıcıya basar.
//
// Güvenlik:
//   - Yalnız loopback'e bağlanır (ağdaki başka cihaz erişemez).
//   - Tarayıcı isteği yalnız sunucunun bildirdiği panel kökenlerinden kabul edilir
//     (CORS); her istek sunucunun türettiği X-Yerel-Anahtar'ı taşımalıdır.
//   - Yalnız baskı: hedef, bu bilgisayarın bildiği bir yazıcı ya da özel ağdaki
//     bir ip:9100 olmalıdır. Satış/ödeme verisine erişim YOK; fiş içeriği loglanmaz.
//   - Çift baskı kalkanı: aynı "kimlik" ikinci kez gelirse basılmaz.
//
// https panel → http://127.0.0.1: tarayıcılar loopback'i güvenli sayar (karışık
// içerik engeli yok). Chrome'un yerel ağ erişimi (Private Network Access / Local
// Network Access) ön-isteğine Access-Control-Allow-Private-Network ile cevap verilir;
// yeni Chrome ilk seferde kullanıcıya bir kez izin sorar.
package yerel

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// YerelPort — panelin sabit bildiği kapı (frontend/src/cevrimdisi/yerelYazici.js ile AYNI).
const YerelPort = 47321

// kalkanSuresi — aynı kimlik bu süre içinde ikinci kez basılmaz.
const kalkanSuresi = 24 * time.Hour

// azamiGovde — tek fişin üst sınırı (raster 80 mm uzun fiş ≈ 200 KB).
const azamiGovde = 4 << 20

// Sunucu — yerel fiş ucu.
type Sunucu struct {
	Surum string
	// Bas — süre sınırlı baskı (ajan.BasZamanAsimiyla).
	Bas func(hedef string, veri []byte) error
	// HedefGecerli — hedef bu bilgisayarın yazıcısı mı (nil → yalnız ip:9100 kabul).
	HedefGecerli func(hedef string) bool

	mu       sync.Mutex
	anahtar  string
	kokenler map[string]bool
	basildi  map[string]time.Time
	simdi    func() time.Time
}

// Yeni — yerel sunucu kurar; anahtar/kökenler sonradan YetkiAyarla ile tazelenir.
func Yeni(surum string, bas func(string, []byte) error) *Sunucu {
	return &Sunucu{
		Surum: surum, Bas: bas,
		kokenler: map[string]bool{}, basildi: map[string]time.Time{}, simdi: time.Now,
	}
}

// YetkiAyarla — sunucudan (nabız) gelen anahtar + izinli kökenler.
func (s *Sunucu) YetkiAyarla(anahtar string, kokenler []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.anahtar = anahtar
	s.kokenler = map[string]bool{}
	for _, k := range kokenler {
		k = strings.TrimRight(strings.TrimSpace(k), "/")
		if k != "" {
			s.kokenler[k] = true
		}
	}
}

func (s *Sunucu) kokenIzinli(koken string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kokenler[koken]
}

func (s *Sunucu) anahtarDogru(gelen string) bool {
	s.mu.Lock()
	a := s.anahtar
	s.mu.Unlock()
	if a == "" || gelen == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(gelen)) == 1
}

// ozelAgYazicisi — "192.168.1.50:9100" gibi ÖZEL ağdaki ham-TCP yazıcı hedefi.
func ozelAgYazicisi(hedef string) bool {
	h, p, err := net.SplitHostPort(hedef)
	if err != nil || p != "9100" {
		return false
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsPrivate() || ip.IsLoopback())
}

func (s *Sunucu) hedefIzinli(hedef string) bool {
	if hedef == "" {
		return false
	}
	if s.HedefGecerli != nil && s.HedefGecerli(hedef) {
		return true
	}
	return ozelAgYazicisi(hedef)
}

// zatenBasildi — kalkan: kimlik görüldüyse true; görülmediyse KAYDEDER ve false döner.
func (s *Sunucu) kalkanKontrol(kimlik string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	simdi := s.simdi()
	for k, t := range s.basildi {
		if simdi.Sub(t) > kalkanSuresi {
			delete(s.basildi, k)
		}
	}
	if _, var_ := s.basildi[kimlik]; var_ {
		return true
	}
	s.basildi[kimlik] = simdi
	return false
}

func (s *Sunucu) kalkanSil(kimlik string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.basildi, kimlik)
}

func jsonYaz(w http.ResponseWriter, kod int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(kod)
	_ = json.NewEncoder(w).Encode(v)
}

// Handler — /yerel/durum (GET) + /yerel/bas (POST) + CORS/PNA ön-istekleri.
func (s *Sunucu) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/yerel/durum", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "yöntem", http.StatusMethodNotAllowed)
			return
		}
		if !s.anahtarDogru(r.Header.Get("X-Yerel-Anahtar")) {
			jsonYaz(w, http.StatusUnauthorized, map[string]any{"ok": false, "hata": "anahtar"})
			return
		}
		jsonYaz(w, http.StatusOK, map[string]any{"ok": true, "surum": s.Surum})
	})
	mux.HandleFunc("/yerel/bas", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "yöntem", http.StatusMethodNotAllowed)
			return
		}
		if !s.anahtarDogru(r.Header.Get("X-Yerel-Anahtar")) {
			jsonYaz(w, http.StatusUnauthorized, map[string]any{"ok": false, "hata": "anahtar"})
			return
		}
		var govde struct {
			Hedef   string `json:"hedef"`
			VeriB64 string `json:"veri_b64"`
			Kimlik  string `json:"kimlik"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, azamiGovde)).Decode(&govde); err != nil {
			jsonYaz(w, http.StatusBadRequest, map[string]any{"ok": false, "hata": "govde"})
			return
		}
		if !s.hedefIzinli(govde.Hedef) {
			jsonYaz(w, http.StatusBadRequest, map[string]any{"ok": false, "hata": "Yazıcı bu bilgisayarda bulunamadı."})
			return
		}
		veri, err := base64.StdEncoding.DecodeString(govde.VeriB64)
		if err != nil || len(veri) == 0 {
			jsonYaz(w, http.StatusBadRequest, map[string]any{"ok": false, "hata": "veri"})
			return
		}
		kimlik := strings.TrimSpace(govde.Kimlik)
		if kimlik != "" && s.kalkanKontrol(kimlik) {
			jsonYaz(w, http.StatusOK, map[string]any{"ok": true, "tekrar": true})
			return
		}
		if err := s.Bas(govde.Hedef, veri); err != nil {
			if kimlik != "" {
				s.kalkanSil(kimlik) // basılamadı → kullanıcı yeniden deneyebilsin
			}
			jsonYaz(w, http.StatusBadGateway, map[string]any{"ok": false, "hata": err.Error()})
			return
		}
		jsonYaz(w, http.StatusOK, map[string]any{"ok": true})
	})
	return s.cors(mux)
}

// cors — yalnız izinli panel kökenleri; Chrome yerel ağ ön-isteğine izin cevabı.
func (s *Sunucu) cors(sonraki http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		koken := r.Header.Get("Origin")
		if koken != "" {
			if !s.kokenIzinli(koken) {
				http.Error(w, "köken", http.StatusForbidden)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", koken)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, X-Yerel-Anahtar")
			h.Set("Access-Control-Allow-Private-Network", "true")
			h.Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		sonraki.ServeHTTP(w, r)
	})
}

// Baslat — 127.0.0.1:YerelPort'a bağlanır ve arka planda servis eder.
// Kapı doluysa (ikinci kopya / başka program) hata döner; ajan yine çalışır.
func (s *Sunucu) Baslat() error {
	if s.Bas == nil {
		return errors.New("baskı işlevi yok")
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(YerelPort))
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(l) }()
	return nil
}
