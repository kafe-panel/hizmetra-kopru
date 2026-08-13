package kopru

import (
	"bytes"
	"io"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

// TestE2EGercekSunucu — GERÇEK Flask sunucusuna karşı uçtan uca kanıt.
//
// Birim testleri httptest ile sözleşmeyi pinler; bu test gerçek backend'in
// gerçekten aynı sözleşmeyi konuştuğunu kanıtlar (zarf biçimi, durum
// makinesi, atomik sahiplenme, sonuç yazımı).
//
// Çalıştırmak için (Flask :5002'de ayakta olmalı):
//
//	HIZMETRA_E2E_KOD=123456 HIZMETRA_E2E_SEED=<python> HIZMETRA_E2E_SEEDER=<script.py> go test ./internal/kopru -run E2E -v
//
// Değişkenler yoksa test ATLANIR (CI'da Flask yok).
func TestE2EGercekSunucu(t *testing.T) {
	kod := os.Getenv("HIZMETRA_E2E_KOD")
	python := os.Getenv("HIZMETRA_E2E_SEED")
	seeder := os.Getenv("HIZMETRA_E2E_SEEDER")
	if kod == "" || python == "" || seeder == "" {
		t.Skip("E2E değişkenleri yok — atlanıyor")
	}
	sunucu := os.Getenv("HIZMETRA_API")
	if sunucu == "" {
		sunucu = "http://localhost:5002"
	}

	// ── 1. Sahte termal yazıcı: 127.0.0.1'de bir TCP soketi (ip:9100 yolunu test eder) ──
	dinleyici, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("sahte yazıcı kurulamadı: %v", err)
	}
	defer dinleyici.Close()
	hedef := dinleyici.Addr().String()

	basilan := make(chan []byte, 1)
	go func() {
		c, err := dinleyici.Accept()
		if err != nil {
			basilan <- nil
			return
		}
		defer c.Close()
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
		veri, _ := io.ReadAll(c)
		basilan <- veri
	}()

	// ── 2. Eşleştirme: 6 haneli kod → kalıcı token ──
	istemci := api.New(sunucu)
	cevap, err := istemci.Eslestir(kod, "E2E PC", "E2E-MAKINE", "test", "windows")
	if err != nil {
		t.Fatalf("eşleştirme başarısız: %v", err)
	}
	if cevap.Token == "" || cevap.CihazID == 0 {
		t.Fatalf("eşleştirme cevabı eksik: %+v", cevap)
	}
	t.Logf("eşleşti: cihaz=%d işletme=%q", cevap.CihazID, cevap.IsletmeAd)

	// ── 3. Nabız: yazıcıyı bildir + ölçek direktifini al ──
	nabiz, err := istemci.Nabiz([]api.Yazici{
		{Ad: "E2E Termal", Hedef: hedef, Tip: "ag", Durum: "online"},
	}, "test")
	if err != nil {
		t.Fatalf("nabız başarısız: %v", err)
	}
	if nabiz.PollSn <= 0 {
		t.Errorf("poll_sn gelmedi: %+v", nabiz)
	}
	t.Logf("nabız ok: poll_sn=%d bekle_sn=%v", nabiz.PollSn, nabiz.BekleSn)

	// ── 4. Sunucu tarafında köprü yazıcısı + bekleyen fiş işi oluştur ──
	seed := exec.Command(python, seeder, "--cihaz-id", itoa(cevap.CihazID), "--hedef", hedef)
	if cikti, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("seed başarısız: %v\n%s", err, cikti)
	}

	// ── 5. İşi çek (atomik sahiplenme) ──
	isler, err := istemci.Isler(0)
	if err != nil {
		t.Fatalf("iş çekilemedi: %v", err)
	}
	if len(isler) != 1 {
		t.Fatalf("1 iş bekleniyordu, gelen %d: %+v", len(isler), isler)
	}
	t.Logf("iş alındı: #%d hedef=%s", isler[0].IsID, isler[0].Hedef)

	// Aynı işi İKİNCİ kez çekmeye çalış — atomik sahiplenme BOŞ dönmeli.
	tekrar, err := istemci.Isler(0)
	if err != nil {
		t.Fatalf("ikinci çekme hata verdi: %v", err)
	}
	if len(tekrar) != 0 {
		t.Errorf("ÇİFT SAHİPLENME: aynı iş iki kez verildi: %+v", tekrar)
	}

	// ── 6. Ajan mantığıyla bas + sonucu bildir ──
	ajan := Yeni(istemci, basHedefe, func() ([]api.Yazici, error) { return nil, nil }, "test", &Durum{})
	ajan.isleriBas(isler)

	select {
	case veri := <-basilan:
		if len(veri) == 0 {
			t.Fatal("sahte yazıcıya hiç bayt gelmedi")
		}
		// Gerçek ESC/POS başlangıcı mı? Panel fişi VARSAYILAN olarak RASTER
		// (görsel) modda üretir → "GS v 0" (1d 76 30). Metin modunda "ESC @"
		// (1b 40) gelir. İkisi de geçerli; başka bir şeyle başlıyorsa bayt
		// akışı bozulmuş demektir.
		rasterMi := bytes.HasPrefix(veri, []byte{0x1D, 0x76, 0x30})
		metinMi := bytes.HasPrefix(veri, []byte{0x1B, 0x40})
		if !rasterMi && !metinMi {
			t.Errorf("ESC/POS başlangıcı tanınmadı: % x", veri[:min2(8, len(veri))])
		}
		t.Logf("sahte yazıcıya %d bayt basıldı (ilk baytlar: % x)", len(veri), veri[:min2(12, len(veri))])
	case <-time.After(8 * time.Second):
		t.Fatal("sahte yazıcı baytları almadı — baskı yolu kopuk")
	}

	// ── 7. Sunucuda iş 'basildi' olmalı ──
	dogrula := exec.Command(python, seeder, "--dogrula", "--cihaz-id", itoa(cevap.CihazID))
	cikti, err := dogrula.CombinedOutput()
	if err != nil {
		t.Fatalf("doğrulama başarısız: %v\n%s", err, cikti)
	}
	if !bytes.Contains(cikti, []byte("DURUM=basildi")) {
		t.Fatalf("sunucuda iş 'basildi' değil:\n%s", cikti)
	}
	t.Log("sunucu doğrulaması: iş 'basildi' ✓")
}

func basHedefe(hedef string, veri []byte) error {
	c, err := net.DialTimeout("tcp", hedef, 3*time.Second)
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.Write(veri)
	time.Sleep(200 * time.Millisecond)
	return err
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
