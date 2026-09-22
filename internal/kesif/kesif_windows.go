//go:build windows

package kesif

import (
	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/yazdir"
)

// bulPlatform — Windows'ta kurulu yazıcıları listeler.
//
// printers.ReadNames YERİNE paylaşımlı önbellek (yazdir.YazicilariOku)
// kullanılır: aynı EnumPrinters taraması hem baskı ön kontrolü hem durum
// penceresi hem de nabız tarafından okunur (eskiden durum penceresi 3 saniyede
// bir ayrı tarama yapıyordu).
func bulPlatform() ([]api.Yazici, error) {
	yazicilar, err := yazdir.YazicilariOku()
	if err != nil {
		return nil, err
	}
	sonuc := make([]api.Yazici, 0, len(yazicilar))
	for _, d := range yazicilar {
		durum, uyari := durumBelirle(d)
		sonuc = append(sonuc, api.Yazici{
			Ad:     d.Ad,
			Hedef:  d.Ad, // Windows'ta hedef = yazıcı adı
			Tip:    "windows",
			Durum:  durum,
			Port:   d.Port,
			Surucu: d.Surucu,
			Uyari:  uyari,
		})
	}
	return sonuc, nil
}
