package api

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"errors"
	"net/http"
	"sync"
	"time"
)

// ─────────────────── ESKİ WINDOWS TLS KURTARMASI (2026-08-21) ───────────────────
//
// SORUN: Go'nun crypto/tls'i SAF GO'dur (Windows 7'de bile TLS 1.2/1.3 konuşur),
// AMA sertifika zinciri doğrulaması Windows'un KÖK DEPOSUNA gider
// (crypto/x509 root_windows.go → CertGetCertificateChain). Sunucularımız
// (api.hizmetra.com / staging-panel.hizmetra.com) Let's Encrypt'in YENİ ECDSA
// hiyerarşisini sunuyor:
//     yaprak ← YE2 ← Root YE ← ISRG Root X2
// Güven çıpası ISRG Root X2 (ya da onu çapraz imzalayan ISRG Root X1) olmak
// zorunda. Bu kökler Windows 7'nin FABRİKA deposunda YOKTUR; Windows normalde
// eksik kökü Windows Update'ten (ctldl.windowsupdate.com) anlık indirir — ama
// otomatik kök güncellemesi grup ilkesiyle kapatılmış, makine uzun süredir
// güncellenmemiş ya da güvenlik duvarı o adresi engelliyorsa İNDİREMEZ.
// O zaman ajan sunucuya HİÇ bağlanamaz: eski-Windows desteğinin tamamı çöker
// ve kullanıcı yalnızca anlaşılmaz bir "bağlantı" hatası görür.
//
// ÇÖZÜM: Varsayılan yol DEĞİŞMEZ — önce her zaman işletim sisteminin kendi
// doğrulayıcısı kullanılır (kurumsal proxy kökleri, iptal listeleri, Windows'un
// anlık kök indirmesi aynen çalışsın diye). YALNIZCA sertifika doğrulaması
// başarısız olursa istek BİR KEZ, gömülü ISRG kökleriyle kurulmuş yedek bir
// havuzla tekrarlanır. Yani bu bir "doğrulamayı atlama" değildir: hâlâ tam
// zincir doğrulaması yapılır, sadece güven çıpası ikili içinden gelir.
//
// GÜVENLİK NOTU: InsecureSkipVerify HİÇBİR YERDE kullanılmaz. Gömülü kökler
// yalnızca EK güven çıpasıdır; geçersiz/süresi dolmuş/adı uyuşmayan bir
// sertifika yine reddedilir.
//
// BAKIM: kokler.pem içindeki kökler 2035 (X1) ve 2040 (X2) sonuna kadar
// geçerlidir. Sunucu sertifika sağlayıcısı değişirse bu dosya güncellenmelidir.

//go:embed kokler.pem
var gomuluKokler []byte

var (
	yedekHavuzBir sync.Once
	yedekHavuz    *x509.CertPool
)

// yedekKokHavuzu — sistem havuzu (varsa) + gömülü ISRG kökleri.
// Sistem havuzu okunamazsa (eski Windows'ta olabilir) yalnız gömülü kökler
// kullanılır; bu yine tam bir zincir doğrulamasıdır.
func yedekKokHavuzu() *x509.CertPool {
	yedekHavuzBir.Do(func() {
		havuz, err := x509.SystemCertPool()
		if err != nil || havuz == nil {
			havuz = x509.NewCertPool()
		}
		havuz.AppendCertsFromPEM(gomuluKokler)
		yedekHavuz = havuz
	})
	return yedekHavuz
}

// sertifikaHatasiMi — hata, sertifika DOĞRULAMA hatası mı? (Ağ kopması, DNS ya
// da zaman aşımı için yedek yol denenmez — onlarda kök sertifikanın suçu yoktur.)
func sertifikaHatasiMi(err error) bool {
	if err == nil {
		return false
	}
	var dogrulama *tls.CertificateVerificationError
	if errors.As(err, &dogrulama) {
		return true
	}
	var bilinmeyenYetkili x509.UnknownAuthorityError
	if errors.As(err, &bilinmeyenYetkili) {
		return true
	}
	var gecersiz x509.CertificateInvalidError
	return errors.As(err, &gecersiz)
}

// yedekIstemci — gömülü köklerle doğrulayan HTTP istemcisi (istek başına timeout).
func yedekIstemci(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	tasima := http.DefaultTransport.(*http.Transport).Clone()
	tasima.TLSClientConfig = &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    yedekKokHavuzu(),
	}
	return &http.Client{Timeout: timeout, Transport: tasima}
}
