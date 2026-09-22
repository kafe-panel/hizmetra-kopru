package yazdir

import (
	"errors"
	"testing"
	"time"
)

// TestOnbellekGercekTaramayiKisar — 10 saniye içinde 10 çağrıda GERÇEK tarama
// sayısı 1 olmalı.
//
// REGRESYON KAPISI: durum penceresi 3 saniyede bir özet topluyor ve her
// seferinde sistemin tüm yazıcılarını yeniden tarıyordu (Windows'ta
// EnumPrinters). Baskı ön kontrolü de aynı veriyi istiyor; üç tüketici tek
// önbellekten okumalı.
func TestOnbellekGercekTaramayiKisar(t *testing.T) {
	eskiTarayici := yaziciTarayici
	defer func() { yaziciTarayici = eskiTarayici; OnbellegiTemizle() }()

	OnbellegiTemizle()
	basSayac := taramaSayaci
	yaziciTarayici = func() (map[string]YaziciDurumu, error) {
		return map[string]YaziciDurumu{"ZJ-80": {Ad: "ZJ-80", Port: "USB001"}}, nil
	}

	for i := 0; i < 10; i++ {
		y, err := YazicilariOku()
		if err != nil {
			t.Fatalf("%d. okuma hata verdi: %v", i, err)
		}
		if y["ZJ-80"].Port != "USB001" {
			t.Fatalf("%d. okumada veri bozuldu: %+v", i, y)
		}
	}
	if gercek := taramaSayaci - basSayac; gercek != 1 {
		t.Fatalf("10 çağrıda 1 gerçek tarama bekleniyordu, %d oldu", gercek)
	}
}

// TestOnbellekKopyaDoner — dönen harita üzerinde oynamak önbelleği BOZMAMALI.
func TestOnbellekKopyaDoner(t *testing.T) {
	eskiTarayici := yaziciTarayici
	defer func() { yaziciTarayici = eskiTarayici; OnbellegiTemizle() }()

	OnbellegiTemizle()
	yaziciTarayici = func() (map[string]YaziciDurumu, error) {
		return map[string]YaziciDurumu{"A": {Ad: "A"}}, nil
	}
	ilk, _ := YazicilariOku()
	delete(ilk, "A")
	ikinci, _ := YazicilariOku()
	if _, varmi := ikinci["A"]; !varmi {
		t.Fatal("önbellek dışarıdan bozuldu — kopya dönmüyor")
	}
}

// TestOnbellekHataDaEskiVeriyiKorur — anlık tarama hatası yüzünden "yazıcı yok"
// deyip HEDEF_YOK üretmemeliyiz.
func TestOnbellekHataDaEskiVeriyiKorur(t *testing.T) {
	eskiTarayici := yaziciTarayici
	eskiTaze := onbellekTazeSn
	defer func() {
		yaziciTarayici = eskiTarayici
		onbellekTazeSn = eskiTaze
		OnbellegiTemizle()
	}()

	OnbellegiTemizle()
	yaziciTarayici = func() (map[string]YaziciDurumu, error) {
		return map[string]YaziciDurumu{"A": {Ad: "A"}}, nil
	}
	if _, err := YazicilariOku(); err != nil {
		t.Fatal(err)
	}
	onbellekTazeSn = time.Nanosecond // önbelleği bayatlat
	yaziciTarayici = func() (map[string]YaziciDurumu, error) {
		return nil, errors.New("spooler yanıt vermedi")
	}
	y, err := YazicilariOku()
	if err == nil {
		t.Error("tarama hatası çağırana bildirilmeli")
	}
	if _, varmi := y["A"]; !varmi {
		t.Fatal("tarama hatasında ESKİ önbellek korunmalı")
	}
}
