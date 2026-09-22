//go:build windows

package yazdir

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/godoes/printers"
	"golang.org/x/sys/windows"
)

// İNCE winspool sarmalayıcı.
//
// NEDEN KENDİ SARMALAYICIMIZ VAR:
//   - godoes/printers'ın StartDocument'i StartDocPrinterW'nin DÖNÜŞ DEĞERİNİ
//     (İŞ KİMLİĞİ) atıyor. O kimlik olmadan "iş kağıda döküldü mü" sorusunu
//     SORAMAYIZ; bugün ajan işi spooler kabul ettiği an 'basildi' diyor ve
//     kağıt çıkmasa bile sunucunun yeniden deneme zinciri HİÇ çalışmıyor.
//   - printers.Open() adı UTF16'ya çevirirken hatayı YUTUYOR ve ad içinde NUL
//     varsa &dizi[0] ifadesinde PANİKLİYOR. Kendi handle'ımızı açarken hatayı
//     kontrol ediyoruz.
//
// Kütüphanenin HAM fonksiyonları (OpenPrinter/ClosePrinter/GetPrinter/SetPrinter/
// EnumPrinters/EnumJobs/WritePrinter/Start-EndPagePrinter/EndDocPrinter) zaten
// dışa açık ve syscall.Handle alıyor — onları doğrudan çağırıyoruz. Yalnız ÜÇ
// eksik proc'u burada tanımlıyoruz.
//
// Kütüphanenin BOZUK SetPrinter2/SetPrinter9 sarmalayıcılarına HİÇ dokunulmaz.
// PRINTER_INFO_2 ayna struct'ı da YAZILMAZ (32-bit hizalaması tek oturumda
// gerçek donanımda doğrulanamaz; yanlış hizalama sessizce YANLIŞ teşhis üretir).

var (
	winspoolDLL = syscall.NewLazyDLL("winspool.drv")

	procStartDocPrinterW             = winspoolDLL.NewProc("StartDocPrinterW")
	procSetJobW                      = winspoolDLL.NewProc("SetJobW")
	procEnumPrintProcessorDatatypesW = winspoolDLL.NewProc("EnumPrintProcessorDatatypesW")
)

// Windows sabitleri (winspool.h) — elle tanımlı.
const (
	PRINTER_ATTRIBUTE_WORK_OFFLINE uint32 = 0x00000400

	PRINTER_CONTROL_RESUME uint32 = 2

	JOB_CONTROL_DELETE uint32 = 5

	PRINTER_ACCESS_ADMINISTER uint32 = 0x00000004
	PRINTER_ACCESS_USE        uint32 = 0x00000008

	// ERROR_INVALID_DATATYPE — spooler veri türünü reddetti (ZJ-80 vakası).
	ERROR_INVALID_DATATYPE syscall.Errno = 1804
	// ERROR_ACCESS_DENIED — yönetici olmayan kullanıcı ayar değiştiremez.
	ERROR_ACCESS_DENIED syscall.Errno = 5
	// ERROR_DISK_FULL — baskı sırası için yer kalmadı.
	ERROR_DISK_FULL syscall.Errno = 112
)

// yaziciAc — verilen erişimle yazıcı handle'ı açar.
//
// printers.Open YERİNE bu kullanılır: ad UTF16'ya çevrilemiyorsa (NUL/kontrol
// karakteri) PANİK yerine düzgün hata döner.
func yaziciAc(ad string, erisim uint32) (syscall.Handle, error) {
	adPtr, err := windows.UTF16PtrFromString(ad)
	if err != nil {
		return 0, fmt.Errorf("yazdir: '%s' adı geçersiz: %w", ad, err)
	}
	var h syscall.Handle
	var varsayilanlar *printers.PrinterDefaults
	if erisim != 0 {
		varsayilanlar = &printers.PrinterDefaults{DesiredAccess: erisim}
	}
	if err := printers.OpenPrinter(adPtr, &h, varsayilanlar); err != nil {
		return 0, err
	}
	return h, nil
}

// belgeBaslat — StartDocPrinterW'yi çağırır ve İŞ KİMLİĞİNİ döndürür.
// 0 dönerse iş oluşmamıştır (hata da döner).
func belgeBaslat(h syscall.Handle, belgeAdi, veriTuru string) (uint32, error) {
	adPtr, err := windows.UTF16PtrFromString(belgeAdi)
	if err != nil {
		return 0, err
	}
	turPtr, err := windows.UTF16PtrFromString(veriTuru)
	if err != nil {
		return 0, err
	}
	bilgi := printers.DOC_INFO_1{DocName: adPtr, Datatype: turPtr}
	r1, _, e1 := syscall.SyscallN(procStartDocPrinterW.Addr(),
		uintptr(h), uintptr(1), uintptr(unsafe.Pointer(&bilgi)))
	if r1 == 0 {
		if e1 != 0 {
			return 0, e1
		}
		return 0, syscall.EINVAL
	}
	return uint32(r1), nil
}

// isiSil — kuyruktaki bir işi siler (JOB_CONTROL_DELETE). YALNIZ kendi
// işlerimiz için çağrılır (bkz. onarim_windows.go).
func isiSil(h syscall.Handle, isKimligi uint32) error {
	r1, _, e1 := syscall.SyscallN(procSetJobW.Addr(),
		uintptr(h), uintptr(isKimligi), 0, 0, uintptr(JOB_CONTROL_DELETE))
	if r1 == 0 {
		if e1 != 0 {
			return e1
		}
		return syscall.EINVAL
	}
	return nil
}

// datatypesInfo1 — EnumPrintProcessorDatatypesW'nin DATATYPES_INFO_1 çıktısı.
// Tek alan (LPWSTR) — hizalama riski yok.
type datatypesInfo1 struct {
	Ad *uint16
}

// veriTurleriniListele — verilen yazdırma işlemcisinin kabul ettiği veri
// türlerini döndürür. Hata durumunda BOŞ liste döner: liste alınamazsa
// merdiven yine RAW → XPS_PASS → TEXT sırasıyla ilerler.
func veriTurleriniListele(islemciAdi string) []string {
	islemciPtr, err := windows.UTF16PtrFromString(islemciAdi)
	if err != nil {
		return nil
	}
	var gerekli, donen uint32
	// İlk çağrı: gereken tampon boyutunu öğren.
	syscall.SyscallN(procEnumPrintProcessorDatatypesW.Addr(),
		0, uintptr(unsafe.Pointer(islemciPtr)), 1, 0, 0,
		uintptr(unsafe.Pointer(&gerekli)), uintptr(unsafe.Pointer(&donen)))
	if gerekli == 0 {
		return nil
	}
	tampon := make([]byte, gerekli)
	r1, _, _ := syscall.SyscallN(procEnumPrintProcessorDatatypesW.Addr(),
		0, uintptr(unsafe.Pointer(islemciPtr)), 1,
		uintptr(unsafe.Pointer(&tampon[0])), uintptr(gerekli),
		uintptr(unsafe.Pointer(&gerekli)), uintptr(unsafe.Pointer(&donen)))
	if r1 == 0 || donen == 0 {
		return nil
	}
	girdiler := unsafe.Slice((*datatypesInfo1)(unsafe.Pointer(&tampon[0])), int(donen))
	out := make([]string, 0, donen)
	for _, g := range girdiler {
		if g.Ad != nil {
			out = append(out, windows.UTF16PtrToString(g.Ad))
		}
	}
	return out
}
