package yazdir

import (
	"sync"

	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
	"github.com/kafe-panel/hizmetra-kopru/internal/onarim"
	"github.com/kafe-panel/hizmetra-kopru/internal/teshis"
)

// Onarım katmanının PLATFORMDAN BAĞIMSIZ kapısı.
//
// Kurallar (hepsi burada zorlanır, platform dosyaları yalnız İŞİ yapar):
//  1. Defter kurulu değilse HİÇBİR onarım yapılmaz (geri alınamaz değişiklik yasak).
//  2. Devre kesici (saatte 3) aşılmışsa bırakılır — kullanıcı bunu bilerek
//     yapıyor olabilir.
//  3. Aynı hedefte iki onarım aynı anda koşmaz (mutfak + kasa yazıcısı ayrı
//     kilitlerle sıralanır).
//  4. Her onarım günlüğe "ne yaptım" satırı bırakır.

// OnarimSonucu — bir onarım denemesinin çıktısı.
type OnarimSonucu struct {
	Yapildi   bool
	Aciklama  string // kullanıcıya gösterilecek tek cümle
	KayitID   int64  // defter kaydı (geri alma için)
	Onarilmaz bool   // bu platformda/bu kodda onarım yolu yok
}

var (
	defterKilit sync.Mutex
	defter      *onarim.Defter

	hedefKilitleri   = map[string]*sync.Mutex{}
	hedefKilitKilidi sync.Mutex
)

// DefterAyarla — main açılışta çağırır (ayar dizini). nil verilirse onarımlar
// kapanır.
func DefterAyarla(d *onarim.Defter) {
	defterKilit.Lock()
	defter = d
	defterKilit.Unlock()
}

// Defter — kurulu onarım defteri (durum penceresi "Son onarım" satırı için).
func Defter() *onarim.Defter {
	defterKilit.Lock()
	defer defterKilit.Unlock()
	return defter
}

func hedefKilidi(hedef string) *sync.Mutex {
	hedefKilitKilidi.Lock()
	defer hedefKilitKilidi.Unlock()
	m, varmi := hedefKilitleri[hedef]
	if !varmi {
		m = &sync.Mutex{}
		hedefKilitleri[hedef] = m
	}
	return m
}

// Onar — teşhis koduna karşılık gelen GERİ ALINABİLİR onarımı dener.
// Onarılamayan kodlarda hiçbir şey yapmaz (Onarilmaz=true).
func Onar(yaziciAd string, kod teshis.Kod) OnarimSonucu {
	bulgu := teshis.BulguAl(kod, yaziciAd, "")
	if !bulgu.Onarilabilir {
		return OnarimSonucu{Onarilmaz: true, Aciklama: "Bu sorun kendiliğinden düzeltilemez."}
	}
	d := Defter()
	if d == nil {
		return OnarimSonucu{Onarilmaz: true, Aciklama: "Onarım defteri açılamadığı için değişiklik yapılmadı."}
	}
	islem := onarimIslemAdi(kod)
	if !d.Izin(yaziciAd, islem) {
		gunluk.Yaz("onarım bırakıldı: '%s' üzerinde '%s' son bir saatte %d kez denendi — kullanıcı bunu bilerek yapıyor olabilir",
			yaziciAd, islem, onarim.SaattekiUstSinir)
		return OnarimSonucu{Aciklama: "Bu ayar kısa sürede birkaç kez düzeltildi; kendiliğinden bir daha değiştirilmeyecek."}
	}

	kilit := hedefKilidi(yaziciAd)
	kilit.Lock()
	defer kilit.Unlock()

	sonuc := onarPlatform(yaziciAd, kod, d)
	if sonuc.Yapildi {
		OnbellegiTemizle() // durum değişti; bir sonraki okuma gerçek tarama yapsın
		gunluk.Yaz("onarım yapıldı: '%s' → %s (geri alınabilir, kayıt #%d)", yaziciAd, islem, sonuc.KayitID)
	}
	return sonuc
}

// onarimIslemAdi — defterdeki işlem etiketi (devre kesici bunu sayar).
func onarimIslemAdi(kod teshis.Kod) string {
	switch kod {
	case teshis.KUYRUK_DURAKLATILDI:
		return "duraklatmayi-kaldir"
	case teshis.CEVRIMDISI_ISARETLI:
		return "cevrimdisi-bayragini-dus"
	}
	return string(kod)
}

// OnarimiGeriAl — defter kaydını pasife çeker ve eski değeri geri yazar.
func OnarimiGeriAl(kayitID int64) bool {
	d := Defter()
	if d == nil {
		return false
	}
	kayit, varmi := d.Sonuncu()
	if !varmi || kayit.ID != kayitID {
		return false
	}
	eski, tamam := d.GeriAl(kayitID)
	if !tamam {
		return false
	}
	kilit := hedefKilidi(kayit.Hedef)
	kilit.Lock()
	defer kilit.Unlock()
	basarili := geriAlPlatform(kayit.Hedef, kayit.Islem, eski)
	gunluk.Yaz("onarım geri alındı: '%s' → %s (eski değer: %s, başarılı=%v)", kayit.Hedef, kayit.Islem, eski, basarili)
	OnbellegiTemizle()
	return basarili
}
