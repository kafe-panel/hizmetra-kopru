//go:build !windows

package yazdir

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// spoolerYazPlatform — macOS/Linux'ta CUPS üzerinden RAW (ham ESC/POS) baskı.
//
// `lp -d <yazici> -o raw`: "-o raw" CUPS filtre zincirini DEVRE DIŞI bırakır →
// baytlar yazıcıya olduğu gibi gider. Bu, Windows spooler'daki RAW datatype'ın
// (bkz. yazdir_windows.go) karşılığıdır; olmazsa CUPS ESC/POS baytlarını
// "metin/PDF" sanıp filtreden geçirir ve fiş bozulur. Veri stdin'den beslenir.
//
// ZAMAN AŞIMI (2026-09-22): eskiden exec.Command süresiz bekliyordu. cupsd
// yanıt vermezse (kapanmış USB, asılı sürücü) İŞ DÖNGÜSÜ SONSUZA DEK BLOKE
// oluyordu; nabız ayrı goroutine'de sürdüğü için panel YEŞİL kalıyor, fiş ise
// hiç basılmıyordu. Artık 30 saniyede süreç öldürülür ve ASILDI bildirilir.
// DEĞİŞKEN (sabit değil): testler kısaltıp gerçekten asılan bir 'lp' ile
// zaman aşımını 30 saniye beklemeden doğrulayabilsin.
var lpZamanAsimi = 30 * time.Second

// teslimYoklamaSuresi — `lpstat -o` ile işin kuyruktan düşmesini bu kadar
// bekleriz. Düştüyse CUPS işi cihaza teslim etmiş demektir.
var teslimYoklamaSuresi = 3 * time.Second

func spoolerYazPlatform(yaziciAdi string, veri []byte) error {
	yol, err := exec.LookPath("lp")
	if err != nil {
		return teshis.Yeni(teshis.HEDEF_YOK, yaziciAdi, "yazdırma servisi (CUPS) bulunamadı", err)
	}
	ctx, iptal := context.WithTimeout(context.Background(), lpZamanAsimi)
	defer iptal()

	cmd := exec.CommandContext(ctx, yol, "-d", yaziciAdi, "-o", "raw")
	cmd.Stdin = bytes.NewReader(veri)
	// CombinedOutput: lp hata verirse stderr'i (ör. "unknown printer") hataya katalım.
	cikti, calismaHatasi := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return teshis.Yeni(teshis.ASILDI, yaziciAdi, "", ctx.Err())
	}
	if calismaHatasi != nil {
		mesaj := strings.TrimSpace(string(bytes.TrimSpace(cikti)))
		return teshis.Yeni(lpHataKodu(mesaj), yaziciAdi, kisaltMetin(mesaj, 60), calismaHatasi)
	}

	isKimligi := lpIsKimligi(string(cikti))
	if isKimligi == "" {
		// Kimlik okunamadı: teslim teyidi yapamayız. Yalan söylemek yerine
		// "belirsiz" diyoruz — iş 'basildi' bildirilir ama günlükte kanıt yok.
		gunluk.YazSessiz("'%s' için CUPS iş kimliği okunamadı — teslim teyidi yapılamadı", yaziciAdi)
		return nil
	}
	if teslimEdildi := cupsTeslimBekle(yaziciAdi, isKimligi); !teslimEdildi {
		gunluk.Yaz("iş %s hâlâ kuyrukta — cihaz onayı yok (kağıt çıkmadıysa panelden yeniden gönderin)", isKimligi)
	}
	return nil
}

// lpIsKimligi — `lp` çıktısındaki "request id is <kuyruk>-<n> (1 file(s))"
// satırından iş kimliğini ayıklar. SAF fonksiyon (tablo testi var).
func lpIsKimligi(cikti string) string {
	for _, satir := range strings.Split(cikti, "\n") {
		s := strings.TrimSpace(satir)
		const onek = "request id is "
		i := strings.Index(s, onek)
		if i < 0 {
			continue
		}
		kalan := strings.TrimSpace(s[i+len(onek):])
		if bosluk := strings.IndexByte(kalan, ' '); bosluk > 0 {
			kalan = kalan[:bosluk]
		}
		if kalan != "" {
			return kalan
		}
	}
	return ""
}

// lpHataKodu — lp'nin stderr metninden teşhis kodu çıkarır (SAF).
func lpHataKodu(mesaj string) teshis.Kod {
	m := strings.ToLower(mesaj)
	switch {
	case strings.Contains(m, "unknown printer"), strings.Contains(m, "does not exist"):
		return teshis.HEDEF_YOK
	case strings.Contains(m, "not accepting"), strings.Contains(m, "disabled"):
		return teshis.KUYRUK_DURAKLATILDI
	case strings.Contains(m, "permission"), strings.Contains(m, "not authorized"):
		return teshis.YETKI_YOK
	case strings.Contains(m, "no space"), strings.Contains(m, "disk full"):
		return teshis.DISK_DOLU
	}
	return teshis.BILINMEYEN
}

// cupsTeslimBekle — iş kuyruktan düşene kadar (en fazla teslimYoklamaSuresi)
// yoklar. true = iş listede yok → CUPS cihaza teslim etti.
func cupsTeslimBekle(kuyruk, isKimligi string) bool {
	yol, err := exec.LookPath("lpstat")
	if err != nil {
		return false
	}
	bitis := time.Now().Add(teslimYoklamaSuresi)
	for {
		ctx, iptal := context.WithTimeout(context.Background(), 2*time.Second)
		cikti, _ := exec.CommandContext(ctx, yol, "-W", "not-completed", "-o", kuyruk).Output()
		iptal()
		if !cupsIsListede(string(cikti), isKimligi) {
			return true
		}
		if time.Now().After(bitis) {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// cupsIsListede — `lpstat -o` çıktısında bu iş kimliği duruyor mu? (SAF)
func cupsIsListede(cikti, isKimligi string) bool {
	for _, satir := range strings.Split(cikti, "\n") {
		alanlar := strings.Fields(satir)
		if len(alanlar) > 0 && alanlar[0] == isKimligi {
			return true
		}
	}
	return false
}

func kisaltMetin(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
