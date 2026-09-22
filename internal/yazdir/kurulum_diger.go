//go:build !windows

package yazdir

import "errors"

// kurPlatform — Windows dışında yazıcı kuyruğu AÇMIYORUZ.
//
// macOS/Linux'ta kuyruk açmak CUPS'a lpadmin ile yazıcı eklemek demektir;
// orada USB fiş yazıcıları zaten sistem tarafından tanınıyor ve sahadaki
// kurulumların tamamı Windows. Sessizce "oldu" demek yerine açıkça reddediyoruz.
func kurPlatform(ad, port string) error {
	return errors.New("yazıcı kurulumu yalnız Windows'ta destekleniyor")
}
