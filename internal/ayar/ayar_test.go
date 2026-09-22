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

	// 2) kayıtlı SunucuURL → ÖNCE o, ama TEK aday DEĞİL.
	//
	// 2026-09-22'de değişti: eskiden yalnız kayıtlı sunucu denendiği için, bir
	// kez eşleşmiş ajan o sunucuya SONSUZA KADAR kilitleniyordu — başka bir
	// panelde üretilen kod 404 alıp "kod geçersiz" gösteriyordu ve arayüzde
	// kilidi açacak düğme yoktu.
	a2 := &Ayar{SunucuURL: "https://staging-panel.hizmetra.com"}
	got2 := a2.SunucuAdaylari()
	if len(got2) < 2 || got2[0] != a2.SunucuURL {
		t.Fatalf("kayıtlı sunucu önce denenmeli ama tek aday olmamalı: %v", got2)
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

// ── Eşleştirme sunucu kilidi (2026-09-22) ───────────────────────────────────

func TestSunucuAdaylariKayitliSunucuyaKILITLEMEZ(t *testing.T) {
	t.Setenv("HIZMETRA_API", "")
	a := &Ayar{SunucuURL: "https://staging-panel.hizmetra.com"}
	adaylar := a.SunucuAdaylari()

	if len(adaylar) < 2 {
		t.Fatalf("kayıtlı sunucu TEK aday olmamalı — başka panelde üretilen kod asla eşleşemezdi: %v", adaylar)
	}
	if adaylar[0] != a.SunucuURL {
		t.Errorf("kayıtlı sunucu İLK denenmeli, gelen sıra: %v", adaylar)
	}
	// production da listede olmalı
	var prodVar bool
	for _, s := range adaylar {
		if s == "https://api.hizmetra.com" {
			prodVar = true
		}
	}
	if !prodVar {
		t.Errorf("diğer bilinen sunucular da denenmeli: %v", adaylar)
	}
}

func TestSunucuAdaylariTekrarEtmez(t *testing.T) {
	t.Setenv("HIZMETRA_API", "")
	a := &Ayar{SunucuURL: "https://api.hizmetra.com"} // zaten BilinenSunucular'da
	adaylar := a.SunucuAdaylari()
	gorulen := map[string]bool{}
	for _, s := range adaylar {
		if gorulen[s] {
			t.Fatalf("aynı sunucu iki kez denenmemeli: %v", adaylar)
		}
		gorulen[s] = true
	}
	if adaylar[0] != "https://api.hizmetra.com" {
		t.Errorf("kayıtlı sunucu ilk sırada olmalı: %v", adaylar)
	}
}

func TestSunucuAdaylariEnvTekBasinaEzer(t *testing.T) {
	t.Setenv("HIZMETRA_API", "http://localhost:5002")
	a := &Ayar{SunucuURL: "https://api.hizmetra.com"}
	adaylar := a.SunucuAdaylari()
	if len(adaylar) != 1 || adaylar[0] != "http://localhost:5002" {
		t.Fatalf("env açık geçersiz kılmadır, TEK aday olmalı: %v", adaylar)
	}
}

func TestSunucuAdaylariKayitYoksaBilinenler(t *testing.T) {
	t.Setenv("HIZMETRA_API", "")
	a := &Ayar{}
	if got := a.SunucuAdaylari(); len(got) != len(BilinenSunucular) {
		t.Fatalf("kayıt yokken bilinen sunucular denenmeli: %v", got)
	}
}
