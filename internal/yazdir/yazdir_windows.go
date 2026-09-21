//go:build windows

package yazdir

import (
	"errors"
	"fmt"

	"github.com/godoes/printers"
)

const spoolerBelgeAdi = "Hizmetra fis"

// spoolerVeriTurleri — spooler'a SIRAYLA denenecek datatype'lar.
//
// "RAW" ÖNCE: ESC/POS baytları sürücünün GDI render'ından geçmeden yazıcıya
// gitmeli. Windows'un standart yazdırma işlemcisi winprint RAW'ı HER ZAMAN
// destekler, yani fiş yazıcılarının ezici çoğunluğu ilk denemede basar.
//
// Kütüphanenin StartRawDocument'ini neden KULLANMIYORUZ (2026-09-21):
// o fonksiyon sürücünün PRINTER_DRIVER_XPS bayrağına bakar ve bayrak set'se
// RAW yerine "XPS_PASS" gönderir. ZIJIANG "ZJ-80 11.3.0.1" gibi bazı v3
// sürücüler bu bayrağı YANLIŞ bildiriyor; kuyruk winprint olduğu için
// XPS_PASS tanınmıyor ve StartDocPrinter 1804 (ERROR_INVALID_DATATYPE —
// "Belirtilen veri türü geçersiz") dönüyor. Sonuç: tek bayt bile yazılmadan
// her iş hataya düşüyor, "Yeniden" de sonsuza kadar aynı hatayı veriyor.
// Canlı vaka: DenizinKafesi / DESKTOP-7D186Q3, ZJ-80 (USB001), işler #2473-#2475.
//
// "XPS_PASS" YEDEK olarak kalıyor: gerçekten v4/XPS tabanlı kuyruklarda
// (ör. Microsoft IPP Class Driver) RAW reddedilir, doğru cevap XPS_PASS'tır.
// Sürücünün ne SÖYLEDİĞİNE değil, spooler'ın ne KABUL ETTİĞİNE bakıyoruz.
var spoolerVeriTurleri = []string{"RAW", "XPS_PASS"}

// spoolerYazPlatform — Windows spooler'a ham iş gönderir.
func spoolerYazPlatform(yaziciAdi string, veri []byte) error {
	if len(veri) == 0 {
		return fmt.Errorf("yazdir: '%s' için boş içerik", yaziciAdi)
	}

	p, err := printers.Open(yaziciAdi)
	if err != nil {
		return fmt.Errorf("yazdir: '%s' açılamadı (Windows'ta kurulu mu?): %w", yaziciAdi, err)
	}
	defer p.Close()

	var reddedilenler []error
	for _, veriTuru := range spoolerVeriTurleri {
		if baslatmaHatasi := p.StartDocument(spoolerBelgeAdi, veriTuru); baslatmaHatasi != nil {
			reddedilenler = append(reddedilenler, fmt.Errorf("%s: %w", veriTuru, baslatmaHatasi))
			continue
		}
		// Belge açıldı → datatype kabul edildi. Bundan sonraki her hata GERÇEK
		// baskı hatasıdır (kağıt, kapak, kablo); başka veri türü DENENMEZ.
		return spoolerGovdeYaz(p, yaziciAdi, veri)
	}

	return fmt.Errorf(
		"yazdir: '%s' hiçbir veri türünü kabul etmedi — yazıcının Windows sürücüsü/yazdırma işlemcisi ham baskıya kapalı: %w",
		yaziciAdi, errors.Join(reddedilenler...))
}

// spoolerGovdeYaz — açılmış belgeye baytları yazar ve belgeyi kapatır.
//
// EndDocument hatası YUTULMAZ: işi spooler'a asıl teslim eden çağrı odur,
// başarısız olursa iş 'basildi' diye bildirilmemeli.
func spoolerGovdeYaz(p *printers.Printer, yaziciAdi string, veri []byte) (err error) {
	defer func() {
		bitisHatasi := p.EndDocument()
		if err == nil && bitisHatasi != nil {
			err = fmt.Errorf("yazdir: '%s' belge kapatılamadı: %w", yaziciAdi, bitisHatasi)
		}
	}()

	if sayfaHatasi := p.StartPage(); sayfaHatasi != nil {
		return fmt.Errorf("yazdir: '%s' sayfa başlatılamadı: %w", yaziciAdi, sayfaHatasi)
	}
	if _, yazmaHatasi := p.Write(veri); yazmaHatasi != nil {
		return fmt.Errorf("yazdir: '%s' yazılamadı: %w", yaziciAdi, yazmaHatasi)
	}
	if kapatmaHatasi := p.EndPage(); kapatmaHatasi != nil {
		return fmt.Errorf("yazdir: '%s' sayfa kapatılamadı: %w", yaziciAdi, kapatmaHatasi)
	}
	return nil
}
