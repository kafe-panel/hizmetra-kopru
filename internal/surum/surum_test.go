package surum

import "testing"

func TestKarsilastir(t *testing.T) {
	testler := []struct {
		a, b string
		bek  int
	}{
		// Eşitlik
		{"0.7.0", "0.7.0", 0},
		{"v0.7.0", "0.7.0", 0}, // "v" öneki atılır
		{"0.7", "0.7.0", 0},    // eksik bileşen 0
		{"1.0.0", "1.0.0", 0},
		// KRİTİK: sayısal kıyas — düz dizgide "0.10.0" < "0.9.0" olurdu (yanlış).
		{"0.10.0", "0.9.0", 1},
		{"0.9.0", "0.10.0", -1},
		{"0.7.0", "0.6.0", 1},   // gerçek yükseltme
		{"0.6.0", "0.7.0", -1},  // downgrade
		{"1.0.0", "0.99.99", 1}, // major baskın
		{"0.7.10", "0.7.9", 1},  // patch sayısal
		{"0.7.9", "0.7.10", -1},
		// Ön-sürüm/derleme ekleri kesilir → çekirdek kıyaslanır
		{"0.7.0-rc1", "0.7.0", 0},
		{"0.7.0+build5", "0.7.0", 0},
		{"v0.8.0-beta", "0.7.0", 1},
	}
	for _, tt := range testler {
		if got := Karsilastir(tt.a, tt.b); got != tt.bek {
			t.Errorf("Karsilastir(%q,%q)=%d, beklenen %d", tt.a, tt.b, got, tt.bek)
		}
	}
}

func TestKarsilastirSayiDisiDusus(t *testing.T) {
	// Sayı-dışı çekirdek → düz dizgi kıyasına düşer, panik/yanlış sonuç YOK.
	if Karsilastir("abc", "abc") != 0 {
		t.Error("aynı sayı-dışı dizgi 0 dönmeli")
	}
	if Karsilastir("0.7.0", "0.7.x") == 0 {
		// biri sayısal biri değil → dizgi kıyası, 0 OLMAMALI (farklı dizgiler)
		t.Error("sayısal vs sayı-dışı: dizgi kıyası farklı olmalı")
	}
}

func TestYeniMi(t *testing.T) {
	if !YeniMi("0.7.0", "0.6.0") {
		t.Error("0.7.0 > 0.6.0 → yeni sürüm var olmalı")
	}
	if YeniMi("0.7.0", "0.7.0") {
		t.Error("eşit sürümde güncelleme sunulmamalı")
	}
	if YeniMi("0.6.0", "0.7.0") {
		t.Error("downgrade'de güncelleme sunulmamalı")
	}
	if !YeniMi("0.10.0", "0.9.0") {
		t.Error("0.10.0 > 0.9.0 → yeni sürüm var olmalı (sayısal)")
	}
}
