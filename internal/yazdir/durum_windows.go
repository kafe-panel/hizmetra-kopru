//go:build windows

package yazdir

import (
	"syscall"
	"unsafe"

	"github.com/godoes/printers"
	"golang.org/x/sys/windows"
)

// Windows yazıcı kuyruklarını TEK EnumPrinters çağrısıyla okur.
//
// LEVEL 5 KULLANILIR, level 2 DEĞİL: PRINTER_INFO_5 godoes/printers'ta DIŞA
// AÇIKTIR (PrinterName, PortName, Attributes) — kendi ayna struct'ımızı yazmak
// GEREKMEZ, dolayısıyla 32-bit hizalama riski de YOKTUR. İhtiyacımız olan iki
// kritik bilgi zaten burada:
//   - PortName  → "USB001" / "FILE:" / "nul:" / "IP_192.168.1.50"
//   - Attributes → PRINTER_ATTRIBUTE_WORK_OFFLINE (0x400) bayrağı
//
// Bu ikisi, sessiz hata modlarının büyük çoğunluğunu (sanal hedef, ölü USB
// portu, "çevrimdışı kullan" işareti) ayna struct YAZMADAN kapatır.

func taraPlatform() (map[string]YaziciDurumu, error) {
	const bayraklar = printers.PRINTER_ENUM_LOCAL | printers.PRINTER_ENUM_CONNECTIONS

	var gerekli, donen uint32
	tampon := make([]byte, 1)
	err := printers.EnumPrinters(bayraklar, nil, 5, &tampon[0], uint32(len(tampon)), &gerekli, &donen)
	if err != nil {
		if err != syscall.ERROR_INSUFFICIENT_BUFFER {
			return nil, err
		}
		tampon = make([]byte, gerekli)
		if err = printers.EnumPrinters(bayraklar, nil, 5, &tampon[0], uint32(len(tampon)), &gerekli, &donen); err != nil {
			return nil, err
		}
	}
	out := make(map[string]YaziciDurumu, donen)
	if donen == 0 {
		return out, nil
	}
	girdiler := unsafe.Slice((*printers.PRINTER_INFO_5)(unsafe.Pointer(&tampon[0])), int(donen))
	for _, g := range girdiler {
		ad := windows.UTF16PtrToString(g.PrinterName)
		if ad == "" {
			continue
		}
		out[ad] = YaziciDurumu{
			Ad:                 ad,
			Port:               windows.UTF16PtrToString(g.PortName),
			CevrimdisiIsaretli: g.Attributes&PRINTER_ATTRIBUTE_WORK_OFFLINE != 0,
		}
	}
	return out, nil
}
