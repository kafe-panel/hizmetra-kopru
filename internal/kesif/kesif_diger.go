//go:build !windows

package kesif

import (
	"os/exec"
	"strings"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// bulPlatform — macOS/Linux'ta CUPS'a kayıtlı yazıcı kuyruklarını listeler.
//
// `lpstat -e` HER satırda tam bir hedef adı verir (ekstra durum metni yok), bu
// yüzden `lpstat -p`ye göre ayrıştırması çok daha az kırılgandır. CUPS/lpstat
// kurulu değilse ya da cupsd çalışmıyorsa BOŞ liste + nil döner: keşif
// OPSİYONELDİR — kullanıcı panelden "ip:9100" ağ yazıcısı girerek ajanı yine
// kullanır (bkz. kesif.go, yazdir.go). Bu yüzden keşif hatası ajanı durdurmaz.
func bulPlatform() ([]api.Yazici, error) {
	yol, err := exec.LookPath("lpstat")
	if err != nil {
		return []api.Yazici{}, nil // CUPS yok — keşif atlanır (hata DEĞİL)
	}
	cikti, err := exec.Command(yol, "-e").Output()
	if err != nil {
		return []api.Yazici{}, nil // cupsd kapalı vb. — keşif atlanır (hata DEĞİL)
	}
	return birlestirDurumlar(ayristirLpstat(string(cikti))), nil
}

// birlestirDurumlar — `lpstat -e` ad listesine, paylaşımlı kuyruk
// önbelleğinden (yazdir) GERÇEK durumu ekler. Ad listesi `lpstat -e`den gelir
// (ayrıştırması sağlam), durum `lpstat -p`den.
func birlestirDurumlar(yazicilar []api.Yazici) []api.Yazici {
	durumlar, err := yazdir.YazicilariOku()
	if err != nil || len(durumlar) == 0 {
		return yazicilar // durum bilgisi yok → best-effort "online" kalır
	}
	for i := range yazicilar {
		d, varmi := durumlar[yazicilar[i].Ad]
		if !varmi {
			continue
		}
		durum, uyari := durumBelirle(d)
		yazicilar[i].Durum = durum
		yazicilar[i].Uyari = uyari
	}
	return yazicilar
}

// ayristirLpstat — `lpstat -e` çıktısını Yazici listesine çevirir. Saf fonksiyon
// (birim testi kesif_diger_test.go'da). kesif_windows.go ile AYNI alanları
// doldurur: Ad, Hedef (=Ad), Tip, Durum.
func ayristirLpstat(cikti string) []api.Yazici {
	sonuc := make([]api.Yazici, 0)
	for _, satir := range strings.Split(cikti, "\n") {
		ad := strings.TrimSpace(satir)
		if ad == "" {
			continue
		}
		sonuc = append(sonuc, api.Yazici{
			Ad:    ad,
			Hedef: ad,       // CUPS'ta hedef = kuyruk adı (lp -d <ad>)
			Tip:   "yerel",  // Windows "windows" der; burada yerel CUPS kuyruğu
			Durum: "online", // best-effort (kesif_windows.go ile aynı); gerçek kanıt baskı sonucudur
		})
	}
	return sonuc
}
