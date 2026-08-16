//go:build !windows

package kesif

import "testing"

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
