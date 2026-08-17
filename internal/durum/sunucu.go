// Package durum — ajanın yerel durum penceresi: 127.0.0.1'e bağlı mini HTTP
// sunucusu + gömülü tek sayfa. Tarayıcıda açılır (WebView/Fyne YOK: cgo'suz
// tek exe kalır). Yalnız loopback'e bind edilir ve rastgele bir token ister,
// böylece aynı makinedeki başka bir kullanıcı/işlem durumu okuyamaz.
//
// GİZLİLİK: sayfa yalnız durum + yazıcı adları + günlük satırlarını gösterir.
// Fiş İÇERİĞİ hiçbir zaman loglanmadığı için burada da görünmez.
package durum

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"net"
	"net/http"
	"strings"
)

//go:embed sayfa.html
var sayfaHTML string

//go:embed logo.png
var logoPNG []byte

// Ozet — durum penceresinin gösterdiği anlık özet (main.go tarafından toplanır).
type Ozet struct {
	Bagli     bool     `json:"bagli"`
	IsletmeAd string   `json:"isletme_ad"`
	Sunucu    string   `json:"sunucu"`    // bağlı olunan API kökü (panel linki bundan türetilir)
	Surum     string   `json:"surum"`     // ajan sürümü (şu an çalışan)
	Yazicilar []string `json:"yazicilar"` // bu bilgisayarda bulunan yazıcılar
	SonIsler  []string `json:"son_isler"` // son baskı işleri (fiş şeridi özeti)
	SonHata   string   `json:"son_hata"`  // en son hata (boşsa sorun yok)
	SonBaski  string   `json:"son_baski"` // "15:04" — en son fiş saati (boşsa henüz basılmadı)
	// GuncelSurum/IndirmeURL — sunucuda daha yeni bir sürüm varsa dolu olur;
	// sayfa bunu görünce "Güncelle" şeridini gösterir (POST /guncelle → indir+kur).
	GuncelSurum string `json:"guncel_surum"`
	IndirmeURL  string `json:"indirme_url"`
}

// Sunucu — durum penceresi HTTP sunucusu.
type Sunucu struct {
	Token      string
	ad         string // üst şeritte gösterilen uygulama adı ("Hizmetra Yazıcı")
	surum      string
	ozet       func() Ozet
	gunluk     func(n int) []string
	sablon     *template.Template
	logoURI    template.URL
	onOdaklan  func() // /odaklan çağrılınca tetiklenir (main.go: pencere.OneGetir())
	onGuncelle func() // /guncelle çağrılınca tetiklenir (main.go: guncelle() — indir+kur)
}

// Yeni — durum sunucusu kurar. ozet anlık durumu, gunluk son N satırı döndürür.
// onOdaklan — v0.4.0: aynı bilgisayarda ikinci bir kopya açılıp tek-kopya
// kilidini alamadığında POST /odaklan ile bu sunucuya "pencereni öne getir"
// sinyali gönderir (bkz. cmd/hizmetra-kopru main.go: digerKopyayaOdaklanDene).
// onGuncelle — kullanıcı sayfadaki "Güncelle" butonuna basınca POST /guncelle
// ile tetiklenir (main.go: guncelle() — installer'ı indirip kurar). Her iki
// callback de nil verilebilir (ör. testlerde), o durumda ilgili uç yalnız 200 döner.
func Yeni(isletmeAd, surum string, ozet func() Ozet, gunluk func(int) []string, onOdaklan, onGuncelle func()) *Sunucu {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	uri := template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(logoPNG))
	return &Sunucu{
		Token:      hex.EncodeToString(b),
		ad:         isletmeAd,
		surum:      surum,
		ozet:       ozet,
		gunluk:     gunluk,
		sablon:     template.Must(template.New("s").Parse(sayfaHTML)),
		logoURI:    uri,
		onOdaklan:  onOdaklan,
		onGuncelle: onGuncelle,
	}
}

// sayfaVeri — sayfa.html şablonuna geçen veri. IlkVeri, /veri.json ile AYNI
// gövdeyi taşır (ilk boyama fetch beklemeden anında olur; JS sonra 3sn'de bir tazeler).
type sayfaVeri struct {
	Token   string
	Ad      string
	Surum   string
	LogoURI template.URL
	IlkVeri template.JS
}

// veriGovdesi — hem /veri.json hem ilk-render için ortak JSON gövde.
func (s *Sunucu) veriGovdesi() map[string]any {
	oz := s.ozet()
	return map[string]any{
		"ozet":      oz,
		"gunluk":    s.gunluk(200),
		"panel_url": panelURLTuret(oz.Sunucu),
	}
}

func (s *Sunucu) yetkili(r *http.Request) bool {
	return r.URL.Query().Get("t") == s.Token
}

// Handler — sayfa + veri uçları. İkisi de doğru token ister.
func (s *Sunucu) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		ilk, _ := json.Marshal(s.veriGovdesi()) // json.Marshal <,>,& kaçışlar → <script> güvenli
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = s.sablon.Execute(w, sayfaVeri{
			Token:   s.Token,
			Ad:      s.ad,
			Surum:   s.surum,
			LogoURI: s.logoURI,
			IlkVeri: template.JS(ilk),
		})
	})
	mux.HandleFunc("/veri.json", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(s.veriGovdesi())
	})
	// /odaklan — v0.4.0: ikinci bir kopya tek-kopya kilidini alamayınca buraya
	// POST eder; bu çalışan kopyayı öne getirir (bkz. main.go: odaklanGeldi).
	// Eski "zaten çalışıyor, Güncelle/Onar/Kaldır?" zenity diyaloğunun yerini
	// alır — soru sormaz, doğrudan pencereyi öne getirir.
	mux.HandleFunc("/odaklan", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		// onOdaklan (main.go: pencere.OneGetir(), açık pencere yoksa YENİ bir
		// WebView penceresi açar) soğuk başlangıçta (WebView2 ortamı ilk kez
		// kuruluyorsa) saniyelerce sürebilir — gerçek Windows'ta ölçüldü. ASENKRON
		// tetiklenir ki 200 yanıtı HER ZAMAN hızlı dönsün: ikinci sürecin kısa
		// (~1sn) istemci zaman aşımı, yavaş bir pencere açılışı yüzünden
		// yanlışlıkla "ulaşılamadı" görmesin — sinyal zaten iletildi, çağıran
		// pencerenin açılmasını beklemeden çıkabilir.
		if s.onOdaklan != nil {
			go s.onOdaklan()
		}
		w.WriteHeader(http.StatusOK)
	})
	// /guncelle — kullanıcı "Güncelle" butonuna basınca POST eder; yeni sürümün
	// installer'ını indirip kurar (main.go: guncelle()). /odaklan ile AYNI desen:
	// token'lı + yalnız POST + callback ASENKRON tetiklenir ki 200 hemen dönsün
	// (indirme uzun sürebilir; buton tarafı yanıtı beklemez).
	mux.HandleFunc("/guncelle", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		if s.onGuncelle != nil {
			go s.onGuncelle()
		}
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

// Baslat — 127.0.0.1'de boş bir porta bağlanır ve arka planda servis eder.
// Açılacak tam adresi ("http://127.0.0.1:PORT/?t=TOKEN") VE bağlandığı port
// numarasını döndürür. Port, aynı bilgisayardaki ikinci bir kopyanın
// /odaklan'a ulaşabilmesi için ayar dosyasına yazılır (main.go:
// baslatDurumSunucusu) — sunucu HER başlangıçta rastgele bir porta bağlanır,
// bu yüzden port önceden bilinemez. Token yalnız dönen adreste; loglanmaz.
func (s *Sunucu) Baslat() (string, int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", 0, err
	}
	srv := &http.Server{Handler: s.Handler()}
	go func() { _ = srv.Serve(l) }()
	port := l.Addr().(*net.TCPAddr).Port
	return "http://" + l.Addr().String() + "/?t=" + s.Token, port, nil
}

// panelURLTuret — API kökünden panel adresini türetir (api.X → panel.X).
func panelURLTuret(sunucu string) string {
	if sunucu == "" {
		return ""
	}
	if strings.Contains(sunucu, "api.") {
		return strings.Replace(sunucu, "api.", "panel.", 1)
	}
	return sunucu
}
