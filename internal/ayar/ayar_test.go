package ayar

import "testing"

// SunucuAdaylari — tek exe'nin hem production hem staging'e bağlanabilmesinin
// çekirdeği. Öncelik: env > kayıt > bilinenlerin hepsi.
func TestSunucuAdaylari(t *testing.T) {
	t.Setenv("HIZMETRA_API", "") // env yok kabul edilir (boş = ayarsız)

	// 1) env yok + kayıt yok → bilinen sunucuların HEPSİ, production önce denenir.
	a := &Ayar{}
	got := a.SunucuAdaylari()
	if len(got) != len(BilinenSunucular) || got[0] != "https://api.hizmetra.com" {
		t.Fatalf("env/kayıt yokken production-önce tüm bilinenler beklenir: %v", got)
	}

	// 2) kayıtlı SunucuURL → YALNIZ o (cihaz zaten bir sunucuya eşleşmiş).
	a2 := &Ayar{SunucuURL: "https://staging-panel.hizmetra.com"}
	if got := a2.SunucuAdaylari(); len(got) != 1 || got[0] != a2.SunucuURL {
		t.Fatalf("kayıtlı sunucu tek aday olmalı: %v", got)
	}

	// 3) env HIZMETRA_API → YALNIZ o (kayıttan da önceliklidir).
	t.Setenv("HIZMETRA_API", "https://ozel.example.com")
	a3 := &Ayar{SunucuURL: "https://kayitli.example.com"}
	if got := a3.SunucuAdaylari(); len(got) != 1 || got[0] != "https://ozel.example.com" {
		t.Fatalf("env tek aday olmalı: %v", got)
	}
}

// TestSilConfigKaldirir — "Onar"/"Kaldır" akışı config.json'u siler; sonraki
// Yukle boş Ayar döner (= yeniden eşleştirme gerekir). Dosya yokken de hata yok.
func TestSilConfigKaldirir(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir()) // os.UserConfigDir (Windows) → izole dizin
	if err := Kaydet(&Ayar{Token: "x", SunucuURL: "https://ornek"}); err != nil {
		t.Fatal(err)
	}
	if err := Sil(); err != nil {
		t.Fatal(err)
	}
	a, err := Yukle()
	if err != nil {
		t.Fatal(err)
	}
	if a.Token != "" || a.SunucuURL != "" {
		t.Fatalf("config silinmeli, kaldı: %+v", a)
	}
	// İkinci Sil (dosya yok) → hata YOK (idempotent).
	if err := Sil(); err != nil {
		t.Fatalf("dosya yokken Sil hata vermemeli: %v", err)
	}
}
