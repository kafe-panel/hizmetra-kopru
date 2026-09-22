//go:build !windows

package yazdir

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// TestLpIsKimligi — `lp` çıktısından CUPS iş kimliği ayrıştırılmalı.
func TestLpIsKimligi(t *testing.T) {
	durumlar := []struct {
		cikti  string
		bekler string
	}{
		{"request id is EPSON_TM_T20-42 (1 file(s))\n", "EPSON_TM_T20-42"},
		{"  request id is Mutfak-7 (1 file(s))", "Mutfak-7"},
		{"request id is Bar-1\n", "Bar-1"},
		{"", ""},
		{"lp: Error - unknown printer\n", ""},
	}
	for _, d := range durumlar {
		if got := lpIsKimligi(d.cikti); got != d.bekler {
			t.Errorf("lpIsKimligi(%q) = %q, beklenen %q", d.cikti, got, d.bekler)
		}
	}
}

// TestCupsIsListede — `lpstat -o` çıktısında iş duruyor mu?
func TestCupsIsListede(t *testing.T) {
	cikti := "Mutfak-7   deniz   1024   Pzt 01 Eyl 2026 10:00:00\nMutfak-8   deniz   2048   Pzt 01 Eyl 2026 10:00:05\n"
	if !cupsIsListede(cikti, "Mutfak-7") {
		t.Error("listedeki iş bulunamadı")
	}
	if cupsIsListede(cikti, "Mutfak-9") {
		t.Error("listede olmayan iş bulundu sanıldı")
	}
	if cupsIsListede("", "Mutfak-7") {
		t.Error("boş listede iş bulunmamalı")
	}
}

// TestLpHataKodu — lp stderr metni teşhis koduna çevrilmeli.
func TestLpHataKodu(t *testing.T) {
	durumlar := []struct {
		mesaj string
		kod   teshis.Kod
	}{
		{"lp: Error - unknown printer or class \"Yok\"", teshis.HEDEF_YOK},
		{"lp: Destination \"Mutfak\" is not accepting jobs.", teshis.KUYRUK_DURAKLATILDI},
		{"lp: Permission denied", teshis.YETKI_YOK},
		{"write error: No space left on device", teshis.DISK_DOLU},
		{"bilinmeyen bir şey", teshis.BILINMEYEN},
	}
	for _, d := range durumlar {
		if got := lpHataKodu(d.mesaj); got != d.kod {
			t.Errorf("lpHataKodu(%q) = %q, beklenen %q", d.mesaj, got, d.kod)
		}
	}
}

// TestSpoolerAsiliLpZamanAsimi — asılı kalan bir 'lp' süreci 30 saniyede (testte
// kısaltılmış süre) ÖLDÜRÜLMELİ ve ASILDI bildirilmeli. Eskiden zaman aşımı
// YOKTU: cupsd yanıt vermezse iş döngüsü sonsuza dek bloke oluyor, nabız ayrı
// goroutine'de sürdüğü için panel YEŞİL kalıyordu.
func TestSpoolerAsiliLpZamanAsimi(t *testing.T) {
	dizin := t.TempDir()
	sahteLp := filepath.Join(dizin, "lp")
	if err := os.WriteFile(sahteLp, []byte("#!/bin/sh\nexec /bin/sleep 60\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dizin)

	eskiSure := lpZamanAsimi
	lpZamanAsimi = 400 * time.Millisecond
	defer func() { lpZamanAsimi = eskiSure }()

	basla := time.Now()
	err := spoolerYazPlatform("Mutfak", []byte("x"))
	gecen := time.Since(basla)

	if kod := teshis.KodunuAl(err); kod != teshis.ASILDI {
		t.Fatalf("ASILDI bekleniyordu, %q geldi (%v)", kod, err)
	}
	if gecen > 5*time.Second {
		t.Fatalf("süreç öldürülmedi, %v bekledik", gecen)
	}
}

// TestAyristirLpstatP — `lpstat -p` çıktısı kuyruk durumlarına çevrilmeli.
func TestAyristirLpstatP(t *testing.T) {
	cikti := "" +
		"printer EPSON_TM_T20 is idle.  enabled since Pzt 01 Eyl 2026 10:00:00\n" +
		"printer Mutfak disabled since Pzt 01 Eyl 2026 10:05:00 -\n" +
		"\treason: media-empty\n" +
		"printer Bar now printing Bar-3.  enabled since Pzt 01 Eyl 2026 10:06:00\n" +
		"printer Kapak disabled since Pzt 01 Eyl 2026 10:07:00 -\n" +
		"\tcover-open\n"

	k := ayristirLpstatP(cikti)
	if len(k) != 4 {
		t.Fatalf("4 kuyruk bekleniyordu, %d geldi: %+v", len(k), k)
	}
	if k["EPSON_TM_T20"].DevreDisi {
		t.Error("idle kuyruk devre dışı sayıldı")
	}
	if !k["Mutfak"].DevreDisi || k["Mutfak"].Sebep != teshis.KAGIT_YOK {
		t.Errorf("Mutfak yanlış çözüldü: %+v", k["Mutfak"])
	}
	if !k["Bar"].Basiyor || k["Bar"].DevreDisi {
		t.Errorf("Bar yanlış çözüldü: %+v", k["Bar"])
	}
	if k["Kapak"].Sebep != teshis.KAPAK_ACIK {
		t.Errorf("Kapak sebebi yanlış: %+v", k["Kapak"])
	}
	if len(ayristirLpstatP("")) != 0 {
		t.Error("boş çıktıda kuyruk olmamalı")
	}
}
