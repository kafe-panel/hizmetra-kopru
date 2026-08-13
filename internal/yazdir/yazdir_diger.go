//go:build !windows

package yazdir

import "errors"

// spoolerYazPlatform — Windows dışında spooler yok. Ajan Windows hedefli;
// bu stub yalnız geliştirme/test derlemesinin (linux CI) geçmesi için var.
// Ağ yazıcıları (ip:9100) her platformda ÇALIŞIR.
func spoolerYazPlatform(yaziciAdi string, veri []byte) error {
	return errors.New("yazdir: yerel yazıcıya baskı yalnız Windows'ta desteklenir (ağ yazıcısı için 'ip:9100' hedefi kullanın)")
}
