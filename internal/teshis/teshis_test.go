package teshis

import (
	"errors"
	"strings"
	"testing"
)

// TestCumleTumKodlar — katalogdaki HER kodun cümlesi kullanıcıya gösterilebilir
// olmalı: dolu, tek satır, en fazla 140 rune ve yasaklı kelime içermeyen.
func TestCumleTumKodlar(t *testing.T) {
	for _, kod := range TumKodlar() {
		c := Cumle(kod, "ZJ-80", "")
		if strings.TrimSpace(c) == "" {
			t.Errorf("%s: cümle boş", kod)
		}
		if n := len([]rune(c)); n > 140 {
			t.Errorf("%s: cümle %d rune (>140): %q", kod, n, c)
		}
		if strings.Contains(c, "\n") || strings.Contains(c, "\r") {
			t.Errorf("%s: cümle satır sonu içeriyor: %q", kod, c)
		}
		yasakliDenetle(t, string(kod), c)
	}
}

// TestCumleYaziciAdiBosken — ad boşken "  yazıcısında" gibi sakat cümle çıkmamalı.
func TestCumleYaziciAdiBosken(t *testing.T) {
	c := Cumle(KAGIT_YOK, "   ", "")
	if !strings.HasPrefix(c, "Yazıcı") {
		t.Errorf("boş adda genel ifade bekleniyordu: %q", c)
	}
}

// TestCumleEkUzunlukKirpar — çok uzun ek 140 rune sınırını AŞMAMALI.
func TestCumleEkUzunlukKirpar(t *testing.T) {
	c := Cumle(BILINMEYEN, "Mutfak", strings.Repeat("çok uzun teknik ayrıntı ", 40))
	if n := len([]rune(c)); n > 140 {
		t.Fatalf("kırpılmadı: %d rune", n)
	}
}

// TestMetinGuvenliTurkceVaryantlar — I/ı tuzağı: yasaklı kelimenin düz, Türkçe
// büyük harfli ve i-ailesi karışık varyantlarının HEPSİ süzülmeli.
// strings.ToLower kullanılsaydı "YAPILANDIR" varyantı kaçardı.
func TestMetinGuvenliTurkceVaryantlar(t *testing.T) {
	varyantlar := []string{
		"yapılandır", "YAPILANDIR", "Yapılandır", "yapilandir", "YAPİLANDİR",
		"bağlan", "BAĞLAN", "Bağlan",
		"printnode", "PRINTNODE", "PrintNode", "PRİNTNODE",
		"bayat", "BAYAT", "Bayat",
	}
	for _, v := range varyantlar {
		cikti := metinGuvenli("Sorun: " + v + " denendi.")
		yasakliDenetle(t, v, cikti)
	}
}

// yasakliDenetle — metinde yasaklı alt-dizgilerin HİÇBİR varyantı olmamalı.
func yasakliDenetle(t *testing.T, baglam, metin string) {
	t.Helper()
	for _, y := range yasakli {
		if bas, _ := normalBul(metin, y.desen); bas >= 0 {
			t.Errorf("%s: yasaklı %q metinde kaldı: %q", baglam, y.desen, metin)
		}
	}
}

// TestIsHataKodu — JOB_STATUS bit kombinasyonları beklenen kodlara eşlenmeli.
func TestIsHataKodu(t *testing.T) {
	durumlar := []struct {
		ad     string
		bitler uint32
		kod    Kod
		hata   bool
	}{
		{"kağıt yok (tek başına)", IsBitiPAPEROUT, KAGIT_YOK, true},
		{"kağıt yok + offline + error", IsBitiPAPEROUT | IsBitiOFFLINE | IsBitiERROR, KAGIT_YOK, true},
		{"kullanıcı müdahalesi", IsBitiUSER_INTERVENTION, KAPAK_ACIK, true},
		{"offline", IsBitiOFFLINE, YAZICI_KAPALI, true},
		{"duraklatıldı", IsBitiPAUSED, KUYRUK_DURAKLATILDI, true},
		{"sürücü basamıyor", IsBitiBLOCKED_DEVQ, VERI_TURU_RED, true},
		{"genel hata", IsBitiERROR, BILINMEYEN, true},
		{"basıldı", IsBitiPRINTED, "", false},
		{"tamamlandı", IsBitiCOMPLETE, "", false},
		{"spooling", IsBitiSPOOLING, "", false},
		{"yazdırılıyor", IsBitiPRINTING, "", false},
		{"boş", 0, "", false},
	}
	for _, d := range durumlar {
		kod, hata := IsHataKodu(d.bitler)
		if hata != d.hata || kod != d.kod {
			t.Errorf("%s: (%q,%v) geldi, (%q,%v) bekleniyordu", d.ad, kod, hata, d.kod, d.hata)
		}
	}
}

// TestTeslimKarari — teslim teyidi karar tablosu.
func TestTeslimKarari(t *testing.T) {
	// Listede yok + liste dolu DEĞİL → başarılı.
	if k, _ := TeslimKarari(false, 0, false); k != KararBasarili {
		t.Errorf("listede yok → başarılı bekleniyordu, %v geldi", k)
	}
	// Listede yok AMA liste 255'e dayandı → BELİRSİZ (yanlış-pozitif kapısı).
	k, kod := TeslimKarari(false, 0, true)
	if k != KararBelirsiz || kod != KUYRUK_SISTI {
		t.Errorf("kuyruk şiştiğinde belirsiz+KUYRUK_SISTI bekleniyordu, (%v,%q) geldi", k, kod)
	}
	// Listede var, PRINTED → başarılı.
	if k, _ := TeslimKarari(true, IsBitiPRINTED, false); k != KararBasarili {
		t.Errorf("PRINTED → başarılı bekleniyordu, %v geldi", k)
	}
	// Listede var, kağıt yok → gerçek hata.
	if k, kod := TeslimKarari(true, IsBitiPAPEROUT, false); k != KararHata || kod != KAGIT_YOK {
		t.Errorf("PAPEROUT → hata+KAGIT_YOK bekleniyordu, (%v,%q) geldi", k, kod)
	}
	// Listede var, hâlâ spool ediliyor → bekliyor.
	if k, _ := TeslimKarari(true, IsBitiSPOOLING, false); k != KararBekliyor {
		t.Errorf("SPOOLING → bekliyor bekleniyordu, %v geldi", k)
	}
}

// TestHataKoduTasima — kodlu hata sarmalandıktan sonra da okunabilmeli.
func TestHataKoduTasima(t *testing.T) {
	h := Yeni(KAGIT_YOK, "ZJ-80", "", errors.New("ham"))
	if KodunuAl(h) != KAGIT_YOK {
		t.Fatalf("kod taşınmadı: %v", KodunuAl(h))
	}
	sarmal := errors.New("dış: " + h.Error())
	if KodunuAl(sarmal) != BILINMEYEN {
		t.Fatalf("kodsuz hata BILINMEYEN olmalı")
	}
	if KodunuAl(nil) != "" {
		t.Fatalf("nil hatada kod boş olmalı")
	}
}

// TestBulguAlEylem — onarılabilir kodlarda ONAR düğmesi, yazıcı seçimi
// gerektirenlerde YAZICI_SEC gösterilmeli.
func TestBulguAlEylem(t *testing.T) {
	if b := BulguAl(KUYRUK_DURAKLATILDI, "Mutfak", ""); b.Eylem != EylemOnar || !b.Onarilabilir {
		t.Errorf("duraklatılmış kuyruk onarılabilir olmalı: %+v", b)
	}
	if b := BulguAl(SANAL_HEDEF, "Microsoft XPS", ""); b.Eylem != EylemYaziciSec {
		t.Errorf("sanal hedefte yazıcı seçimi istenmeli: %+v", b)
	}
	if b := BulguAl(Kod("YOK_BOYLE"), "X", ""); b.Kod != BILINMEYEN {
		t.Errorf("bilinmeyen kod BILINMEYEN'e düşmeli: %+v", b)
	}
}
