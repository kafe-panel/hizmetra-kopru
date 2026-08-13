//go:build windows

package yazdir

import (
	"fmt"

	"github.com/godoes/printers"
)

// spoolerYazPlatform — Windows spooler'a RAW iş gönderir.
//
// RAW datatype KRİTİK: sürücünün GDI render'ını bypass eder, baytlar yazıcıya
// olduğu gibi gider. Aksi halde ESC/POS komutları "metin" sanılıp bozulur.
func spoolerYazPlatform(yaziciAdi string, veri []byte) error {
	p, err := printers.Open(yaziciAdi)
	if err != nil {
		return fmt.Errorf("yazdir: '%s' açılamadı (Windows'ta kurulu mu?): %w", yaziciAdi, err)
	}
	defer p.Close()

	if err := p.StartRawDocument("Hizmetra fis"); err != nil {
		return fmt.Errorf("yazdir: '%s' RAW belge başlatılamadı: %w", yaziciAdi, err)
	}
	defer p.EndDocument()

	if err := p.StartPage(); err != nil {
		return fmt.Errorf("yazdir: '%s' sayfa başlatılamadı: %w", yaziciAdi, err)
	}
	if _, err := p.Write(veri); err != nil {
		return fmt.Errorf("yazdir: '%s' yazılamadı: %w", yaziciAdi, err)
	}
	if err := p.EndPage(); err != nil {
		return fmt.Errorf("yazdir: '%s' sayfa kapatılamadı: %w", yaziciAdi, err)
	}
	return nil
}
