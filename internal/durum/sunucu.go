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
	"io"
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

	// ── BASKI SORUNU (2026-09-22) ────────────────────────────────────────
	// Bunlar SonHata'dan AYRIDIR: bağlantı sapasağlamken fiş basılamıyor
	// olabilir. Sayfa bu alanları oz.bagli'den TAMAMEN BAĞIMSIZ bir blokta
	// çizer — eskiden ciz() önce oz.bagli'ye baktığı için son_hata dalına
	// HİÇ girilmiyordu ve kullanıcı yeşil kart görüp fişin çıkmadığını fark
	// etmiyordu.
	BaskiSorunu string `json:"baski_sorunu"` // tek cümlelik Türkçe açıklama
	BaskiKodu   string `json:"baski_kodu"`   // teshis kodu (makine tarafı)
	// OnarimEylemi — "ONAR" | "YAZICI_SEC" | "YOK" (tek düğme seçimi)
	OnarimEylemi string `json:"onarim_eylemi"`
	// EylemID — o anki BEKLEYEN eylemin kimliği. /onar isteği bununla
	// eşleşmezse HİÇBİR ŞEY yapılmaz (yerel tarayıcı sekmesi token'ı
	// görebilir; eski bir sekmenin yanlışlıkla onarım tetiklemesi engellenir).
	EylemID   string `json:"eylem_id"`
	SonOnarim string `json:"son_onarim"`  // "14:32 · Mutfak — baskı sırası yeniden çalıştırıldı"
	GeriAlKod string `json:"geri_al_kod"` // dolu ise "Geri Al" düğmesi gösterilir
	GunlukYol string `json:"gunluk_yolu"` // "Günlüğü Aç" düğmesi için (bilgi amaçlı)

	// ── KURULMAMIŞ YAZICI (2026-09-22) ───────────────────────────────────
	// Kablosu TAKILI ama Windows'ta kuyruğu OLMAYAN yazıcılar. Windows bu
	// cihazları "Belirtilmemiş" bölümünde bırakır; yazıcı olarak görünmez,
	// panele hiç düşmez ve kafe sahibinin bunu fark etmesi imkânsızdır.
	// Sayfa her biri için tek bir "Kur" düğmesi çizer.
	KurulabilirYazicilar []KurulabilirYazici `json:"kurulabilir_yazicilar,omitempty"`
}

// KurulabilirYazici — takılı ama kurulmamış bir yazıcının sayfaya taşınan özeti.
type KurulabilirYazici struct {
	Ad      string `json:"ad"`      // önerilen kuyruk adı
	Port    string `json:"port"`    // "USB002"
	Donanim string `json:"donanim"` // kayıt defteri kimliği (bilgi amaçlı)
}

// Sunucu — durum penceresi HTTP sunucusu.
type Sunucu struct {
	Token             string
	ad                string // üst şeritte gösterilen uygulama adı ("Hizmetra Yazıcı")
	surum             string
	ozet              func() Ozet
	gunluk            func(n int) []string
	sablon            *template.Template
	logoURI           template.URL
	onOdaklan         func()                      // /odaklan çağrılınca tetiklenir (main.go: pencere.OneGetir())
	onGuncelle        func()                      // /guncelle çağrılınca tetiklenir (main.go: guncelle() — indir+kur)
	onYenidenEslestir func()                      // /yeniden-eslestir çağrılınca (main.go: token temizle + yeniden başlat)
	onOnar            func()                      // /onar çağrılınca (main.go: bekleyen sorunu onarmayı dene)
	onGeriAl          func()                      // /geri-al çağrılınca (son onarımı geri al)
	onGunlukAc        func()                      // /gunluk-ac çağrılınca (günlük dosyasını aç)
	onYaziciKur       func(ad, port string) error // /yazici-kur çağrılınca
}

// OnarAyarla / GeriAlAyarla / GunlukAcAyarla — YenidenEslestirAyarla ile AYNI
// desen: Yeni()'nin imzasını büyütüp mevcut çağrıları (main.go + testler)
// kırmamak için ayrı setter'lar. nil bırakılırsa ilgili uç yalnız 200 döner.
func (s *Sunucu) OnarAyarla(fn func())     { s.onOnar = fn }
func (s *Sunucu) GeriAlAyarla(fn func())   { s.onGeriAl = fn }
func (s *Sunucu) GunlukAcAyarla(fn func()) { s.onGunlukAc = fn }

// YaziciKurAyarla — sayfadaki "Kur" düğmesi (POST /yazici-kur) tetiklendiğinde
// çağrılacak işlev. nil bırakılırsa uç 501 döner ve sayfa düğmeyi çizmez.
func (s *Sunucu) YaziciKurAyarla(fn func(ad, port string) error) { s.onYaziciKur = fn }

// YenidenEslestirAyarla — kullanıcı durum penceresindeki "Yeniden Eşleştir"
// butonuna basınca (POST /yeniden-eslestir) tetiklenecek callback'i bağlar
// (main.go: yenidenEslestirGovde — token temizle + süreci yeniden başlat →
// yeni 6-hane kod → farklı hesap/kafe). AYRI SETTER: Yeni'nin imzasını büyütüp
// tüm mevcut çağrıları (main.go + testler) kırmamak için (onGuncelle deseninin
// aksine sonradan eklendi). Sayfa tarafı zaten confirm() ile onay alır.
func (s *Sunucu) YenidenEslestirAyarla(fn func()) { s.onYenidenEslestir = fn }

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
	// /yeniden-eslestir — kullanıcı "Yeniden Eşleştir (kod gir)" butonuna basınca
	// POST eder; token'ı temizleyip süreci yeniden başlatır (main.go:
	// yenidenEslestirGovde → boş token → ilkKurulum penceresi → yeni kod). Farklı
	// bir hesaba/kafeye bağlanmanın YOLU budur (token geçerliyken 401-otomatik
	// yeniden eşleşme tetiklenmez). /guncelle ile AYNI güvenlik deseni: token'lı +
	// yalnız POST + callback ASENKRON (süreç birazdan yeniden başlar, 200 hemen dönsün).
	mux.HandleFunc("/yeniden-eslestir", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		if s.onYenidenEslestir != nil {
			go s.onYenidenEslestir()
		}
		w.WriteHeader(http.StatusOK)
	})
	// /onar — kullanıcı sorun şeridindeki "Onar" düğmesine bastı. /guncelle ile
	// AYNI güvenlik deseni (token + yalnız POST + asenkron callback) ARTI
	// EYLEM KİMLİĞİ DOĞRULAMASI: gövdedeki eylem_id ajanın O ANKİ bekleyen
	// eylemiyle eşleşmezse HİÇBİR ŞEY yapılmaz. Böylece açık kalmış eski bir
	// sekme, çoktan geçmiş bir sorunu "onarmaya" kalkamaz.
	mux.HandleFunc("/onar", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		var govde struct {
			EylemID string `json:"eylem_id"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&govde)
		bekleyen := s.ozet().EylemID
		if bekleyen == "" || govde.EylemID != bekleyen {
			http.Error(w, "eylem güncel değil", http.StatusConflict)
			return
		}
		if s.onOnar != nil {
			go s.onOnar()
		}
		w.WriteHeader(http.StatusOK)
	})
	// /geri-al — son onarımı geri alır (defterdeki eski değer yazılır).
	mux.HandleFunc("/geri-al", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		if s.onGeriAl != nil {
			go s.onGeriAl()
		}
		w.WriteHeader(http.StatusOK)
	})
	// /gunluk-ac — günlük dosyasını işletim sisteminin varsayılan uygulamasında
	// açar (gunluk.Yolu()). Destek istemek için kullanıcı dosyayı kolayca bulsun.
	// /yazici-kur — takılı ama kurulmamış yazıcı için Windows kuyruğu açar.
	//
	// SENKRON: /onar'ın aksine sonucu kullanıcıya DÖNÜYORUZ. Kurulum saniyeler
	// sürer ve başarısız olursa kafe sahibinin nedenini görmesi şart; arka plana
	// atsaydık düğmeye basıp hiçbir şey olmadığını sanırdı.
	//
	// Port ve ad SAYFADAN gelir ama İKİSİ DE doğrulanır: sayfa yerel bir
	// tarayıcı sekmesidir, gövdesine güvenilmez.
	mux.HandleFunc("/yazici-kur", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		if s.onYaziciKur == nil {
			http.Error(w, "bu platformda yazıcı kurulumu yok", http.StatusNotImplemented)
			return
		}
		var govde struct {
			Ad   string `json:"ad"`
			Port string `json:"port"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4<<10)).Decode(&govde)

		// PORT SAYFADAN GELDİĞİ GİBİ KABUL EDİLMEZ: yalnız o an GERÇEKTEN
		// kurulabilir olarak listelenen portlardan biri olabilir. Aksi halde
		// eski bir sekme, artık takılı olmayan bir porta kuyruk açtırabilirdi.
		gecerli := false
		for _, k := range s.ozet().KurulabilirYazicilar {
			if k.Port == govde.Port {
				gecerli = true
				break
			}
		}
		if !gecerli {
			http.Error(w, "bu yazıcı artık takılı görünmüyor", http.StatusConflict)
			return
		}
		if err := s.onYaziciKur(govde.Ad, govde.Port); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/gunluk-ac", func(w http.ResponseWriter, r *http.Request) {
		if !s.yetkili(r) {
			http.Error(w, "yetkisiz", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "yalnız POST", http.StatusMethodNotAllowed)
			return
		}
		if s.onGunlukAc != nil {
			go s.onGunlukAc()
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
