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
}

// VarsayilanSunucu — panel API kökü. HIZMETRA_API env'i ezer (geliştirme).
const VarsayilanSunucu = "https://api.hizmetra.com"

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
