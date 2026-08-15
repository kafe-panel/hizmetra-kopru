package main

// Kurulu kopya yönetimi (v0.3.0): exe nereden açılırsa açılsın (İndirilenler'deki
// "hizmetra-kopru (3).exe" dahil) TEK bir kurulu konumdan çalışır:
//
//	%LOCALAPPDATA%\HizmetraKopru\hizmetra-kopru.exe
//
// Run anahtarı DAİMA bu yola yazılır. Eskiden os.Executable() (İndirilenler yolu)
// yazılıyordu; kullanıcı indirdiği dosyayı silince otomatik başlatma kırılıyordu
// (emre 2026-08-16, madde 2). Bu dosya platform-bağımsızdır (testler Linux CI'da
// da koşar); mutex/registry/süreç-kapatma platform_windows.go'dadır.

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

const (
	exeAdi        = "hizmetra-kopru.exe" // kurulu kopyanın dosya adı (teknik kimlik — DEĞİŞMEZ)
	kurulumKlasor = "HizmetraKopru"      // %LOCALAPPDATA% altındaki klasör
	// devredildiEnv — kurulu kopyaya devredilirken çocuğa geçer; çocuk BİR DAHA
	// devretmeye kalkmaz (kopyalama tespiti şaşsa bile sonsuz döngü olmasın).
	devredildiEnv = "HIZMETRA_DEVREDILDI"
	// yerindeEnv — "1" ise kurulu konuma kopyalama/devretme YAPILMAZ (geliştirme:
	// dist/ altındaki derlemeyi olduğu yerden çalıştırmak için).
	yerindeEnv = "HIZMETRA_YERINDE"
	// durumAcArg — devralan kopya açılınca durum penceresini tarayıcıda açsın
	// (kullanıcı exe'ye tıkladı, geri bildirim bekliyor; otomatik başlatmada verilmez).
	durumAcArg = "--durum-ac"
)

// kopyaAdiDeseni — çalışan kopyanın süreç adı: kurulu ad ya da tarayıcının
// yinelenen indirme adı ("hizmetra-kopru (2).exe", "hizmetra-kopru(3).exe").
// Büyük/küçük harf duyarsız. Test ikilisi (hizmetra-kopru.test.exe) EŞLEŞMEZ.
var kopyaAdiDeseni = regexp.MustCompile(`(?i)^hizmetra-kopru(?: ?\(\d+\))?\.exe$`)

// kopyaAdiMi — süreç adı bu programın bir kopyası mı?
func kopyaAdiMi(ad string) bool { return kopyaAdiDeseni.MatchString(ad) }

// kuruluYol — kurulu kopyanın tam yolu (%LOCALAPPDATA%\HizmetraKopru\hizmetra-kopru.exe).
// os.UserCacheDir: Windows'ta %LocalAppData% (kullanıcıya özel, gezinmeyen).
func kuruluYol() (string, error) {
	kok, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(kok, kurulumKlasor, exeAdi), nil
}

// ayniDosya — iki yol aynı dosyaya mı çıkıyor (büyük/küçük harf, 8.3 ad vb.
// farklarına dayanıklı: dosya kimliğinden karşılaştırır). Biri yoksa false.
func ayniDosya(a, b string) bool {
	fa, err := os.Stat(a)
	if err != nil {
		return false
	}
	fb, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(fa, fb)
}

// dosyaKopyala — kaynağı hedefe kopyalar: önce hedef+".yeni"ye yazar, sonra
// üzerine taşır (yarım kopya kalmasın). Hedef çalışan bir exe ise taşıma
// başarısız olur (Windows kilidi) — çağıran önce çalışan kopyayı kapatır.
func dosyaKopyala(kaynak, hedef string) error {
	if err := os.MkdirAll(filepath.Dir(hedef), 0o700); err != nil {
		return err
	}
	src, err := os.Open(kaynak)
	if err != nil {
		return err
	}
	defer src.Close()
	gecici := hedef + ".yeni"
	dst, err := os.OpenFile(gecici, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(gecici)
		return err
	}
	if err := dst.Sync(); err != nil {
		dst.Close()
		_ = os.Remove(gecici)
		return err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(gecici)
		return err
	}
	if err := os.Rename(gecici, hedef); err != nil {
		_ = os.Remove(gecici)
		return err
	}
	return nil
}

// kuruluKonumaKopyala — çalışan exe'yi kurulu konuma kopyalar. Zaten oradan
// çalışıyorsa (ya da HIZMETRA_YERINDE=1) kopyalamaz. Döner: hedef yol (Run
// anahtarına yazılacak yol), kopyalandı mı, hata. Hata halinde hedef = kendi
// yolumuz (autostart yine de kurulsun diye).
func kuruluKonumaKopyala() (string, bool, error) {
	kendi, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if os.Getenv(yerindeEnv) == "1" {
		return kendi, false, nil
	}
	hedef, err := kuruluYol()
	if err != nil {
		return kendi, false, err
	}
	if ayniDosya(kendi, hedef) {
		return hedef, false, nil
	}
	if err := dosyaKopyala(kendi, hedef); err != nil {
		return kendi, false, err
	}
	return hedef, true, nil
}

// kuruluKopyayaKur — kopyala + Run anahtarını KOPYALANAN yola yaz. Kopyalama
// başaramazsa autostart mevcut yola kurulur (v0.2.0 davranışı) ve loglanır.
// Döner: kurulu yol, bu süreç kurulu kopya DEĞİLSE (devretmek gerekir) true.
func kuruluKopyayaKur() (string, bool) {
	hedef, kopyalandi, err := kuruluKonumaKopyala()
	if err != nil {
		gunluk.Yaz("kurulu konuma kopyalanamadı (%v); bu yoldan devam: %s", err, hedef)
	} else if kopyalandi {
		gunluk.Yaz("kurulu konuma kopyalandı: %s", hedef)
	}
	if hedef != "" {
		if err := otomatikBaslatKur(hedef); err != nil {
			gunluk.Yaz("otomatik başlatma kurulamadı: %v", err)
		}
	}
	return hedef, kopyalandi
}

// kuruluKopyayiBaslat — tek-kopya kilidini bırakır ve kurulu kopyayı başlatır.
// true → çocuk çalışıyor, BU SÜREÇ ÇIKMALI. Başlatılamazsa kilidi geri alır ve
// false döner (ajan bu yoldan çalışmaya devam eder — hiç çalışmamaktan iyidir).
func kuruluKopyayiBaslat(yol string, arg ...string) bool {
	if os.Getenv(devredildiEnv) == "1" {
		gunluk.Yaz("devredilmiş kopya yeniden devretmez; buradan devam")
		return false
	}
	tekKopyaKilidiBirak()
	if err := yeniKopyayiBaslat(yol, arg...); err != nil {
		gunluk.Yaz("kurulu kopya başlatılamadı (%v); buradan devam", err)
		if !tekKopyaKilidi() {
			gunluk.Yaz("kilit geri alınamadı — başka kopya başlamış olabilir")
		}
		return false
	}
	gunluk.Yaz("kurulu kopya başlatıldı: %s", yol)
	return true
}

// kuruluKopyayaDevret — kur (kopya + Run anahtarı) ve gerekiyorsa devret.
// true → bu süreç çıkmalı. Zaten kurulu yoldaysak false (buradan devam).
func kuruluKopyayaDevret(arg ...string) bool {
	hedef, kopyalandi := kuruluKopyayaKur()
	return kopyalandi && kuruluKopyayiBaslat(hedef, arg...)
}

// yeniKopyayiBaslat — yolu ayrık süreç olarak başlatır (beklemez). Çocuğa
// HIZMETRA_DEVREDILDI=1 geçer.
func yeniKopyayiBaslat(yol string, arg ...string) error {
	cmd := exec.Command(yol, arg...)
	cmd.Env = append(os.Environ(), devredildiEnv+"=1")
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// kuruluKopyayiSil — "Kaldır": kurulu exe'yi siler (kendimiz DEĞİLSE — Windows
// çalışan exe'yi silemez; o durumda kullanıcıya klasör söylenir). Best-effort.
func kuruluKopyayiSil() (silindi bool) {
	hedef, err := kuruluYol()
	if err != nil {
		return false
	}
	kendi, _ := os.Executable()
	if ayniDosya(kendi, hedef) {
		return false
	}
	if err := os.Remove(hedef); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false
	}
	_ = os.Remove(filepath.Dir(hedef)) // boşsa klasör de gitsin
	return true
}

// bayrakVar — komut satırında verilen bayrak var mı (flag paketi kullanılmaz:
// tek bayrak, systray/zenity argümanlarına karışmasın).
func bayrakVar(ad string) bool {
	for _, a := range os.Args[1:] {
		if a == ad {
			return true
		}
	}
	return false
}
