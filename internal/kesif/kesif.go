// Package kesif — bilgisayardaki yazıcıları bulur (nabızla sunucuya bildirilir).
//
// Sunucu bunları "Bulunan Yazıcılar" listesinde gösterir; kullanıcı panelden
// seçer, hedef (Windows yazıcı adı) Yazici.kopru_hedef'e yazılır. Böylece
// kullanıcı hiçbir ID/IP ezberlemez.
package kesif

import (
	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// Durum değerleri — PROTOKOL DONMUŞ: api.Yazici.Durum YALNIZ bu iki değeri alır.
const (
	DurumCevrimici  = "online"
	DurumCevrimdisi = "offline"
)

// Bul — sistemdeki yazıcıları listeler. Hata durumunda BOŞ liste + hata döner
// (çağıran nabzı yine atar; keşif başarısızlığı ajanı durdurmaz).
func Bul() ([]api.Yazici, error) {
	return bulPlatform()
}

// durumBelirle — SABİT "online" YALANINI bitiren saf fonksiyon.
//
// Eskiden keşif her yazıcıya koşulsuz "online" diyordu; panelde yeşil görünen
// bir yazıcı fiilen çevrimdışı işaretliyken fiş basmıyordu. Şema DEĞİŞMEDİ,
// yalnız DEĞER artık doğru.
func durumBelirle(d yazdir.YaziciDurumu) (durum, uyari string) {
	if d.CevrimdisiIsaretli || d.Duraklatildi {
		return DurumCevrimdisi, teshis.Cumle(teshis.CEVRIMDISI_ISARETLI, d.Ad, "")
	}
	switch teshis.PortSinifi(d.Port) {
	case teshis.PortDosya:
		return DurumCevrimdisi, teshis.Cumle(teshis.SANAL_HEDEF, d.Ad, "")
	case teshis.PortUSB:
		// Canlı port kümesi EKSİK olabiliyorsa (tamListe=false) bu kural susar:
		// eksik kümeyle "USB girişi boşta" demek, takılı ve çalışan yazıcıyı
		// panelde çevrimdışı göstermek olurdu.
		canliPortlar, tamListe := yazdir.CanliUsbPortlari()
		if !tamListe {
			return DurumCevrimici, ""
		}
		if olu, kanitVar := teshis.UsbPortOlu(d.Port, canliPortlar); kanitVar && olu {
			return DurumCevrimdisi, teshis.Cumle(teshis.PORT_OLU, d.Ad, "")
		}
	}
	return DurumCevrimici, ""
}
