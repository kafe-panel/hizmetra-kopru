package api

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"testing"
)

// Gömülü kökler GERÇEKTEN ayrıştırılabiliyor mu? Bozuk/boş bir PEM sessizce
// "yedek havuz var ama içi boş" durumuna yol açar ve eski Windows kurtarması
// çalışmaz — bu test onu yakalar.
func TestGomuluKoklerAyristirilabiliyor(t *testing.T) {
	kalan := gomuluKokler
	var adlar []string
	for {
		var blok *pem.Block
		blok, kalan = pem.Decode(kalan)
		if blok == nil {
			break
		}
		sertifika, err := x509.ParseCertificate(blok.Bytes)
		if err != nil {
			t.Fatalf("gömülü kök ayrıştırılamadı: %v", err)
		}
		if !sertifika.IsCA {
			t.Fatalf("gömülü sertifika CA değil: %s", sertifika.Subject.CommonName)
		}
		adlar = append(adlar, sertifika.Subject.CommonName)
	}
	// Sunucularımız Let's Encrypt'in ECDSA zincirini sunuyor; çıpa X2 (ya da onu
	// çapraz imzalayan X1) olmak zorunda — İKİSİ de gömülü olmalı.
	istenen := map[string]bool{"ISRG Root X1": false, "ISRG Root X2": false}
	for _, ad := range adlar {
		if _, ok := istenen[ad]; ok {
			istenen[ad] = true
		}
	}
	for ad, bulundu := range istenen {
		if !bulundu {
			t.Errorf("gömülü köklerde %q YOK (bulunanlar: %v)", ad, adlar)
		}
	}
}

// Yedek havuz gömülü kökleri içermeli (sistem havuzu okunamasa bile).
func TestYedekKokHavuzuDolu(t *testing.T) {
	if havuz := yedekKokHavuzu(); havuz == nil || len(havuz.Subjects()) == 0 { //nolint:staticcheck // Subjects() testte yeterli
		t.Fatal("yedek kök havuzu boş")
	}
}

// Yedek TLS yolu YALNIZ sertifika hatalarında denenmeli. Ağ kopması/zaman aşımı
// gibi hatalarda denenirse her geçici arıza iki kat gecikme yaratır.
func TestSertifikaHatasiSiniflandirmasi(t *testing.T) {
	if !sertifikaHatasiMi(x509.UnknownAuthorityError{}) {
		t.Error("UnknownAuthorityError sertifika hatası sayılmalı")
	}
	if !sertifikaHatasiMi(x509.CertificateInvalidError{Reason: x509.Expired}) {
		t.Error("CertificateInvalidError sertifika hatası sayılmalı")
	}
	// Sarmalanmış hata da tanınmalı (http.Client hataları sarmalar).
	if !sertifikaHatasiMi(errors.Join(errors.New("get"), x509.UnknownAuthorityError{})) {
		t.Error("sarmalanmış sertifika hatası tanınmalı")
	}
	if sertifikaHatasiMi(errors.New("bağlantı kapandı")) {
		t.Error("düz hata sertifika hatası SAYILMAMALI")
	}
	if sertifikaHatasiMi(&net.OpError{Op: "dial", Err: errors.New("timeout")}) {
		t.Error("ağ hatası sertifika hatası SAYILMAMALI")
	}
	if sertifikaHatasiMi(nil) {
		t.Error("nil sertifika hatası SAYILMAMALI")
	}
}
