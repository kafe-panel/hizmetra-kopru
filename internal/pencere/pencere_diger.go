//go:build !windows

// Package pencere — Windows dışı derlemede no-op (CI/geliştirme). WebView2
// yalnız Windows'ta anlamlı; ajan Windows hedeflidir (platform_diger.go ile
// aynı desen: main.go bir hata görür, tarayicidaAc() sistem tarayıcısına düşer).
package pencere

import "errors"

// Ac — bkz. pencere_windows.go. Windows dışında her zaman hata döner; çağıran
// bunu tarayıcı düşüşü sinyali olarak kullanır.
func Ac(url string, genislik, yukseklik int) error {
	return errors.New("pencere: yalnız Windows'ta desteklenir")
}

// OneGetir — bkz. pencere_windows.go.
func OneGetir() error {
	return errors.New("pencere: yalnız Windows'ta desteklenir")
}

// Kapat — bkz. pencere_windows.go.
func Kapat() {}
