package main

import (
	"os"
	"path/filepath"
	"testing"
)

// izoleKurulumDizini — os.UserCacheDir'i geçici dizine yönlendirir (Windows:
// LOCALAPPDATA, Linux: XDG_CACHE_HOME, macOS: HOME). Testler gerçek
// %LOCALAPPDATA%\HizmetraKopru'ya DOKUNMAZ.
func izoleKurulumDizini(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("LOCALAPPDATA", d)
	t.Setenv("XDG_CACHE_HOME", d)
	t.Setenv("HOME", d)
	t.Setenv(yerindeEnv, "")
	return d
}

// TestKopyaAdiMi — çalışan kopya tespiti: kurulu ad + tarayıcı yinelenen indirme
// adları ("hizmetra-kopru (3).exe" — emre'nin makinesindeki gerçek durum) eşleşir;
// test ikilisi ve alakasız süreçler eşleşmez.
func TestKopyaAdiMi(t *testing.T) {
	for _, ad := range []string{
		"hizmetra-kopru.exe", "HIZMETRA-KOPRU.EXE", "hizmetra-kopru (2).exe",
		"hizmetra-kopru (13).exe", "hizmetra-kopru(1).exe",
	} {
		if !kopyaAdiMi(ad) {
			t.Errorf("%q kopya olarak tanınmalı", ad)
		}
	}
	for _, ad := range []string{
		"hizmetra-kopru.test.exe", "hizmetra-kopru", "hizmetra-kopru.exe.yeni",
		"hizmetra-kopruX.exe", "hizmetra.exe", "explorer.exe", "hizmetra-kopru (2).exe.bak",
	} {
		if kopyaAdiMi(ad) {
			t.Errorf("%q kopya sayılmamalı", ad)
		}
	}
}

// TestKuruluYol — kurulu konum %LOCALAPPDATA%\HizmetraKopru\hizmetra-kopru.exe.
func TestKuruluYol(t *testing.T) {
	d := izoleKurulumDizini(t)
	yol, err := kuruluYol()
	if err != nil {
		t.Fatal(err)
	}
	if beklenen := filepath.Join(d, kurulumKlasor, exeAdi); yol != beklenen {
		t.Fatalf("kurulu yol %q, beklenen %q", yol, beklenen)
	}
}

// TestKuruluKonumaKopyalaBaskaYoldan — başka yoldan (İndirilenler) çalışırken:
// exe kurulu konuma kopyalanır (bayt bayt aynı), kopyalandi=true, hedef kurulu yol.
// İkinci çağrı da (yine başka yoldayız) üzerine yazar — Güncelle idempotent.
func TestKuruluKonumaKopyalaBaskaYoldan(t *testing.T) {
	d := izoleKurulumDizini(t)
	kendi, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable yok:", err)
	}
	for i := 0; i < 2; i++ {
		hedef, kopyalandi, err := kuruluKonumaKopyala()
		if err != nil {
			t.Fatalf("tur %d: %v", i, err)
		}
		if !kopyalandi {
			t.Fatalf("tur %d: başka yoldan çalışırken kopyalanmalı", i)
		}
		if hedef != filepath.Join(d, kurulumKlasor, exeAdi) {
			t.Fatalf("tur %d: hedef %q", i, hedef)
		}
		a, _ := os.Stat(kendi)
		b, err := os.Stat(hedef)
		if err != nil {
			t.Fatalf("tur %d: hedef yok: %v", i, err)
		}
		if a.Size() != b.Size() {
			t.Fatalf("tur %d: boyut farklı %d≠%d", i, a.Size(), b.Size())
		}
		if _, err := os.Stat(hedef + ".yeni"); !os.IsNotExist(err) {
			t.Fatalf("tur %d: geçici .yeni dosyası kalmamalı", i)
		}
	}
}

// TestKuruluKonumaKopyalaYerinde — HIZMETRA_YERINDE=1 (geliştirme): kopyalama
// yok, hedef = kendi yolu (autostart yine kurulur ama devretme olmaz).
func TestKuruluKonumaKopyalaYerinde(t *testing.T) {
	d := izoleKurulumDizini(t)
	t.Setenv(yerindeEnv, "1")
	kendi, _ := os.Executable()
	hedef, kopyalandi, err := kuruluKonumaKopyala()
	if err != nil {
		t.Fatal(err)
	}
	if kopyalandi || hedef != kendi {
		t.Fatalf("yerinde modda kopyalanmamalı: hedef=%q kopyalandi=%v", hedef, kopyalandi)
	}
	if _, err := os.Stat(filepath.Join(d, kurulumKlasor, exeAdi)); !os.IsNotExist(err) {
		t.Fatal("yerinde modda kurulu konuma dosya yazılmamalı")
	}
}

// TestAyniDosya — aynı yol iki kez → true; farklı dosya / olmayan yol → false.
func TestAyniDosya(t *testing.T) {
	d := t.TempDir()
	a := filepath.Join(d, "a.txt")
	b := filepath.Join(d, "b.txt")
	_ = os.WriteFile(a, []byte("x"), 0o600)
	_ = os.WriteFile(b, []byte("x"), 0o600)
	if !ayniDosya(a, a) {
		t.Error("aynı yol aynı dosya olmalı")
	}
	if ayniDosya(a, b) {
		t.Error("farklı dosyalar aynı sayılmamalı")
	}
	if ayniDosya(a, filepath.Join(d, "yok.txt")) {
		t.Error("olmayan dosya aynı sayılmamalı")
	}
}

// TestDosyaKopyalaUzerineYazar — var olan hedefin üzerine yazar; .yeni artığı kalmaz.
func TestDosyaKopyalaUzerineYazar(t *testing.T) {
	d := t.TempDir()
	src := filepath.Join(d, "src.bin")
	dst := filepath.Join(d, "alt", "dst.bin")
	_ = os.WriteFile(src, []byte("yeni-icerik"), 0o600)
	_ = os.MkdirAll(filepath.Dir(dst), 0o700)
	_ = os.WriteFile(dst, []byte("eski"), 0o600)
	if err := dosyaKopyala(src, dst); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "yeni-icerik" {
		t.Fatalf("hedef içeriği %q", got)
	}
	if _, err := os.Stat(dst + ".yeni"); !os.IsNotExist(err) {
		t.Fatal(".yeni geçici dosyası kalmamalı")
	}
}

// TestKuruluKopyayiSilBaskaYoldan — kurulu exe biz değilsek silinir, klasör de
// (boşsa) gider; dosya yoksa da true (idempotent).
func TestKuruluKopyayiSilBaskaYoldan(t *testing.T) {
	d := izoleKurulumDizini(t)
	hedef := filepath.Join(d, kurulumKlasor, exeAdi)
	_ = os.MkdirAll(filepath.Dir(hedef), 0o700)
	_ = os.WriteFile(hedef, []byte("exe"), 0o755)
	if !kuruluKopyayiSil() {
		t.Fatal("kurulu exe silinmeli")
	}
	if _, err := os.Stat(hedef); !os.IsNotExist(err) {
		t.Fatal("kurulu exe silinmedi")
	}
	if !kuruluKopyayiSil() {
		t.Fatal("dosya yokken de başarılı sayılmalı")
	}
}

// TestBayrakVar — --durum-ac bayrağı os.Args'tan okunur.
func TestBayrakVar(t *testing.T) {
	eski := os.Args
	defer func() { os.Args = eski }()
	os.Args = []string{"hizmetra-kopru.exe", durumAcArg}
	if !bayrakVar(durumAcArg) {
		t.Fatal("bayrak bulunmalı")
	}
	os.Args = []string{"hizmetra-kopru.exe"}
	if bayrakVar(durumAcArg) {
		t.Fatal("bayrak yokken bulunmamalı")
	}
}
