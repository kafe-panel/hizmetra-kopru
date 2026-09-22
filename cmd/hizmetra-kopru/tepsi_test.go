package main

import (
	"strings"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/kopru"
)

// TestTepsiDurumMetniBaskiSorunuOncelikli — REGRESYON KAPISI.
//
// Baskı hatasında BAĞLANTI SAĞLAMDIR (d.Bagli true kalır). Eski switch önce
// "SonHata != ” && !d.Bagli" dalına bakıyordu, o dal hiç çalışmıyordu ve
// kullanıcı fiş çıkmazken "✓ Bağlı — son fiş 14:32" görüyordu.
func TestTepsiDurumMetniBaskiSorunuOncelikli(t *testing.T) {
	d := kopru.Durum{
		Bagli:          true,
		SonBaski:       time.Now(),
		YaziciSayi:     2,
		SonBaskiSorunu: "ZJ-80 yazıcısında kağıt bitti — yeni rulo takın.",
		SonBaskiKodu:   "KAGIT_YOK",
	}
	metin := tepsiDurumMetni(&d)
	if strings.Contains(metin, "✓ Bağlı") {
		t.Fatalf("baskı sorunu varken '✓ Bağlı' GÖSTERİLMEMELİ: %q", metin)
	}
	if !strings.Contains(metin, "Fiş basılamıyor") {
		t.Fatalf("baskı sorunu metni eksik: %q", metin)
	}
}

func TestTepsiDurumMetniDigerHaller(t *testing.T) {
	if m := tepsiDurumMetni(&kopru.Durum{}); !strings.Contains(m, "bekleniyor") {
		t.Errorf("başlangıç metni yanlış: %q", m)
	}
	if m := tepsiDurumMetni(&kopru.Durum{SonHata: "ağ yok"}); !strings.HasPrefix(m, "⚠") {
		t.Errorf("kopuk bağlantıda uyarı bekleniyordu: %q", m)
	}
	if m := tepsiDurumMetni(&kopru.Durum{Bagli: true, YaziciSayi: 3}); !strings.Contains(m, "✓ Bağlı (3 yazıcı)") {
		t.Errorf("bağlı metni yanlış: %q", m)
	}
	d := kopru.Durum{Bagli: true, YaziciSayi: 1, IsletmeAd: "Karikatür Bi Kafe"}
	if m := tepsiDurumMetni(&d); !strings.HasPrefix(m, "Karikatür Bi Kafe · ") {
		t.Errorf("işletme adı öne konmalı: %q", m)
	}
}

func TestKurArgleri(t *testing.T) {
	if ad, port, ok := kurArgleri([]string{"--yazici-kur", "Fiş Yazıcısı", "USB001"}); !ok || ad != "Fiş Yazıcısı" || port != "USB001" {
		t.Fatalf("beklenen ayıklama olmadı: %q %q %v", ad, port, ok)
	}
	// Bayrak başka konumda da bulunmalı.
	if _, port, ok := kurArgleri([]string{"--baska", "--yazici-kur", "Kasa", "USB002"}); !ok || port != "USB002" {
		t.Fatal("bayrak ikinci konumda bulunamadı")
	}
	// Eksik argüman → asla tamam değil (yükseltilmiş süreç yanlış anlamayla çalışmamalı).
	for _, args := range [][]string{
		{"--yazici-kur"},
		{"--yazici-kur", "Kasa"},
		{},
		{"--kaldir-sunucu"},
	} {
		if _, _, ok := kurArgleri(args); ok {
			t.Errorf("tamam=false beklenirdi: %v", args)
		}
	}
}
