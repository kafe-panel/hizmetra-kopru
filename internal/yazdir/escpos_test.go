package yazdir

import (
	"bytes"
	"testing"
)

func TestDleEotSorguBaytlari(t *testing.T) {
	if !bytes.Equal(DleEotSorgu(4), []byte{0x10, 0x04, 0x04}) {
		t.Errorf("n=4 sorgusu yanlış: % x", DleEotSorgu(4))
	}
	if !bytes.Equal(DleEotSorgu(2), []byte{0x10, 0x04, 0x02}) {
		t.Errorf("n=2 sorgusu yanlış: % x", DleEotSorgu(2))
	}
	if !bytes.Equal(DleEotSorgu(1), []byte{0x10, 0x04, 0x01}) {
		t.Errorf("n=1 sorgusu yanlış: % x", DleEotSorgu(1))
	}
}

func TestDurumBitiKagitSensoru(t *testing.T) {
	// Taban geçerlilik bitleri: bit1 + bit4 = 0x12
	const taban byte = 0x12

	// Kağıt var.
	kagitYok, _, _, gecerli := DurumBiti(DleEotKagitSensoru, taban)
	if !gecerli || kagitYok {
		t.Errorf("kağıt varken kagitYok=false olmalı (gecerli=%v)", gecerli)
	}
	// Kağıt AZALDI (bit2+bit3) — bu hata değil, baskı sürer.
	kagitYok, _, _, _ = DurumBiti(DleEotKagitSensoru, taban|0x0C)
	if kagitYok {
		t.Error("kağıt azaldı ≠ kağıt bitti")
	}
	// Kağıt BİTTİ (bit5+bit6).
	kagitYok, _, _, gecerli = DurumBiti(DleEotKagitSensoru, taban|0x60)
	if !gecerli || !kagitYok {
		t.Errorf("kağıt bitti okunmadı (kagitYok=%v gecerli=%v)", kagitYok, gecerli)
	}
}

func TestDurumBitiKapakAcik(t *testing.T) {
	const taban byte = 0x12
	_, kapakAcik, cevrimdisi, gecerli := DurumBiti(DleEotCevrimdisiSebep, taban|0x04)
	if !gecerli || !kapakAcik || !cevrimdisi {
		t.Errorf("kapak açık biti okunmadı (kapak=%v cevrimdisi=%v gecerli=%v)", kapakAcik, cevrimdisi, gecerli)
	}
	kagitYok, kapakAcik, _, _ := DurumBiti(DleEotCevrimdisiSebep, taban|0x20)
	if !kagitYok || kapakAcik {
		t.Errorf("n=2 kağıt-sonu biti yanlış (kagitYok=%v kapak=%v)", kagitYok, kapakAcik)
	}
}

func TestDurumBitiGecersiz(t *testing.T) {
	// Geçerlilik bitleri tutmuyor → hiçbir sonuç çıkarılmamalı.
	if _, _, _, gecerli := DurumBiti(DleEotKagitSensoru, 0x00); gecerli {
		t.Error("geçersiz cevap geçerli sayıldı")
	}
	if _, _, _, gecerli := DurumBiti(DleEotKagitSensoru, 0xFF); gecerli {
		t.Error("0xFF geçerli sayılmamalı")
	}
	// Bilinmeyen n.
	if _, _, _, gecerli := DurumBiti(7, 0x12); gecerli {
		t.Error("bilinmeyen n için gecerli=false olmalı")
	}
}

func TestDurumBitiCevrimdisi(t *testing.T) {
	const taban byte = 0x12
	_, _, cevrimdisi, gecerli := DurumBiti(DleEotYaziciDurumu, taban|0x08)
	if !gecerli || !cevrimdisi {
		t.Errorf("n=1 çevrimdışı biti okunmadı (cevrimdisi=%v)", cevrimdisi)
	}
	_, _, cevrimdisi, _ = DurumBiti(DleEotYaziciDurumu, taban)
	if cevrimdisi {
		t.Error("çevrimiçi yazıcı çevrimdışı sayıldı")
	}
}
