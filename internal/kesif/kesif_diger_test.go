//go:build !windows

package kesif

import (
	"testing"

	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// TestAyristirLpstat — `lpstat -e` çıktısı Yazici listesine doğru çevrilmeli.
// Windows'ta derlenmez (build tag); linux CI'da `go test ./...` ile koşar.
func TestAyristirLpstat(t *testing.T) {
	cikti := "EPSON_TM_T20\nMutfak-80\n\n  Bar_Yazici  \n"
	yzc := ayristirLpstat(cikti)

	if len(yzc) != 3 {
		t.Fatalf("3 yazıcı bekleniyordu, %d geldi: %+v", len(yzc), yzc)
	}
	beklenen := []string{"EPSON_TM_T20", "Mutfak-80", "Bar_Yazici"}
	for i, ad := range beklenen {
		if yzc[i].Ad != ad {
			t.Errorf("[%d] Ad = %q, beklenen %q", i, yzc[i].Ad, ad)
		}
		if yzc[i].Hedef != ad {
			t.Errorf("[%d] Hedef = %q, beklenen %q (CUPS'ta hedef = ad)", i, yzc[i].Hedef, ad)
		}
		if yzc[i].Tip != "yerel" {
			t.Errorf("[%d] Tip = %q, beklenen \"yerel\"", i, yzc[i].Tip)
		}
		if yzc[i].Durum != "online" {
			t.Errorf("[%d] Durum = %q, beklenen \"online\"", i, yzc[i].Durum)
		}
	}
}

// TestAyristirLpstatBos — boş çıktı (yazıcı yok) boş ama non-nil liste vermeli.
func TestAyristirLpstatBos(t *testing.T) {
	yzc := ayristirLpstat("\n   \n")
	if yzc == nil {
		t.Fatal("non-nil boş liste bekleniyordu, nil geldi")
	}
	if len(yzc) != 0 {
		t.Fatalf("boş liste bekleniyordu, %d geldi", len(yzc))
	}
}

// TestDurumBelirle — SABİT "online" yalanı bitti: çevrimdışı işaretli / devre
// dışı kuyruk "offline", normal kuyruk "online" olmalı.
func TestDurumBelirle(t *testing.T) {
	if d, _ := durumBelirle(yazdir.YaziciDurumu{Ad: "Mutfak", Duraklatildi: true}); d != DurumCevrimdisi {
		t.Errorf("devre dışı kuyruk offline olmalı, %q geldi", d)
	}
	if d, _ := durumBelirle(yazdir.YaziciDurumu{Ad: "Kasa", CevrimdisiIsaretli: true}); d != DurumCevrimdisi {
		t.Errorf("çevrimdışı işaretli yazıcı offline olmalı, %q geldi", d)
	}
	if d, _ := durumBelirle(yazdir.YaziciDurumu{Ad: "Bar", Port: "USB001"}); d != DurumCevrimici {
		t.Errorf("normal kuyruk online olmalı, %q geldi", d)
	}
	// Sanal/dosya portu kağıt çıkarmaz → offline + uyarı.
	d, uyari := durumBelirle(yazdir.YaziciDurumu{Ad: "XPS", Port: "PORTPROMPT:"})
	if d != DurumCevrimdisi || uyari == "" {
		t.Errorf("sanal hedef offline + uyarılı olmalı: (%q,%q)", d, uyari)
	}
}

// TestDurumDegerleriYALNIZIkiTane — PROTOKOL SABİTİ: sunucu bu alanda başka
// değer beklemiyor. Yeni bir durum değeri eklemek şema değişikliğidir.
func TestDurumDegerleriYalnizIkiTane(t *testing.T) {
	gorulen := map[string]bool{}
	ornekler := []yazdir.YaziciDurumu{
		{Ad: "a"},
		{Ad: "b", Duraklatildi: true},
		{Ad: "c", CevrimdisiIsaretli: true},
		{Ad: "d", Port: "nul:"},
		{Ad: "e", Port: "USB001"},
		{Ad: "f", Port: "IP_192.168.1.50"},
	}
	for _, o := range ornekler {
		d, _ := durumBelirle(o)
		gorulen[d] = true
	}
	for d := range gorulen {
		if d != DurumCevrimici && d != DurumCevrimdisi {
			t.Fatalf("beklenmeyen durum değeri: %q", d)
		}
	}
	if len(gorulen) != 2 {
		t.Fatalf("iki durum değeri bekleniyordu, görülenler: %v", gorulen)
	}
}
