// Package kurulum — ajanın ilk-kurulum sihirbazı: 127.0.0.1'e bağlı mini HTTP
// sunucusu + gömülü tek sayfa, internal/durum ile AYNI desende (token'lı
// loopback + //go:embed). WebView penceresinde açılır (main.go: pencere.Ac),
// sistem tarayıcısında DEĞİL — zenity.Entry'nin yerini alır (v0.4.0, madde 3:
// eski diyaloğun metni "Kuruluma hoş geldiniz!1) Hizmetra Panel'i a…" gibi
// OKUNAMAYACAK KADAR kırpılıyordu, emre 2026-08-16 ekran görüntüsü kanıtlı).
//
// GİZLİLİK: /eslestir-dene ucu KASITLI OLARAK token istemez — kurulum kodu
// henüz hiçbir cihaza bağlı değildir (eşleşme tam olarak bu uçta YAPILIR) ve
// loopback'e zaten yalnız bu bilgisayardaki süreçler erişebilir. "/" sayfası
// ise durum sunucusuyla AYNI şekilde token ister (savunma katmanı, ör. aynı
// makinedeki başka bir kullanıcı oturumu bu pencereyi göremesin).
package kurulum

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

//go:embed sayfa.html
var sayfaHTML string

//go:embed logo.png
var logoPNG []byte

// EslestirFonk — main.go'nun enjekte ettiği eşleştirme denemesi. main.go'daki
// eslestirmeDene(kod, makineAdi) ile AYNI davranış: kodu bilinen sunucuların
// hepsinde dener, KABUL eden sunucunun cevabını ve adresini döner. Makine adı
// closure ile önceden bağlanmış gelir (bu paket makine adını bilmez).
type EslestirFonk func(kod string) (*api.EslestirCevap, string, error)

// EslesmeSonucu — başarılı eşleşmede Sonuc() kanalından main.go'ya bildirilir.
type EslesmeSonucu struct {
	Cevap  *api.EslestirCevap
	Sunucu string // kazanan sunucu kökü (ör. https://api.hizmetra.com)
}

// Sunucu — kurulum sihirbazı HTTP sunucusu.
type Sunucu struct {
	Token    string
	surum    string
	kodOneri string
	esle     EslestirFonk
	sablon   *template.Template
	logoURI  template.URL
	sonuc    chan EslesmeSonucu // buffered(1); ilk başarılı eşleşmede doldurulur
}

// Yeni — kurulum sunucusu kurar. kodOneri panodan önerilen 6 haneli kodu
// (yoksa "") sayfaya önceden doldurur (main.go: panodanKodOner()). esle her
// "Bağlan" denemesinde çağrılır.
func Yeni(surum, kodOneri string, esle EslestirFonk) *Sunucu {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	uri := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG))
	return &Sunucu{
		Token:    hex.EncodeToString(b),
		surum:    surum,
		kodOneri: kodOneri,
		esle:     esle,
		sablon:   template.Must(template.New("k").Parse(sayfaHTML)),
		logoURI:  uri,
		sonuc:    make(chan EslesmeSonucu, 1),
	}
}

// Sonuc — başarılı eşleşmeyi bildiren (yalnız okunur) kanal. main.go bunu
// bekler; sihirbaz penceresi kullanıcı tarafından kapatılırsa main.go bunu
// pencere.OneGetir() yoklamasıyla AYRICA ele alır (bu paketin bilmediği bir
// şey — burada yalnız "başarılı eşleşme" olayı var).
func (s *Sunucu) Sonuc() <-chan EslesmeSonucu { return s.sonuc }

// sayfaVeri — sayfa.html şablonuna geçen veri.
type sayfaVeri struct {
	Surum    string
	KodOneri string
	LogoURI  template.URL
}

func (s *Sunucu) yetkili(r *http.Request) bool {
	return r.URL.Query().Get("t") == s.Token
}

// eslestirYanit — POST /eslestir-dene JSON cevabı.
type eslestirYanit struct {
	Basarili  bool   `json:"basarili"`
	Hata      string `json:"hata,omitempty"`
	IsletmeAd string `json:"isletme_ad,omitempty"`
}

// Handler — sayfa + eşleştirme ucu. Yalnız "/" token ister; "/eslestir-dene"
// KASITLI OLARAK istemez (paket dokümantasyonuna bkz.).
func (s *Sunucu) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = s.sablon.Execute(w, sayfaVeri{
			Surum:    s.surum,
			KodOneri: s.kodOneri,
			LogoURI:  s.logoURI,
		})
	})
	mux.HandleFunc("/eslestir-dene", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		govde, err := io.ReadAll(io.LimitReader(r.Body, 64))
		if err != nil {
			http.Error(w, "gövde okunamadı", http.StatusBadRequest)
			return
		}
		kod := strings.TrimSpace(string(govde))

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		if len(kod) != 6 {
			_ = json.NewEncoder(w).Encode(eslestirYanit{Hata: "Kod 6 haneli olmalı."})
			return
		}
		cevap, sunucu, err := s.esle(kod)
		if err != nil {
			mesaj := "Kod geçersiz veya süresi dolmuş. Panelden yeni kod alıp tekrar deneyin."
			if err != api.ErrKodGecersiz {
				mesaj = "Sunucuya ulaşılamadı: " + err.Error() + ". İnternet bağlantınızı kontrol edin."
			}
			_ = json.NewEncoder(w).Encode(eslestirYanit{Hata: mesaj})
			return
		}
		_ = json.NewEncoder(w).Encode(eslestirYanit{Basarili: true, IsletmeAd: cevap.IsletmeAd})

		// Buffered(1): main.go henüz okumadıysa dolar; okuduysa (teoride çift
		// tık/çift istek) ikinci başarı sessizce yok sayılır — main.go artık
		// yalnızca ilk eşleşmeyi işler.
		select {
		case s.sonuc <- EslesmeSonucu{Cevap: cevap, Sunucu: sunucu}:
		default:
		}
	})
	return mux
}

// Baslat — 127.0.0.1'de boş bir porta bağlanır ve arka planda servis eder.
// Açılacak tam adresi ("http://127.0.0.1:PORT/?t=TOKEN") döndürür — çağıran
// bunu pencere.Ac ile açar. Token yalnız bu adreste; loglanmaz.
func (s *Sunucu) Baslat() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: s.Handler()}
	go func() { _ = srv.Serve(l) }()
	return "http://" + l.Addr().String() + "/?t=" + s.Token, nil
}
