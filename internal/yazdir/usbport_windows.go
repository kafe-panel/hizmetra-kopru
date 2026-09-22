//go:build windows

package yazdir

import (
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
