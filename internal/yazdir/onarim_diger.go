//go:build !windows

package yazdir

import (
	"context"
	"os/exec"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/onarim"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// macOS/Linux onarım dalı — onarim_windows.go ile AYNI dışa açık imzaları
// karşılar (yazdir_diger.go / kesif_diger.go deseni). Windows'a özgü hiçbir
// sembol (registry, winspool) buraya SIZMAZ.
//
// Gerçekleşen tek onarım: `cupsenable <kuyruk>` — Windows'taki
// PRINTER_CONTROL_RESUME'un ikizi, `cupsdisable` ile geri alınır.

const cupsOnarimZamanAsimi = 10 * time.Second

func onarPlatform(yaziciAd string, kod teshis.Kod, defter *onarim.Defter) OnarimSonucu {
	if kod != teshis.KUYRUK_DURAKLATILDI {
		return OnarimSonucu{Onarilmaz: true, Aciklama: "Bu sorun bu bilgisayarda kendiliğinden düzeltilemez."}
	}
	yol, err := exec.LookPath("cupsenable")
	if err != nil {
		return OnarimSonucu{Onarilmaz: true, Aciklama: "Yazdırma servisi aracı bulunamadı; değişiklik yapılmadı."}
	}
	// ÖNCE deftere yaz (eski değer), SONRA uygula.
	kayitID, yazmaHatasi := defter.Yaz(yaziciAd, "duraklatmayi-kaldir", "devre-disi", "devrede")
	if yazmaHatasi != nil {
		return OnarimSonucu{Aciklama: "Onarım kaydı yazılamadığı için değişiklik yapılmadı."}
	}
	ctx, iptal := context.WithTimeout(context.Background(), cupsOnarimZamanAsimi)
	defer iptal()
	if err := exec.CommandContext(ctx, yol, yaziciAd).Run(); err != nil {
		defter.GeriAl(kayitID) // uygulanamadı → kayıt aktif kalmasın
		return OnarimSonucu{Aciklama: "Baskı sırası yeniden başlatılamadı; yazıcı ayarlarını kontrol edin."}
	}
	return OnarimSonucu{
		Yapildi:  true,
		KayitID:  kayitID,
		Aciklama: "Baskı sırası yeniden çalıştırıldı.",
	}
}

// geriAlPlatform — deftere göre eski durumu geri yükler.
func geriAlPlatform(yaziciAd, islem, eskiDeger string) bool {
	if islem != "duraklatmayi-kaldir" || eskiDeger != "devre-disi" {
		return false
	}
	yol, err := exec.LookPath("cupsdisable")
	if err != nil {
		return false
	}
	ctx, iptal := context.WithTimeout(context.Background(), cupsOnarimZamanAsimi)
	defer iptal()
	return exec.CommandContext(ctx, yol, yaziciAd).Run() == nil
}
