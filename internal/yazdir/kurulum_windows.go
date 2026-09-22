//go:build windows

package yazdir

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/godoes/printers"
	"golang.org/x/sys/windows"
)

// TAKILI AMA KURULMAMIŞ YAZICI İÇİN KUYRUK AÇMA (Windows).
//
// Kafe sahibi USB kablosunu takar, Windows sürücüyü tanımaz ve cihaz
// "Belirtilmemiş" bölümünde öylece kalır. Panelde hiçbir şey görünmez.
// Burada o cihaz için AddPrinterW ile bir kuyruk açıyoruz.
//
// NEDEN "Generic / Text Only": Windows'un KUTUDAN ÇIKAN sürücüsüdür, her
// kurulumda vardır, indirme gerektirmez ve ham (RAW) baskıda sürücü zaten
// hiçbir dönüşüm yapmaz — ESC/POS baytları olduğu gibi yazıcıya gider.
// Üreticinin kendi sürücüsünü KULLANMIYORUZ: sahada ZIJIANG "ZJ-80 11.3.0.1"
// sürücüsü PRINTER_DRIVER_XPS bayrağını yanlış bildirip her fişi öldürdü.
//
// PRINTER_INFO_2 burada OKUNMUYOR, YALNIZ YAZILIYOR. Ayna struct'ı okuma
// tarafında riskliydi (OS'in doldurduğu tamponu yanlış hizalamayla okumak
// sessizce çöp üretir); yazma tarafında struct'ı BİZ kuruyoruz ve hizalama
// yanlışsa AddPrinterW hata döner — sessiz bozulma değil, görünür hata.
var (
	procAddPrinterW         = winspoolDLL.NewProc("AddPrinterW")
	procEnumPrinterDriversW = winspoolDLL.NewProc("EnumPrinterDriversW")
)

const (
	printerAttributeLocal uint32 = 0x00000040
	// ERROR_PRINTER_ALREADY_EXISTS — aynı adda kuyruk zaten var.
	errPrinterAlreadyExists syscall.Errno = 1802
	// ERROR_UNKNOWN_PORT — verilen port spooler'da tanımlı değil.
	errUnknownPort syscall.Errno = 1796
)

// printerInfo2Yaz — AddPrinterW'ye verilecek PRINTER_INFO_2W.
// Alan SIRASI winspool.h ile BİREBİR aynı olmalı (hepsi işaretçi, sonra DWORD'ler).
type printerInfo2Yaz struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            uintptr
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor uintptr
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	CJobs              uint32
	AveragePPM         uint32
}

// kurulumSurucuAdaylari — sırayla denenecek sürücüler. İlki her Windows'ta vardır.
var kurulumSurucuAdaylari = []string{"Generic / Text Only", "Generic/Text Only"}

// kurPlatform — port'a bakan yeni bir yazıcı kuyruğu açar.
func kurPlatform(ad, port string) error {
	surucu, err := kurulumSurucusuSec()
	if err != nil {
		return err
	}

	adU, err := windows.UTF16PtrFromString(ad)
	if err != nil {
		return fmt.Errorf("yazıcı adı kullanılamaz: %w", err)
	}
	portU, err := windows.UTF16PtrFromString(port)
	if err != nil {
		return fmt.Errorf("port adı kullanılamaz: %w", err)
	}
	surucuU, err := windows.UTF16PtrFromString(surucu)
	if err != nil {
		return fmt.Errorf("sürücü adı kullanılamaz: %w", err)
	}
	// winprint + RAW: ESC/POS baytları sürücü dönüşümünden GEÇMEDEN gitsin.
	islemciU, _ := windows.UTF16PtrFromString("winprint")
	veriTuruU, _ := windows.UTF16PtrFromString("RAW")

	bilgi := printerInfo2Yaz{
		PrinterName:    adU,
		PortName:       portU,
		DriverName:     surucuU,
		PrintProcessor: islemciU,
		Datatype:       veriTuruU,
		Attributes:     printerAttributeLocal,
	}

	h, _, errno := procAddPrinterW.Call(0, 2, uintptr(unsafe.Pointer(&bilgi)))
	if h == 0 {
		switch errno {
		case errPrinterAlreadyExists:
			return fmt.Errorf("%q adında bir yazıcı zaten var", ad)
		case errUnknownPort:
			return fmt.Errorf("%s girişi Windows'ta tanımlı değil — kabloyu çıkarıp yeniden takın", port)
		case ERROR_ACCESS_DENIED:
			return fmt.Errorf("yazıcı kurmak için yönetici yetkisi gerekiyor")
		default:
			return fmt.Errorf("yazıcı kurulamadı: %w", errno)
		}
	}
	_ = printers.ClosePrinter(syscall.Handle(h))

	// Önbellek bayatlamasın: yeni kuyruk hemen görünsün.
	OnbellegiTemizle()
	return nil
}

// kurulumSurucusuSec — kurulu sürücüler arasından ham baskıya uygun olanı seçer.
func kurulumSurucusuSec() (string, error) {
	kurulu, err := surucuAdlariniOku()
	if err != nil {
		// Okuyamadıysak yine de en yaygın adı deneriz; AddPrinterW hata verirse
		// kullanıcı net mesajı zaten görür.
		return kurulumSurucuAdaylari[0], nil
	}
	for _, aday := range kurulumSurucuAdaylari {
		for _, k := range kurulu {
			if strings.EqualFold(strings.TrimSpace(k), aday) {
				return k, nil
			}
		}
	}
	return "", fmt.Errorf("Windows'un \"Generic / Text Only\" sürücüsü bu bilgisayarda bulunamadı")
}

// surucuAdlariniOku — EnumPrinterDriversW level 1 (yalnız ad). Yönetici gerekmez.
func surucuAdlariniOku() ([]string, error) {
	var gerekli, adet uint32
	// İlk çağrı boyutu ölçer; ERROR_INSUFFICIENT_BUFFER beklenen cevaptır.
	procEnumPrinterDriversW.Call(0, 0, 1, 0, 0,
		uintptr(unsafe.Pointer(&gerekli)), uintptr(unsafe.Pointer(&adet)))
	if gerekli == 0 {
		return nil, fmt.Errorf("sürücü listesi okunamadı")
	}
	tampon := make([]byte, gerekli)
	ok, _, errno := procEnumPrinterDriversW.Call(0, 0, 1,
		uintptr(unsafe.Pointer(&tampon[0])), uintptr(gerekli),
		uintptr(unsafe.Pointer(&gerekli)), uintptr(unsafe.Pointer(&adet)))
	if ok == 0 {
		return nil, fmt.Errorf("sürücü listesi okunamadı: %w", errno)
	}
	// DRIVER_INFO_1 = { LPWSTR pName } → tamponda ardışık işaretçiler.
	const isaretciBoyu = unsafe.Sizeof(uintptr(0))
	if uintptr(adet)*isaretciBoyu > uintptr(len(tampon)) {
		return nil, fmt.Errorf("sürücü listesi tutarsız")
	}
	out := make([]string, 0, adet)
	for i := uint32(0); i < adet; i++ {
		p := *(**uint16)(unsafe.Pointer(&tampon[uintptr(i)*isaretciBoyu]))
		if p == nil {
			continue
		}
		out = append(out, windows.UTF16PtrToString(p))
	}
	return out, nil
}
