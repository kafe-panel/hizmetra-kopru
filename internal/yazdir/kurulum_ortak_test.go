package yazdir

import "testing"

func TestKurulabilirleriBulEksikListedeSusar(t *testing.T) {
	canli := map[string]string{"USB001": "ZiJiangZJ-80D6E4"}
	if got := KurulabilirleriBul(canli, false, nil); got != nil {
		t.Fatalf("liste EKSİK iken hiçbir şey önerilmemeli, gelen: %+v", got)
	}
	if got := KurulabilirleriBul(nil, true, nil); got != nil {
		t.Fatalf("canlı port yokken hiçbir şey önerilmemeli, gelen: %+v", got)
	}
}

func TestKurulabilirleriBulKuyruguOlaniAtlar(t *testing.T) {
	canli := map[string]string{
		"USB001": "ZiJiangZJ-80D6E4",
		"USB002": "ZiJiangZJ-80A1B2",
	}
	kurulu := map[string]YaziciDurumu{
		"Kasa": {Ad: "Kasa", Port: "usb001"}, // harf duyarsız eşleşmeli
	}
	got := KurulabilirleriBul(canli, true, kurulu)
	if len(got) != 1 {
		t.Fatalf("1 kurulabilir bekleniyordu, %d geldi: %+v", len(got), got)
	}
	if got[0].Port != "USB002" {
		t.Errorf("USB002 bekleniyordu, %q geldi", got[0].Port)
	}
}

func TestKurulabilirleriBulSiraliVeAdlariBenzersiz(t *testing.T) {
	canli := map[string]string{
		"USB003": "ZiJiangZJ-80D6E4",
		"USB001": "ZiJiangZJ-80A1B2",
		"USB002": "ZiJiangZJ-80FFFF",
	}
	got := KurulabilirleriBul(canli, true, map[string]YaziciDurumu{})
	if len(got) != 3 {
		t.Fatalf("3 bekleniyordu, %d geldi", len(got))
	}
	if got[0].Port != "USB001" || got[1].Port != "USB002" || got[2].Port != "USB003" {
		t.Fatalf("port sırası deterministik değil: %+v", got)
	}
	// Üçü de aynı model → adlar ÇAKIŞMAMALI (kullanıcı ayırt edebilsin).
	gorulen := map[string]bool{}
	for _, k := range got {
		if gorulen[k.OnerilenAd] {
			t.Fatalf("ad tekrar etti: %q (%+v)", k.OnerilenAd, got)
		}
		gorulen[k.OnerilenAd] = true
	}
}

func TestKurulabilirleriBulMevcutAdlaCakismaz(t *testing.T) {
	canli := map[string]string{"USB002": "ZiJiangZJ-80D6E4"}
	kurulu := map[string]YaziciDurumu{
		"ZiJiangZJ-80": {Ad: "ZiJiangZJ-80", Port: "USB001"},
	}
	got := KurulabilirleriBul(canli, true, kurulu)
	if len(got) != 1 {
		t.Fatalf("1 bekleniyordu, %d geldi", len(got))
	}
	if got[0].OnerilenAd == "ZiJiangZJ-80" {
		t.Errorf("var olan kuyruk adıyla çakıştı: %q", got[0].OnerilenAd)
	}
}

func TestDonanimdanAd(t *testing.T) {
	durumlar := []struct{ girdi, bekler string }{
		{"ZiJiangZJ-80D6E4", "ZiJiangZJ-80"}, // sondaki sağlama atılır
		{"Zjiang_POS-80", "Zjiang POS-80"},   // alt çizgi boşluk olur
		{"EPSONTM-T20II9B5E", "EPSONTM-T20II"},
		{"POS-80", "POS-80"},     // kısa model KIRPILMAZ
		{"POS-8000", "POS-8000"}, // saf rakamlı model numarası sağlama sanılmaz
		{"Zjiang POS-5890K", "Zjiang POS-5890K"},
		{"ABCD", "ABCD"},     // kesilince 3 karakter kalmıyor → dokunma
		{"", "Fiş Yazıcısı"}, // tanınmayan → genel ad
		{"   ", "Fiş Yazıcısı"},
		{`Kötü\Ad,1`, "Kötü Ad 1"}, // Windows'ta yasak karakterler temizlenir
	}
	for _, d := range durumlar {
		if got := DonanimdanAd(d.girdi); got != d.bekler {
			t.Errorf("DonanimdanAd(%q) = %q, beklenen %q", d.girdi, got, d.bekler)
		}
	}
}

func TestDonanimdanAdHepGecerliAdUretir(t *testing.T) {
	girdiler := []string{"", "!!!", `\\\`, ",,,", "ZiJiangZJ-80D6E4",
		"cokcokcokcokcokcokcokcokcokcokcokcokcokcokcokcokcokuzunbirdonanimkimligi"}
	for _, g := range girdiler {
		if ad := DonanimdanAd(g); !AdGecerliMi(ad) {
			t.Errorf("DonanimdanAd(%q) geçersiz ad üretti: %q", g, ad)
		}
	}
}

func TestAdGecerliMi(t *testing.T) {
	gecerli := []string{"ZJ-80", "Kasa", "Mutfak 2", "Fiş Yazıcısı"}
	for _, a := range gecerli {
		if !AdGecerliMi(a) {
			t.Errorf("%q geçerli olmalıydı", a)
		}
	}
	gecersiz := []string{"", "   ", `a\b`, "a,b", "a!b", "a\x00b", "a\nb"}
	for _, a := range gecersiz {
		if AdGecerliMi(a) {
			t.Errorf("%q geçersiz olmalıydı", a)
		}
	}
}

func TestKucukHarfTurkceTuzagi(t *testing.T) {
	// strings.ToLower Türkçe yerelinde 'I' → 'ı' yapabilir; bizimki YAPMAMALI.
	if got := kucukHarf("USB001"); got != "usb001" {
		t.Errorf("kucukHarf(USB001) = %q", got)
	}
	if got := kucukHarf("İIıi"); got != "İIıi" && got != "İiıi" {
		t.Logf("ASCII dışı rune'lar korunuyor: %q", got)
	}
	if got := kucukHarf("ıIİi"); []rune(got)[1] == 'ı' {
		t.Error("ASCII 'I' Türkçe 'ı'ya çevrilmemeli — port karşılaştırması bozulur")
	}
}

// Sahada iki yazıcı için de kayıt defterinde "UnknownPrinter" yazıyordu ve
// kullanıcı "UnknownPrinter" / "UnknownPrinter 2" kartları gördü — hiçbir şey
// anlatmayan, ayırt edilemeyen isimler.
func TestDonanimdanAdAnlamsizYerTutuculariAtar(t *testing.T) {
	for _, g := range []string{
		"UnknownPrinter", "unknownprinter", "UNKNOWNPRINTER",
		"Unknown", "Printer", "USBPrinter", "LocalPrint", "Generic", "DOT4PRT",
	} {
		if got := DonanimdanAd(g); got != "Fiş Yazıcısı" {
			t.Errorf("DonanimdanAd(%q) = %q, 'Fiş Yazıcısı' beklenir", g, got)
		}
	}
}

// Gerçek model adları YER TUTUCU SANILMAMALI.
func TestDonanimdanAdGercekModelleriKorur(t *testing.T) {
	for girdi, bekler := range map[string]string{
		"ZiJiangZJ-80D6E4": "ZiJiangZJ-80",
		"Zjiang_POS-80":    "Zjiang POS-80",
		"POS-80":           "POS-80",
		"GenericPOS-58":    "GenericPOS-58", // "generic" ile BAŞLIYOR ama eşit değil
	} {
		if got := DonanimdanAd(girdi); got != bekler {
			t.Errorf("DonanimdanAd(%q) = %q, beklenen %q", girdi, got, bekler)
		}
	}
}

// İki yer tutucu yan yana gelince adlar yine ÇAKIŞMAMALI.
func TestIkiAnlamsizYaziciAyriAdAlir(t *testing.T) {
	canli := map[string]string{"USB001": "UnknownPrinter", "USB002": "UnknownPrinter"}
	got := KurulabilirleriBul(canli, true, map[string]YaziciDurumu{})
	if len(got) != 2 {
		t.Fatalf("2 bekleniyordu, %d geldi", len(got))
	}
	if got[0].OnerilenAd == got[1].OnerilenAd {
		t.Fatalf("adlar çakıştı: %q", got[0].OnerilenAd)
	}
	if got[0].OnerilenAd != "Fiş Yazıcısı" {
		t.Errorf("ilki 'Fiş Yazıcısı' olmalı, %q geldi", got[0].OnerilenAd)
	}
}
