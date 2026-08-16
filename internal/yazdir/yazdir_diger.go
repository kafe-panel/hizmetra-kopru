//go:build !windows

package yazdir

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
)

// spoolerYazPlatform — macOS/Linux'ta CUPS üzerinden RAW (ham ESC/POS) baskı.
//
// `lp -d <yazici> -o raw`: "-o raw" CUPS filtre zincirini DEVRE DIŞI bırakır →
// baytlar yazıcıya olduğu gibi gider. Bu, Windows spooler'daki RAW datatype'ın
// (bkz. yazdir_windows.go) karşılığıdır; olmazsa CUPS ESC/POS baytlarını
// "metin/PDF" sanıp filtreden geçirir ve fiş bozulur. Veri stdin'den beslenir.
//
// Bu fonksiyon YALNIZ yerel/CUPS kuyruk adları için çağrılır (bkz. yazdir.go:Bas).
// Ağ yazıcıları (ip:9100) ortak yazdir.go'daki agaBas ile HER platformda çalışır
// ve buraya HİÇ gelmez — o yüzden burada IP/soket mantığı yoktur.
func spoolerYazPlatform(yaziciAdi string, veri []byte) error {
	yol, err := exec.LookPath("lp")
	if err != nil {
		return errors.New("yazdir: CUPS/lp bulunamadı — CUPS'u yükleyin veya panelde 'ip:9100' ağ hedefi kullanın")
	}
	cmd := exec.Command(yol, "-d", yaziciAdi, "-o", "raw")
	cmd.Stdin = bytes.NewReader(veri)
	// CombinedOutput: lp hata verirse stderr'i (ör. "unknown printer") hataya katalım.
	cikti, err := cmd.CombinedOutput()
	if err != nil {
		mesaj := bytes.TrimSpace(cikti)
		if len(mesaj) == 0 {
			return fmt.Errorf("yazdir: '%s' CUPS baskısı başarısız: %w", yaziciAdi, err)
		}
		return fmt.Errorf("yazdir: '%s' CUPS baskısı başarısız: %w (%s)", yaziciAdi, err, mesaj)
	}
	return nil
}
