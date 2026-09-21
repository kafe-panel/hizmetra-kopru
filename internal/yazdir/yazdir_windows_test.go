//go:build windows

package yazdir

import "testing"

// RAW'ın İLK sırada olması sözleşmedir: ESC/POS baytları sürücü render'ından
// geçmemeli ve winprint kuyrukları (fiş yazıcılarının ezici çoğunluğu) RAW'ı
// her zaman destekler. Sıra bozulursa ZJ-80 sınıfı yazıcılar yine 1804 alır.
func TestSpoolerVeriTurleriRAWOnce(t *testing.T) {
	if len(spoolerVeriTurleri) == 0 || spoolerVeriTurleri[0] != "RAW" {
		t.Fatalf("ilk veri türü RAW olmalı, gelen: %v", spoolerVeriTurleri)
	}
	var xpsVar bool
	for _, d := range spoolerVeriTurleri {
		if d == "XPS_PASS" {
			xpsVar = true
		}
	}
	if !xpsVar {
		t.Fatal("gerçek v4/XPS kuyruklar için XPS_PASS yedeği kalmalı")
	}
}

func TestSpoolerBosIcerikYazmaz(t *testing.T) {
	if err := spoolerYazPlatform("yok-boyle-yazici", nil); err == nil {
		t.Fatal("boş içerik hata döndürmeliydi")
	}
}
