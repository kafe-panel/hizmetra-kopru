//go:build windows

package kesif

import (
	"github.com/godoes/printers"
	"github.com/kafe-panel/hizmetra-kopru/internal/api"
)

// bulPlatform — Windows'ta kurulu yazıcıları listeler.
//
// Durum ("online"/"offline") burada BEST-EFFORT: spooler yazıcının fiziksel
// açık olduğunu güvenilir söylemez. Gerçek kanıt baskı sonucudur — bu yüzden
// sunucu 'basildi'yi ajanın sonuç bildiriminden öğrenir, bu listeden DEĞİL.
func bulPlatform() ([]api.Yazici, error) {
	adlar, err := printers.ReadNames()
	if err != nil {
		return nil, err
	}
	sonuc := make([]api.Yazici, 0, len(adlar))
	for _, ad := range adlar {
		sonuc = append(sonuc, api.Yazici{
			Ad:    ad,
			Hedef: ad, // Windows'ta hedef = yazıcı adı
			Tip:   "windows",
			Durum: "online",
		})
	}
	return sonuc, nil
}
