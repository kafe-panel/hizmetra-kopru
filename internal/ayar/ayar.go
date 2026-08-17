// Package ayar — kalıcı yapılandırma (%LOCALAPPDATA%\HizmetraKopru\config.json).
//
// Cihaz token'ı BURADA durur. Dosya kullanıcı profilinde (diğer Windows
// kullanıcıları okuyamaz) ve token yalnız BU kafenin baskı işlerine yetki
// verir — para/PII yetkisi YOKTUR. Çalınırsa panelden "Kaldır" ile anında
// iptal edilir. (v2: DPAPI ile şifreleme.)
package ayar

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Ayar — diske yazılan yapılandırma.
type Ayar struct {
	SunucuURL string `json:"sunucu_url"`
	Token     string `json:"token"`
	CihazAd   string `json:"cihaz_ad"`
	IsletmeAd string `json:"isletme_ad"`

	// SonBilinenPort/SonBilinenToken — çalışan sürecin yerel durum sunucusunun
	// (127.0.0.1, internal/durum) EN SON bilinen adresi (v0.4.0). Aynı
	// bilgisayarda ikinci bir kopya açılıp tek-kopya kilidini alamazsa buradan
	// POST /odaklan ile çalışan kopyaya "pencereni öne getir" sinyali gönderir
	// (bkz. cmd/hizmetra-kopru main.go: digerKopyayaOdaklanDene). SonBilinenToken
	// YALNIZ bu loopback sunucusuna erişim içindir — Token alanıyla (panel API
	// yetkisi) KARIŞTIRILMAMALI, sunucuya hiçbir yetki taşımaz. Her başlangıçta
	// üzerine yazılır; eski/geçersiz bir değer kalmışsa (önceki kopya port
	// kaydetmeden çökmüş vb.) /odaklan denemesi başarısız olur ve ikinci süreç
	// sessizce çıkar — bu nadir kenar durum için yeniden deneme YOKTUR.
	SonBilinenPort  int    `json:"son_bilinen_port,omitempty"`
	SonBilinenToken string `json:"son_bilinen_token,omitempty"`

	// SonGosterilenSurum — durum penceresinin EN SON hangi sürümde bir kez
	// kendiliğinden gösterildiği (v0.7.0). Kurulumdan/güncellemeden sonra ajan
	// sessizce tepside açılıyordu; kullanıcı "açılmadı" sanıyordu (emre 2026-08-17).
	// Startup'ta bu değer çalışan Surum'dan farklıysa pencere BİR KEZ otomatik
	// açılır ve buraya o sürüm yazılır — böylece her login'de değil, yalnız yeni
	// kurulum/güncelleme sonrası ilk açılışta görünür (bkz. main.go: ilkAcilisGoster).
	SonGosterilenSurum string `json:"son_gosterilen_surum,omitempty"`
}

// VarsayilanSunucu — panel API kökü. HIZMETRA_API env'i ezer (geliştirme).
const VarsayilanSunucu = "https://api.hizmetra.com"

// BilinenSunucular — ilk kurulumda (env/kayıt yoksa) SIRAYLA denenir; kurulum
// kodunu KABUL eden sunucu kullanılır. Böylece TEK exe hem production hem
// staging'e bağlanır: staging'de üretilen kod staging'i, production'da üretilen
// kod production'ı otomatik bulur. Ortam değişkeni / .bat GEREKMEZ.
var BilinenSunucular = []string{
	"https://api.hizmetra.com",           // production (canlı kafeler)
	"https://staging-panel.hizmetra.com", // staging (test)
}

// Dizin — ayar/log dizini; yoksa oluşturur.
func Dizin() (string, error) {
	kok, err := os.UserConfigDir() // Windows: %APPDATA%; yoksa hata
	if err != nil {
		kok, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	d := filepath.Join(kok, "HizmetraKopru")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return "", err
	}
	return d, nil
}

func yol() (string, error) {
	d, err := Dizin()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// Yukle — kayıtlı ayarı okur. Dosya yoksa boş Ayar + nil hata döner
// (ilk çalıştırma = eşleştirme gerekiyor demektir).
func Yukle() (*Ayar, error) {
	p, err := yol()
	if err != nil {
		return nil, err
	}
	ham, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Ayar{}, nil
	}
	if err != nil {
		return nil, err
	}
	var a Ayar
	if err := json.Unmarshal(ham, &a); err != nil {
		// Bozuk config ajanı kilitlemesin — sıfırdan eşleşir.
		return &Ayar{}, nil
	}
	return &a, nil
}

// Kaydet — ayarı diske yazar (0600: yalnız kullanıcı).
func Kaydet(a *Ayar) error {
	p, err := yol()
	if err != nil {
		return err
	}
	ham, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, ham, 0o600)
}

// Sil — config.json'u kaldırır (token dahil her şey). "Onar" (yeniden
// eşleştir) ve "Kaldır" akışları çağırır; dosya zaten yoksa hata DÖNMEZ.
// SunucuURL de gittiği için sonraki eşleştirme bilinen sunucuların hepsini
// yeniden dener (kod hangi paneldeyse oraya bağlanır).
func Sil() error {
	p, err := yol()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SunucuAdresi — env ezmesi > kayıtlı > varsayılan.
func (a *Ayar) SunucuAdresi() string {
	if v := os.Getenv("HIZMETRA_API"); v != "" {
		return v
	}
	if a.SunucuURL != "" {
		return a.SunucuURL
	}
	return VarsayilanSunucu
}

// SunucuAdaylari — eşleştirmede DENENECEK sunucular, öncelik sırasıyla:
//   - env HIZMETRA_API varsa YALNIZ onu (açık geçersiz kılma),
//   - kayıtlı SunucuURL varsa YALNIZ onu (zaten bir sunucuya eşleşmiş),
//   - yoksa BilinenSunucular (production→staging otomatik keşif).
// Eşleşme başarınca kazanan sunucu SunucuURL'e yazılır; sonraki tüm
// nabız/işler tek sunucuya gider.
func (a *Ayar) SunucuAdaylari() []string {
	if v := os.Getenv("HIZMETRA_API"); v != "" {
		return []string{v}
	}
	if a.SunucuURL != "" {
		return []string{a.SunucuURL}
	}
	return BilinenSunucular
}
