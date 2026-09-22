//go:build windows

package yazdir

import (
	"errors"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/godoes/printers"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/onarim"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// GERİ ALINABİLİR Windows onarımları.
//
// ALTIN KURAL: hiçbir onarım, ÖNCE hiçbir şeyi değiştirmeden yetki ölçmeden
// başlamaz. Standart (yönetici olmayan) bir kullanıcıda PRINTER_ACCESS_ADMINISTER
// ile açmak errno 5 (ERROR_ACCESS_DENIED) verir; o durumda HİÇBİR ŞEY denenmez,
// kullanıcıya net Türkçe talimat verilir. Yarım kalmış bir SetPrinter, kafenin
// çalışan kurulumunu bozabilir.
//
// Toptan kuyruk temizleme komutu HİÇ KULLANILMAZ: kullanıcının kendi
// belgelerini silerdik. Yalnız KENDİ ölü işlerimizi, önek eşleşmesiyle sileriz.

// yetkiOlc — ayar değiştirme yetkimiz var mı? Değiştirmeden ÖNCE ölçülür.
func yetkiOlc(yaziciAdi string) (syscall.Handle, error) {
	h, err := yaziciAc(yaziciAdi, PRINTER_ACCESS_ADMINISTER|PRINTER_ACCESS_USE)
	if err != nil {
		var errno syscall.Errno
		if errors.As(err, &errno) && errno == ERROR_ACCESS_DENIED {
			return 0, teshis.Yeni(teshis.YETKI_YOK, yaziciAdi, "", err)
		}
		return 0, err
	}
	return h, nil
}

func onarPlatform(yaziciAd string, kod teshis.Kod, defter *onarim.Defter) OnarimSonucu {
	h, err := yetkiOlc(yaziciAd)
	if err != nil {
		return OnarimSonucu{Onarilmaz: true, Aciklama: teshis.Cumle(teshis.YETKI_YOK, yaziciAd, "")}
	}
	defer printers.ClosePrinter(h) //nolint:errcheck

	switch kod {
	case teshis.KUYRUK_DURAKLATILDI:
		return duraklatmayiKaldir(h, yaziciAd, defter)
	case teshis.CEVRIMDISI_ISARETLI:
		return cevrimdisiBayraginiDus(h, yaziciAd, defter)
	}
	return OnarimSonucu{Onarilmaz: true, Aciklama: "Bu sorun kendiliğinden düzeltilemez."}
}

// duraklatmayiKaldir — PRINTER_CONTROL_RESUME. Geri alma: kullanıcı kuyruğu
// yeniden duraklatabilir (biz duraklatmayız — kimseyi baskısız bırakmayız).
func duraklatmayiKaldir(h syscall.Handle, yaziciAd string, defter *onarim.Defter) OnarimSonucu {
	kayitID, err := defter.Yaz(yaziciAd, "duraklatmayi-kaldir", "duraklatilmis", "calisiyor")
	if err != nil {
		return OnarimSonucu{Aciklama: "Onarım kaydı yazılamadığı için değişiklik yapılmadı."}
	}
	if err := printers.SetPrinter(h, 0, nil, PRINTER_CONTROL_RESUME); err != nil {
		defter.GeriAl(kayitID)
		return OnarimSonucu{Aciklama: "Baskı sırası yeniden başlatılamadı."}
	}
	return OnarimSonucu{Yapildi: true, KayitID: kayitID, Aciklama: "Baskı sırası yeniden çalıştırıldı."}
}

// cevrimdisiBayraginiDus — PRINTER_INFO_5'teki 0x400 (WORK_OFFLINE) bitini
// temizler.
//
// AYNA STRUCT YOK: PRINTER_INFO_5 godoes/printers'ta DIŞA AÇIK. GetPrinter ile
// gelen TAMPONUN İÇİNDE yalnız Attributes alanı değiştirilip aynı tampon geri
// yazılır — diğer alanlara (isim/port işaretçileri dahil) DOKUNULMAZ.
func cevrimdisiBayraginiDus(h syscall.Handle, yaziciAd string, defter *onarim.Defter) OnarimSonucu {
	tampon, err := yaziciBilgisiOku(h, 5)
	if err != nil {
		return OnarimSonucu{Aciklama: "Yazıcı ayarları okunamadı; değişiklik yapılmadı."}
	}
	bilgi := (*printers.PRINTER_INFO_5)(unsafe.Pointer(&tampon[0]))
	if bilgi.Attributes&PRINTER_ATTRIBUTE_WORK_OFFLINE == 0 {
		return OnarimSonucu{Aciklama: "Çevrimdışı işareti zaten kapalıydı."}
	}
	kayitID, err := defter.Yaz(yaziciAd, "cevrimdisi-bayragini-dus", "cevrimdisi-acik", "cevrimdisi-kapali")
	if err != nil {
		return OnarimSonucu{Aciklama: "Onarım kaydı yazılamadığı için değişiklik yapılmadı."}
	}
	bilgi.Attributes &^= PRINTER_ATTRIBUTE_WORK_OFFLINE
	if err := printers.SetPrinter(h, 5, &tampon[0], 0); err != nil {
		defter.GeriAl(kayitID)
		return OnarimSonucu{Aciklama: "Çevrimdışı işareti kaldırılamadı."}
	}
	return OnarimSonucu{Yapildi: true, KayitID: kayitID, Aciklama: "Yazıcı yeniden çevrimiçi kullanılacak şekilde işaretlendi."}
}

// geriAlPlatform — deftere yazılmış eski değeri geri yükler.
func geriAlPlatform(yaziciAd, islem, eskiDeger string) bool {
	if islem != "cevrimdisi-bayragini-dus" || eskiDeger != "cevrimdisi-acik" {
		// "duraklatmayi-kaldir" GERİ ALINMAZ: kuyruğu yeniden duraklatmak
		// kafeyi fişsiz bırakırdı. Kayıt pasife çekilir, eylem uygulanmaz.
		return false
	}
	h, err := yetkiOlc(yaziciAd)
	if err != nil {
		return false
	}
	defer printers.ClosePrinter(h) //nolint:errcheck
	tampon, err := yaziciBilgisiOku(h, 5)
	if err != nil {
		return false
	}
	bilgi := (*printers.PRINTER_INFO_5)(unsafe.Pointer(&tampon[0]))
	bilgi.Attributes |= PRINTER_ATTRIBUTE_WORK_OFFLINE
	return printers.SetPrinter(h, 5, &tampon[0], 0) == nil
}

// yaziciBilgisiOku — GetPrinter için iki aşamalı tampon ayırma.
func yaziciBilgisiOku(h syscall.Handle, seviye uint32) ([]byte, error) {
	var gerekli uint32
	tampon := make([]byte, 1)
	err := printers.GetPrinter(h, seviye, &tampon[0], uint32(len(tampon)), &gerekli)
	if err == nil {
		return tampon, nil
	}
	if err != syscall.ERROR_INSUFFICIENT_BUFFER || gerekli == 0 {
		return nil, err
	}
	tampon = make([]byte, gerekli)
	if err := printers.GetPrinter(h, seviye, &tampon[0], gerekli, &gerekli); err != nil {
		return nil, err
	}
	return tampon, nil
}

// kendiOluIslerimiziSil — YALNIZ bizim gönderdiğimiz, 10 dakikadan eski, hata
// biti taşıyan ve HİÇ sayfa basmamış işleri siler.
//
// Dört koşulun hepsi birden aranır; biri bile tutmazsa iş DURUR. Başkasının
// belgesine asla dokunulmaz, toptan temizlik (PURGE) hiç kullanılmaz.
func kendiOluIslerimiziSil(yaziciAd string) int {
	h, err := yetkiOlc(yaziciAd)
	if err != nil {
		return 0
	}
	defer printers.ClosePrinter(h) //nolint:errcheck

	isler, _, err := islerOku(h, 255)
	if err != nil {
		return 0
	}
	silinen := 0
	simdi := time.Now()
	for _, is := range isler {
		if !strings.HasPrefix(isBelgeAdi(is), spoolerBelgeAdiOnek) {
			continue // BAŞKASININ belgesi
		}
		if is.PagesPrinted != 0 {
			continue // kısmen basmış — dokunma
		}
		if _, hataVar := teshis.IsHataKodu(is.StatusCode); !hataVar {
			continue // hata biti yok — bekliyor olabilir
		}
		if simdi.Sub(isGonderimZamani(is)) < 10*time.Minute {
			continue // taze iş — spooler hâlâ uğraşıyor olabilir
		}
		if err := isiSil(h, is.JobID); err != nil {
			continue
		}
		silinen++
		gunluk.Yaz("ölü baskı işi silindi: '%s' kuyruğunda iş #%d (%s)", yaziciAd, is.JobID, isBelgeAdi(is))
	}
	return silinen
}

// isGonderimZamani — JOB_INFO_1.Submitted (UTC SYSTEMTIME) → yerel zaman.
func isGonderimZamani(is printers.JOB_INFO_1) time.Time {
	s := is.Submitted
	if s.Year == 0 {
		return time.Now() // okunamadı → "taze" say, silme
	}
	return time.Date(int(s.Year), time.Month(s.Month), int(s.Day),
		int(s.Hour), int(s.Minute), int(s.Second), 0, time.UTC).Local()
}
