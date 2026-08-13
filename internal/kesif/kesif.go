// Package kesif — bilgisayardaki yazıcıları bulur (nabızla sunucuya bildirilir).
//
// Sunucu bunları "Bulunan Yazıcılar" listesinde gösterir; kullanıcı panelden
// seçer, hedef (Windows yazıcı adı) Yazici.kopru_hedef'e yazılır. Böylece
// kullanıcı hiçbir ID/IP ezberlemez.
package kesif

import "github.com/kafe-panel/hizmetra-kopru/internal/api"

// Bul — sistemdeki yazıcıları listeler. Hata durumunda BOŞ liste + hata döner
// (çağıran nabzı yine atar; keşif başarısızlığı ajanı durdurmaz).
func Bul() ([]api.Yazici, error) {
	return bulPlatform()
}
