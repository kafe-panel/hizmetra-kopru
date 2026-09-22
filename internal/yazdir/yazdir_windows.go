//go:build windows

package yazdir

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/godoes/printers"
	"golang.org/x/sys/windows"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// ÇEKİRDEK: "spooler'a verdim" YALANINI bitiren dosya.
//
// ESKİ DAVRANIŞ (ve canlı hata): EndDocPrinter nil dönünce iş 'basildi'
// bildiriliyordu. Oysa EndDocPrinter yalnız "spooler işi KABUL ETTİ" demektir;
// kağıt bitmiş, kapak açık, kuyruk duraklatılmış, yazıcı "çevrimdışı kullan"
// işaretli veya port ölü olsa bile aynı nil döner. Sunucu 'basildi' duyduğu için
// 2 dakikalık yeniden deneme zinciri HİÇ çalışmıyordu: fiş sessizce kayboluyordu.
//
// YENİ DAVRANIŞ üç katman:
//  1. ÖN KAPI — yalnız yanlış-pozitifi neredeyse imkânsız sinyallerde işi HİÇ
//     göndermeyiz (kuyruk yok, sanal/dosya portu, kanıtlı ölü USB portu).
//  2. KENDİ İŞ KİMLİĞİMİZ — StartDocPrinterW'nin dönüşünü saklarız.
//  3. TESLİM TEYİDİ — EnumJobs ile işi izleriz; karar tablosu teshis pakette
//     (saf + macOS'ta testli).

const spoolerBelgeAdiOnek = "Hizmetra fis"

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
// XPS_PASS tanınmıyor ve StartDocPrinter 1804 (ERROR_INVALID_DATATYPE) dönüyor.
// Canlı vaka: DenizinKafesi / DESKTOP-7D186Q3, ZJ-80 (USB001), işler #2473-#2475.
//
// "TEXT" en sonda: fiş içeriğini bozar ama hiç basmamaktan iyidir (son çare).
var spoolerVeriTurleri = []string{"RAW", "XPS_PASS", "TEXT"}

// kazananTurler — yazıcı adı → daha önce KABUL EDİLMİŞ veri türü.
//
// Neden ayar.json'a değil belleğe yazıyoruz: config dosyasını main süreci
// kendi alanları için yazıyor; buradan da yazmak iki tarafın birbirinin
// değişikliğini ezmesi demek olurdu (token kaybı riski). Süreç ömrü boyunca
// hatırlamak "her işte veri türü merdivenini baştan tırman" gecikmesini zaten
// kaldırıyor.
var (
	kazananKilit  sync.Mutex
	kazananTurler = map[string]string{}
)

func kazananTur(ad string) string {
	kazananKilit.Lock()
	defer kazananKilit.Unlock()
	return kazananTurler[ad]
}

func kazananTurYaz(ad, tur string) {
	kazananKilit.Lock()
	kazananTurler[ad] = tur
	kazananKilit.Unlock()
}

// spoolerYazPlatform — Windows spooler'a ham iş gönderir ve TESLİMİ DOĞRULAR.
func spoolerYazPlatform(yaziciAdi string, veri []byte) error {
	if len(veri) == 0 {
		return teshis.Yeni(teshis.BILINMEYEN, yaziciAdi, "boş fiş içeriği", nil)
	}
	yaziciAdi, err := onKapi(yaziciAdi)
	if err != nil {
		return err
	}

	h, err := yaziciAc(yaziciAdi, PRINTER_ACCESS_USE)
	if err != nil {
		return teshis.Yeni(teshis.HEDEF_YOK, yaziciAdi, "", err)
	}
	defer printers.ClosePrinter(h) //nolint:errcheck

	isKimligi, kullanilanTur, err := belgeAcVeriTuruMerdiveni(h, yaziciAdi)
	if err != nil {
		return err
	}
	kazananTurYaz(yaziciAdi, kullanilanTur)

	yazmaHatasi := govdeYaz(h, yaziciAdi, veri)
	bitisHatasi := printers.EndDocPrinter(h)

	if yazmaHatasi != nil {
		return yazmaHatasi
	}
	// EndDocPrinter HATA verse bile EnumJobs'a soruyoruz: iş kuyrukta duruyorsa
	// spooler onu ALMIŞTIR ve birazdan basacaktır. Hata deyip yeniden göndermek
	// ÇİFT FİŞ üretirdi.
	return teslimDogrula(h, yaziciAdi, isKimligi, bitisHatasi)
}

// onKapi — baskıdan ÖNCE, YALNIZ yanlış-pozitifi neredeyse imkânsız sinyallerde
// işi durdurur. Emin olmadığımız hiçbir şeyde durdurmayız: yanlış yere "basma"
// demek, sessizce basmamaktan da kötüdür.
//
// Dönen ilk değer KULLANILACAK kuyruk adıdır: liste taramasında hedefin harf
// duyarsız karşılığı bulunduysa kuyruğun GERÇEK adı döner (panelde "POS-80"
// yazıp kuyruğun "Pos-80" olması Windows'ta gayet normaldir).
//
// KAPI SUSAR KURALI (2026-09-22): hedef listede hiç bulunamazsa kapı HİÇBİR
// ŞEY SÖYLEMEZ, hedef olduğu gibi geri verilir ve karar OpenPrinter'a bırakılır.
// Eski kod burada HEDEF_YOK üretip işi hiç göndermiyordu; oysa EnumPrinters
// yalnız yerel + bağlı kuyrukları listeler — "\\SUNUCU\Kasa" gibi UNC
// paylaşımlar listede görünmez ama OpenPrinter ile pekâlâ açılır. HEDEF_YOK
// artık YALNIZ OpenPrinter da başarısız olduğunda üretilir.
func onKapi(yaziciAdi string) (string, error) {
	yazicilar, err := YazicilariOku()
	if err != nil || len(yazicilar) == 0 {
		// Tarama yapılamadı → kapı KONUŞMAZ, baskı normal denenir.
		return yaziciAdi, nil
	}
	gercekAd, durum, varmi := kuyrukBul(yazicilar, yaziciAdi)
	if !varmi {
		gunluk.YazSessiz("'%s' kuyruk listesinde yok (bulunanlar: %s) — yine de doğrudan açmayı deniyoruz",
			yaziciAdi, adayListesi(yazicilar))
		return yaziciAdi, nil
	}
	if gercekAd != yaziciAdi {
		gunluk.YazSessiz("hedef '%s' kuyruk adıyla birebir aynı değil; '%s' kullanılıyor", yaziciAdi, gercekAd)
	}
	switch teshis.PortSinifi(durum.Port) {
	case teshis.PortDosya:
		// En sinsi mod: "nul:" portunda iş anında 'basildi' olur ve buharlaşır.
		return gercekAd, teshis.Yeni(teshis.SANAL_HEDEF, gercekAd, "çıkış: "+durum.Port, nil)
	case teshis.PortUSB:
		canliPortlar, tamListe := usbPortlariOkuPlatform()
		// tamListe=false → tarama SIRASINDA en az bir hata oldu, küme EKSİK
		// olabilir. Eksik kümeyle "port ölü" demek, takılı ve çalışan bir
		// yazıcıya hiç iş göndermemek demektir (bkz. usbport_windows.go).
		if tamListe {
			if olu, kanitVar := teshis.UsbPortOlu(durum.Port, canliPortlar); kanitVar && olu {
				return gercekAd, teshis.Yeni(teshis.PORT_OLU, gercekAd, durum.Port+" boşta", nil)
			}
		}
	}
	if durum.CevrimdisiIsaretli {
		// GERİ ALINABİLİR onarımı dene, sonra baskıya DEVAM ET: onarım
		// başarısız olsa bile işi göndermeyi deneriz (belki yine de basar).
		sonuc := Onar(gercekAd, teshis.CEVRIMDISI_ISARETLI)
		if sonuc.Yapildi {
			gunluk.Yaz("'%s' çevrimdışı işareti kaldırıldı, baskı deneniyor", gercekAd)
		}
	}
	return gercekAd, nil
}

// kuyrukBul — hedefi önce BİREBİR, tutmazsa HARF DUYARSIZ (ve baştaki/sondaki
// boşluğu yok sayarak) arar. Windows'un kendi OpenPrinter'ı da böyle davranır;
// Go map'i ise duyarlıdır ve eskiden bu fark yüzünden dün basan yazıcı
// "bu bilgisayarda yok" sayılıyordu.
func kuyrukBul(yazicilar map[string]YaziciDurumu, hedef string) (string, YaziciDurumu, bool) {
	if durum, varmi := yazicilar[hedef]; varmi {
		return hedef, durum, true
	}
	// İkinci tur: harf duyarsız eşleşme. Birden fazla aday çıkarsa (ör. "Kasa"
	// ve "kasa") adı sıralayıp İLKİNİ seçeriz — seçim en azından belirlidir.
	adaylar := make([]string, 0, 2)
	for ad := range yazicilar {
		if teshis.AdEsitMi(ad, hedef) {
			adaylar = append(adaylar, ad)
		}
	}
	if len(adaylar) == 0 {
		return hedef, YaziciDurumu{}, false
	}
	sort.Strings(adaylar)
	return adaylar[0], yazicilar[adaylar[0]], true
}

// adayListesi — kullanıcıya "hangi yazıcılar var" demek için kısa liste.
func adayListesi(yazicilar map[string]YaziciDurumu) string {
	adlar := make([]string, 0, len(yazicilar))
	for ad := range yazicilar {
		adlar = append(adlar, ad)
	}
	sort.Strings(adlar)
	if len(adlar) > 3 {
		adlar = adlar[:3]
	}
	return strings.Join(adlar, ", ")
}

// belgeAcVeriTuruMerdiveni — kabul edilen İLK veri türüyle belge açar ve
// İŞ KİMLİĞİNİ döndürür. Belge adı is takibi için önekli yazılır.
func belgeAcVeriTuruMerdiveni(h syscall.Handle, yaziciAdi string) (uint32, string, error) {
	// Belge adı ÖNEKLİ yazılır: onarım tarafı "kendi ölü işlerimi sil" filtresini
	// bu ÖNEK eşleşmesiyle yapar ve BAŞKASININ belgesine asla dokunmaz. Saat
	// damgası günlükle eşleştirmeyi kolaylaştırır.
	belgeAdi := spoolerBelgeAdiOnek + " " + time.Now().Format("15:04:05")
	var reddedilenler []error
	for _, tur := range veriTuruSirasi(yaziciAdi) {
		isKimligi, err := belgeBaslat(h, belgeAdi, tur)
		if err == nil {
			return isKimligi, tur, nil
		}
		reddedilenler = append(reddedilenler, fmt.Errorf("%s: %w", tur, err))
	}
	// 1804 (ERROR_INVALID_DATATYPE) → sürücü/işlemci ham baskıya kapalı.
	kod := teshis.BILINMEYEN
	for _, r := range reddedilenler {
		var errno syscall.Errno
		if errors.As(r, &errno) {
			switch errno {
			case ERROR_INVALID_DATATYPE:
				kod = teshis.VERI_TURU_RED
			case ERROR_ACCESS_DENIED:
				kod = teshis.YETKI_YOK
			case ERROR_DISK_FULL:
				kod = teshis.DISK_DOLU
			}
		}
	}
	if kod == teshis.BILINMEYEN {
		kod = teshis.VERI_TURU_RED
	}
	return 0, "", teshis.Yeni(kod, yaziciAdi, "", errors.Join(reddedilenler...))
}

// veriTuruSirasi — daha önce kabul edilen tür varsa ÖNCE o denenir; sonra
// yazdırma işlemcisinin gerçekten desteklediği türler; sonra sabit merdiven.
func veriTuruSirasi(yaziciAdi string) []string {
	sira := make([]string, 0, 4)
	ekle := func(t string) {
		if t == "" {
			return
		}
		for _, v := range sira {
			if v == t {
				return
			}
		}
		sira = append(sira, t)
	}
	ekle(kazananTur(yaziciAdi))
	// winprint, Windows'un standart yazdırma işlemcisidir; fiş kuyruklarının
	// neredeyse tamamı onu kullanır. Liste alınamazsa sabit merdivene düşeriz.
	for _, t := range veriTurleriniListele("winprint") {
		if t == "RAW" || t == "XPS_PASS" || t == "TEXT" {
			ekle(t)
		}
	}
	for _, t := range spoolerVeriTurleri {
		ekle(t)
	}
	return sira
}

// govdeYaz — açılmış belgeye baytları OFFSET DÖNGÜSÜYLE yazar.
//
// Tek bir yazma çağrısı YETMEZ: WritePrinter kısmi yazabilir. Eskiden
// dönen bayt sayısı kontrol edilmiyordu → yarım fiş "başarılı" sayılıyordu.
// Üç turda hiç ilerleme yoksa YARIM_YAZILDI verilir ve iş KÖR YENİDEN
// DENENMEZ (yarım + tam fiş = çift fiş).
func govdeYaz(h syscall.Handle, yaziciAdi string, veri []byte) error {
	if err := printers.StartPagePrinter(h); err != nil {
		return teshis.Yeni(teshis.BILINMEYEN, yaziciAdi, "sayfa başlatılamadı", err)
	}
	yazilan := 0
	ilerlemeyenTur := 0
	for yazilan < len(veri) {
		parca := veri[yazilan:]
		var n uint32
		err := printers.WritePrinter(h, &parca[0], uint32(len(parca)), &n)
		if err != nil {
			if yazilan == 0 {
				// Hiç bayt gitmedi → yeniden denemek GÜVENLİ.
				return teshis.Yeni(yazmaHataKodu(err), yaziciAdi, "", err)
			}
			return teshis.Yeni(teshis.YARIM_YAZILDI, yaziciAdi, "", err)
		}
		if n == 0 {
			ilerlemeyenTur++
			if ilerlemeyenTur >= 3 {
				if yazilan == 0 {
					return teshis.Yeni(teshis.ASILDI, yaziciAdi, "", nil)
				}
				return teshis.Yeni(teshis.YARIM_YAZILDI, yaziciAdi, "", nil)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		ilerlemeyenTur = 0
		yazilan += int(n)
	}
	if err := printers.EndPagePrinter(h); err != nil {
		return teshis.Yeni(teshis.BILINMEYEN, yaziciAdi, "sayfa kapatılamadı", err)
	}
	return nil
}

func yazmaHataKodu(err error) teshis.Kod {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case ERROR_ACCESS_DENIED:
			return teshis.YETKI_YOK
		case ERROR_DISK_FULL:
			return teshis.DISK_DOLU
		}
	}
	return teshis.BILINMEYEN
}

// teslimYoklamaAraliklari — artan aralıklarla toplam ~4,2 sn (üst sınır 3 sn'lik
// pencere; son aralık taşarsa döngü zaten çıkar).
var teslimYoklamaAraliklari = []time.Duration{
	150 * time.Millisecond, 300 * time.Millisecond, 500 * time.Millisecond,
	750 * time.Millisecond, 1000 * time.Millisecond, 1500 * time.Millisecond,
}

// teslimDogrula — işi kuyrukta izler ve GERÇEK sonucu döndürür.
func teslimDogrula(h syscall.Handle, yaziciAdi string, isKimligi uint32, bitisHatasi error) error {
	if isKimligi == 0 {
		// Kimlik yoksa izleyemeyiz. EndDocPrinter hatası varsa hata, yoksa belirsiz.
		if bitisHatasi != nil {
			return teshis.Yeni(teshis.BILINMEYEN, yaziciAdi, "belge kapatılamadı", bitisHatasi)
		}
		gunluk.YazSessiz("'%s': iş kimliği alınamadı — cihaz onayı yok", yaziciAdi)
		return nil
	}

	bitis := time.Now().Add(3 * time.Second)
	var sonBitler uint32
	var sonSayfa uint32
	// onarilanlar — bu iş için hangi sorunu ZATEN onardık. Her sorun için tek
	// deneme: onarım tutmadıysa ikinci kez uğraşmak kullanıcının ayarlarıyla
	// oynamak olur.
	onarilanlar := map[teshis.Kod]bool{}
	turOfseti := 0
	for tur := 0; ; tur++ {
		listedeVar, bitler, sayfa, listeDoldu, err := isiBul(h, isKimligi)
		if err != nil {
			// Kuyruk okunamıyor: EndDocPrinter temizse işi teslim sayarız ama
			// günlüğe not düşeriz (yalan söylemiyoruz, kanıt yok diyoruz).
			gunluk.YazSessiz("'%s': baskı sırası okunamadı (%v) — cihaz onayı yok", yaziciAdi, err)
			if bitisHatasi != nil {
				return teshis.Yeni(teshis.BILINMEYEN, yaziciAdi, "belge kapatılamadı", bitisHatasi)
			}
			return nil
		}
		karar, kod := teshis.TeslimKarari(listedeVar, bitler, listeDoldu)
		switch karar {
		case teshis.KararBasarili:
			return nil
		case teshis.KararHata:
			// ÇİFT FİŞ TUZAĞI (2026-09-22'de kapatıldı): duraklatılmış kuyruğu
			// onarıp (RESUME) sonra AYNI iş için 'hata' döndürmek en kötü
			// bileşimdi — bizim iş kuyruktan SİLİNMİYOR, devam ettirilen kuyruk
			// onu basıyor, sunucu ise 'hata' duyduğu için 2 dakika sonra aynı
			// fişi yeniden veriyordu: mutfağa iki fiş.
			//
			// Doğrusu: onarım TUTTUYSA iş artık "bekliyor" sayılır ve teslim
			// doğrulamaya DEVAM edilir. Onarım tutmazsa (yetki yok, devre
			// kesici dolu, defter yok) eskisi gibi hata döneriz — o zaman iş
			// gerçekten basılmıyor ve yeniden verilmesi DOĞRU.
			if (kod == teshis.KUYRUK_DURAKLATILDI || kod == teshis.CEVRIMDISI_ISARETLI) && !onarilanlar[kod] {
				onarilanlar[kod] = true
				if sonuc := Onar(yaziciAdi, kod); sonuc.Yapildi {
					gunluk.Yaz("'%s': %s onarıldı, fiş kuyrukta basılmayı bekliyor — yeniden gönderilmeyecek",
						yaziciAdi, onarimIslemAdi(kod))
					// Onarımdan sonra kuyruğun işi alıp basması zaman alır:
					// yoklama penceresini ve tur sayacını bir kez sıfırlıyoruz,
					// sonra AŞAĞIDAKİ normal yoklama akışı devam ediyor.
					bitis = bitis.Add(3 * time.Second)
					turOfseti = tur // yoklama merdiveni bu turdan yeniden başlar
					break           // yalnız switch'ten çıkar; döngü sürer
				}
			}
			return teshis.Yeni(kod, yaziciAdi, "", nil)
		case teshis.KararBelirsiz:
			gunluk.Yaz("'%s': baskı sırası dolu, teslim doğrulanamadı — kağıt çıkmadıysa panelden yeniden gönderin", yaziciAdi)
			// Sıra şişmişse KENDİ ölü işlerimizi (10 dk'dan eski, hata bitli,
			// hiç sayfa basmamış) temizleyip yer açalım. Başkasının belgesine
			// dokunulmaz (bkz. onarim_windows.go).
			if silinen := kendiOluIslerimiziSil(yaziciAdi); silinen > 0 {
				gunluk.Yaz("'%s': %d ölü baskı işi temizlendi", yaziciAdi, silinen)
			}
			return nil
		}
		sonBitler, sonSayfa = bitler, sayfa
		// turOfseti: onarımdan sonra yoklama merdiveni baştan başlasın (iş
		// yeniden sıraya girip basılana kadar birkaç tur daha bakarız).
		adim := tur - turOfseti
		if adim >= len(teslimYoklamaAraliklari) || time.Now().After(bitis) {
			break
		}
		time.Sleep(teslimYoklamaAraliklari[adim])
	}

	// Süre doldu ve iş hâlâ sırada: SPOOLING'de takılıysa ve sayfa ilerlemiyorsa
	// teslim belirsizdir. 'basildi' bildiririz (çift fiş riski almayız) ama
	// günlüğe "cihaz onayı yok" satırı bırakırız.
	gunluk.Yaz("'%s': fiş spooler'a verildi ama cihaz onayı gelmedi (durum=0x%X, basılan sayfa=%d)",
		yaziciAdi, sonBitler, sonSayfa)
	if bitisHatasi != nil {
		return teshis.Yeni(teshis.BELIRSIZ_TESLIM, yaziciAdi, "", bitisHatasi)
	}
	return nil
}

// isiBul — kuyrukta bu iş kimliğini arar.
// listeDoldu: EnumJobs istenen üst sınır kadar iş döndürdü → liste kesilmiş
// olabilir, "listede yok = başarılı" kuralı UYGULANAMAZ.
func isiBul(h syscall.Handle, isKimligi uint32) (listedeVar bool, bitler, sayfa uint32, listeDoldu bool, err error) {
	const ustSinir = 255
	isler, donen, err := islerOku(h, ustSinir)
	if err != nil {
		return false, 0, 0, false, err
	}
	listeDoldu = donen >= ustSinir
	for _, is := range isler {
		if is.JobID == isKimligi {
			return true, is.StatusCode, is.PagesPrinted, listeDoldu, nil
		}
	}
	return false, 0, 0, listeDoldu, nil
}

// islerOku — EnumJobs level 1. JOB_INFO_1 godoes/printers'ta DIŞA AÇIK olduğu
// için ayna struct GEREKMEZ.
func islerOku(h syscall.Handle, ustSinir uint32) ([]printers.JOB_INFO_1, uint32, error) {
	var gerekli, donen uint32
	tampon := make([]byte, 1)
	err := printers.EnumJobs(h, 0, ustSinir, 1, &tampon[0], uint32(len(tampon)), &gerekli, &donen)
	if err != nil {
		if err != syscall.ERROR_INSUFFICIENT_BUFFER {
			return nil, 0, err
		}
		if gerekli == 0 {
			return nil, 0, nil
		}
		tampon = make([]byte, gerekli)
		if err = printers.EnumJobs(h, 0, ustSinir, 1, &tampon[0], uint32(len(tampon)), &gerekli, &donen); err != nil {
			return nil, 0, err
		}
	}
	if donen == 0 {
		return nil, 0, nil
	}
	return unsafe.Slice((*printers.JOB_INFO_1)(unsafe.Pointer(&tampon[0])), int(donen)), donen, nil
}

// isBelgeAdi — JOB_INFO_1'deki belge adını okur (onarım filtresi kullanır).
func isBelgeAdi(is printers.JOB_INFO_1) string {
	if is.Document == nil {
		return ""
	}
	return windows.UTF16PtrToString(is.Document)
}
