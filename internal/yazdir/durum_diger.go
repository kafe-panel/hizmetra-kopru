//go:build !windows

package yazdir

import (
	"os/exec"
	"strings"

	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// macOS/Linux kuyruk durumu — `lpstat -p` çıktısından okunur.
//
// Windows'taki EnumPrinters karşılığıdır (bkz. durum_windows.go): amaç AYNI —
// "kuyruk var mı, devre dışı mı, sebebi ne". Ayrıştırma SAF fonksiyondadır
// (ayristirLpstatP) ve macOS'ta tablo testiyle sınanır; komutu çalıştırma işi
// ayrıdır, çünkü CI'da CUPS olmayabilir.

// lpstatKuyruk — `lpstat -p` satırlarından çıkan tek kuyruk.
type lpstatKuyruk struct {
	Ad        string
	DevreDisi bool       // "disabled since …" → iş kabul edilir ama basılmaz
	Basiyor   bool       // "now printing …"
	Sebep     teshis.Kod // reason satırından çıkan sorun kodu (yoksa boş)
}

// taraPlatform — CUPS kuyruklarını okur. CUPS/lpstat yoksa BOŞ harita + nil
// döner (keşif opsiyoneldir; kullanıcı "ip:9100" ağ hedefiyle yine çalışır).
func taraPlatform() (map[string]YaziciDurumu, error) {
	yol, err := exec.LookPath("lpstat")
	if err != nil {
		return map[string]YaziciDurumu{}, nil
	}
	cikti, err := exec.Command(yol, "-p").Output()
	if err != nil {
		// cupsd kapalı / hiç yazıcı yok → lpstat sıfırdan farklı dönebilir.
		// Bu bir ARIZA değildir; boş liste dön.
		return map[string]YaziciDurumu{}, nil
	}
	out := map[string]YaziciDurumu{}
	for ad, k := range ayristirLpstatP(string(cikti)) {
		out[ad] = YaziciDurumu{
			Ad:                 k.Ad,
			CevrimdisiIsaretli: k.DevreDisi,
			Duraklatildi:       k.DevreDisi,
		}
	}
	return out, nil
}

// usbPortlariOkuPlatform — Windows'a özgü kayıt defteri taraması burada YOK.
// Boş küme + tamListe=false dönmek, teshis.UsbPortOlu'nun "kanıt yok → ASLA
// ölü deme" kuralını tetikler; yani macOS/Linux'ta port-ölü teşhisi hiç
// üretilmez.
func usbPortlariOkuPlatform() (map[string]string, bool) { return map[string]string{}, false }

// ayristirLpstatP — `lpstat -p` çıktısını kuyruk haritasına çevirir (SAF).
//
// Tipik çıktı:
//
//	printer EPSON_TM_T20 is idle.  enabled since Pzt 01 Eyl 2026 10:00:00
//	printer Mutfak disabled since Pzt 01 Eyl 2026 10:00:00 -
//	        reason: media-empty
//
// Yalnız "printer <ad> " ile başlayan satırlar ad verir; sonraki girintili
// satırlar en son görülen kuyruğun sebebini taşır.
func ayristirLpstatP(cikti string) map[string]lpstatKuyruk {
	out := map[string]lpstatKuyruk{}
	sonAd := ""
	for _, ham := range strings.Split(cikti, "\n") {
		satir := strings.TrimRight(ham, "\r")
		duz := strings.TrimSpace(satir)
		if duz == "" {
			continue
		}
		if strings.HasPrefix(duz, "printer ") {
			kalan := strings.TrimSpace(strings.TrimPrefix(duz, "printer "))
			bosluk := strings.IndexByte(kalan, ' ')
			if bosluk <= 0 {
				continue
			}
			ad := kalan[:bosluk]
			gerisi := kalan[bosluk+1:]
			k := lpstatKuyruk{Ad: ad}
			k.DevreDisi = strings.Contains(gerisi, "disabled")
			k.Basiyor = strings.Contains(gerisi, "now printing")
			k.Sebep = sebepKodu(gerisi)
			out[ad] = k
			sonAd = ad
			continue
		}
		// Girintili devam satırı → en son kuyruğun sebebi.
		if sonAd != "" {
			k := out[sonAd]
			if kod := sebepKodu(duz); kod != "" {
				k.Sebep = kod
				out[sonAd] = k
			}
		}
	}
	return out
}

// sebepKodu — CUPS "reason" anahtar kelimelerini teşhis koduna çevirir.
func sebepKodu(metin string) teshis.Kod {
	switch {
	case strings.Contains(metin, "media-empty"), strings.Contains(metin, "media-needed"),
		strings.Contains(metin, "out of paper"):
		return teshis.KAGIT_YOK
	case strings.Contains(metin, "cover-open"), strings.Contains(metin, "door-open"):
		return teshis.KAPAK_ACIK
	case strings.Contains(metin, "offline"), strings.Contains(metin, "shutdown"):
		return teshis.YAZICI_KAPALI
	case strings.Contains(metin, "paused"):
		return teshis.KUYRUK_DURAKLATILDI
	}
	return ""
}
