//go:build windows

package yazdir

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// CANLI USB yazıcı portlarını kayıt defterinden okur. SALT OKUMA; yönetici
// yetkisi GEREKMEZ.
//
// NEDEN: Windows, USB yazıcı çıkarıldığında kuyruğun portunu ("USB001")
// DEĞİŞTİRMEZ. Kuyruk olduğu gibi durur, spooler işi kabul eder ve iş sessizce
// kaybolur. Gerçekten takılı olan portları HKLM\SYSTEM\CurrentControlSet\Enum\
// USBPRINT altındaki "Device Parameters\PortName" değerlerinden öğreniyoruz.
//
// WOW64_64KEY: 32-bit ajan ikilisi 64-bit Windows'ta çalışırken kayıt
// defterinin 32-bit yansımasına düşmesin diye açıkça 64-bit dal istenir.
//
// KAPALI BAŞARISIZLIK: okuma başarısızsa ya da hiç kayıt yoksa BOŞ küme döner.
// teshis.UsbPortOlu boş kümede ASLA "ölü port" demez — vendor'a özgü port
// monitörü kullanan sürücülerde sahte uyarı çıkmasın.
//
// İKİNCİ DÖNÜŞ (tamListe) 2026-09-22'de eklendi ve KRİTİKTİR: eskiden bir
// cihazın alt anahtarı açılamadığında (standart kullanıcıda ACL yüzünden bu
// çok olağan) o cihaz sessizce atlanıyor, küme EKSİK ama BOŞ OLMADIĞI için
// "kanıt var" sayılıyordu. Sonuç: kablosu takılı, açık ve çalışan fiş
// yazıcısına PORT_OLU deyip işi HİÇ göndermemek. Artık okuma sırasında BİR
// TANE bile hata olursa tamListe=false döner ve çağıran bu kuralı SUSTURUR.
func usbPortlariOkuPlatform() (map[string]string, bool) {
	const yol = `SYSTEM\CurrentControlSet\Enum\USBPRINT`
	const erisim = registry.READ | registry.ENUMERATE_SUB_KEYS | registry.WOW64_64KEY

	out := map[string]string{}
	kok, err := registry.OpenKey(registry.LOCAL_MACHINE, yol, erisim)
	if err != nil {
		return out, false
	}
	defer kok.Close()

	donanimlar, err := kok.ReadSubKeyNames(-1)
	if err != nil {
		return out, false
	}
	tam := true
	for _, donanim := range donanimlar {
		dk, err := registry.OpenKey(kok, donanim, erisim)
		if err != nil {
			tam = false // bu cihazı hiç göremedik → küme eksik olabilir
			continue
		}
		ornekler, err := dk.ReadSubKeyNames(-1)
		if err != nil {
			tam = false
			dk.Close()
			continue
		}
		for _, ornek := range ornekler {
			// CİHAZ ŞU AN TAKILI MI? (2026-09-22, ikinci deneme)
			//
			// Windows, USB yazıcı çıkarıldığında Enum kaydını SİLMEZ — kayıt
			// yıllarca durur. Kaydın varlığını "takılı" saymak, hiç yazıcı
			// olmayan bilgisayarda hayalet "Kur" kartları üretti.
			//
			// İLK deneme uçucu "Control" alt anahtarına bakıyordu; o anahtar
			// bazı kurulumlarda normal kullanıcıya KAPALI çıktı ve kod
			// "emin değilim" deyip HER ŞEYİ susturdu — GERÇEKTEN takılı
			// yazıcı için bile kart çıkmadı (saha: DESKTOP-UNH2P0F, v0.15.2,
			// bulunan_yazicilar=[] ve kart yok).
			//
			// Doğru araç CM_Locate_DevNodeW: Windows'un "bu cihaz ŞU AN
			// takılı mı" sorusunun resmî cevabı. Yönetici yetkisi gerektirmez,
			// kayıt defteri ACL'lerinden etkilenmez. CR_NO_SUCH_DEVNODE =
			// kesin "takılı değil"; başka her cevap belirsizdir ve küme
			// eksik işaretlenir (kapalı başarısızlık korunur).
			takili, kesin := cihazTakiliMi(`USBPRINT\` + donanim + `\` + ornek)
			if !kesin {
				tam = false
				continue
			}
			if !takili {
				continue
			}

			pk, err := registry.OpenKey(dk, ornek+`\Device Parameters`, registry.READ|registry.WOW64_64KEY)
			if err != nil {
				tam = false
				continue
			}
			port, _, err := pk.GetStringValue("PortName")
			pk.Close()
			if err != nil {
				tam = false
				continue
			}
			if port != "" {
				out[port] = donanim
			}
		}
		dk.Close()
	}
	return out, tam
}

// ── CM_Locate_DevNodeW — cihaz ŞU AN takılı mı? ────────────────────────────
//
// cfgmgr32, Tak-Çalıştır yöneticisinin kullanıcı-modu yüzüdür; Aygıt
// Yöneticisi de aynı soruyu böyle sorar. CM_LOCATE_DEVNODE_NORMAL yalnız
// ŞU AN VAR OLAN (takılı + başlatılmış) düğümleri bulur.

var (
	cfgmgrDLL          = syscall.NewLazyDLL("cfgmgr32.dll")
	procLocateDevNodeW = cfgmgrDLL.NewProc("CM_Locate_DevNodeW")
)

const (
	crBasarili      = 0x00 // CR_SUCCESS
	crDugumYok      = 0x0D // CR_NO_SUCH_DEVNODE — cihaz şu an takılı değil
	locateDevNormal = 0x00 // CM_LOCATE_DEVNODE_NORMAL
)

// cihazTakiliMi — kimlik "USBPRINT\<donanım>\<örnek>" biçimindedir.
// kesin=false → cevap belirsiz (DLL yüklenemedi, beklenmedik CR kodu);
// çağıran kümeyi eksik işaretlemeli, ASLA "takılı değil" varsaymamalı.
func cihazTakiliMi(kimlik string) (takili, kesin bool) {
	u, err := windows.UTF16PtrFromString(kimlik)
	if err != nil {
		return false, true // NUL'lu kimlik gerçek bir cihaz olamaz
	}
	var dugum uint32
	r, _, _ := procLocateDevNodeW.Call(
		uintptr(unsafe.Pointer(&dugum)),
		uintptr(unsafe.Pointer(u)),
		locateDevNormal,
	)
	switch r {
	case crBasarili:
		return true, true
	case crDugumYok:
		return false, true
	default:
		return false, false
	}
}
