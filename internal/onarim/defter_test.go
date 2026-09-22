package onarim

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDevreKesici — aynı hedefte aynı işlem saatte 3 kez; 4.'de izin YOK.
func TestDevreKesici(t *testing.T) {
	d := Ac(t.TempDir())
	for i := 0; i < SaattekiUstSinir; i++ {
		if !d.Izin("ZJ-80", "duraklatmayi-kaldir") {
			t.Fatalf("%d. denemede izin verilmeliydi", i+1)
		}
		if _, err := d.Yaz("ZJ-80", "duraklatmayi-kaldir", "duraklatilmis", "calisiyor"); err != nil {
			t.Fatalf("yazılamadı: %v", err)
		}
	}
	if d.Izin("ZJ-80", "duraklatmayi-kaldir") {
		t.Fatal("4. denemede devre kesici devreye girmeliydi")
	}
	// Başka bir işlem/hedef ETKİLENMEMELİ.
	if !d.Izin("ZJ-80", "cevrimdisi-bayragi") {
		t.Error("farklı işlem devre kesiciden etkilenmemeli")
	}
	if !d.Izin("Mutfak", "duraklatmayi-kaldir") {
		t.Error("farklı hedef devre kesiciden etkilenmemeli")
	}
}

// TestGeriAl — kayıt pasife çekilir ve ESKİ değer döner.
func TestGeriAl(t *testing.T) {
	d := Ac(t.TempDir())
	id, err := d.Yaz("Mutfak", "cevrimdisi-bayragi", "0x400-acik", "0x400-kapali")
	if err != nil {
		t.Fatalf("yazılamadı: %v", err)
	}
	if k, varmi := d.Sonuncu(); !varmi || k.ID != id {
		t.Fatalf("son aktif kayıt bulunamadı: %+v", k)
	}
	eski, tamam := d.GeriAl(id)
	if !tamam || eski != "0x400-acik" {
		t.Fatalf("geri alma yanlış: (%q,%v)", eski, tamam)
	}
	if _, tekrar := d.GeriAl(id); tekrar {
		t.Error("aynı kayıt iki kez geri alınmamalı")
	}
	if _, varmi := d.Sonuncu(); varmi {
		t.Error("geri alınan kayıt aktif görünmemeli")
	}
}

// TestDiskeYazipOkuma — yeni bir Defter örneği kayıtları diskten görmeli.
func TestDiskeYazipOkuma(t *testing.T) {
	dizin := t.TempDir()
	d := Ac(dizin)
	if _, err := d.Yaz("ZJ-80", "duraklatmayi-kaldir", "a", "b"); err != nil {
		t.Fatalf("yazılamadı: %v", err)
	}
	d2 := Ac(dizin)
	if d2.Sayi() != 1 {
		t.Fatalf("diskten 1 kayıt bekleniyordu, %d geldi", d2.Sayi())
	}
	if k, varmi := d2.Sonuncu(); !varmi || k.Hedef != "ZJ-80" || k.EskiDeger != "a" {
		t.Fatalf("kayıt bozuldu: %+v", k)
	}
}

// TestBozukDosyaPanikYapmaz — yarım/bozuk JSON'da boş defterle devam edilmeli.
func TestBozukDosyaPanikYapmaz(t *testing.T) {
	dizin := t.TempDir()
	if err := os.WriteFile(filepath.Join(dizin, "onarim.json"), []byte("{yarim"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := Ac(dizin)
	if d.Sayi() != 0 {
		t.Fatalf("bozuk dosyada boş defter bekleniyordu, %d kayıt geldi", d.Sayi())
	}
	if !d.Izin("X", "y") {
		t.Error("boş defterde izin verilmeli")
	}
}

// TestAtomikYazim — yazımdan sonra geçici dosya KALMAMALI.
func TestAtomikYazim(t *testing.T) {
	dizin := t.TempDir()
	d := Ac(dizin)
	if _, err := d.Yaz("X", "y", "1", "2"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dizin, "onarim.json.tmp")); !os.IsNotExist(err) {
		t.Error("geçici dosya silinmemiş (atomik yazım bozuk)")
	}
	if _, err := os.Stat(filepath.Join(dizin, "onarim.json")); err != nil {
		t.Errorf("defter dosyası yok: %v", err)
	}
}

// TestNilDefterGuvenli — defter kurulamadıysa (dizin yok) çağrılar panik
// yerine "izin yok" demeli: onarım yapılmaz, ajan çalışmaya devam eder.
func TestNilDefterGuvenli(t *testing.T) {
	var d *Defter
	if d.Izin("X", "y") {
		t.Error("nil defterde izin verilmemeli")
	}
	if _, err := d.Yaz("X", "y", "a", "b"); err == nil {
		t.Error("nil deftere yazım hata dönmeli")
	}
	if _, varmi := d.Sonuncu(); varmi {
		t.Error("nil defterde kayıt olmamalı")
	}
	if d.Sayi() != 0 {
		t.Error("nil defterde sayı 0 olmalı")
	}
}
