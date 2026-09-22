package kopru

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

// ÇİFT BASKI KALKANININ DİSKE KALICI HÂLİ.
//
// NEDEN: kalkan bugün yalnız BELLEKTE. Ajan güncelleme/çökme/yeniden başlatma
// sonrası açıldığında hafızası boş oluyor; sunucu o sırada 'gonderiliyor'da
// kalmış işi yeniden verdiğinde AYNI FİŞ İKİNCİ KEZ basılıyor. Kafede bu,
// mutfağa iki kez düşen sipariş demek.
//
// GİZLİLİK: dosyada YALNIZ {is_id, zaman} durur. Fiş içeriği, hedef adı, tutar
// — hiçbiri yazılmaz.
//
// Dosya ayrıca BİLDİRİLEMEYEN sonuçları da taşır: sonuç POST'u başarısız
// olursa iş sunucuda 'gonderiliyor' kalır; sıradaki turda yeniden bildiririz.

const kalkanDosyaAdi = "basildi.json"

// kalkanSaklamaSuresi — bundan eski basıldı kayıtları atılır. Sunucunun
// yeniden-sahiplenme penceresinden (300 sn) kat kat uzun; 1 saat fazlasıyla yeter.
const kalkanSaklamaSuresi = time.Hour

type kalkanKayit struct {
	IsID  int64     `json:"is_id"`
	Zaman time.Time `json:"zaman"`
}

type kalkanDosya struct {
	Basildi      []kalkanKayit `json:"basildi"`
	Bildirilmedi []api.Sonuc   `json:"bildirilmedi"`
	// Supheli — baskısı YARIDA KALMIŞ işler (30 sn'de bitmeyen "asıldı" işi,
	// yarım yazılan fiş). Bu işlerin kağıda dökülüp dökülmediğini BİLMİYORUZ:
	// spooler tıkanmayı açtığı an eski iş kendiliğinden basabilir. Bu yüzden
	// bir daha ASLA basılmazlar (bkz. dongu.go supheliIsaretle).
	Supheli []kalkanKayit `json:"supheli"`
}

// KalkanYolu — verilen ayar dizinindeki kalkan dosyasının tam yolu.
func KalkanYolu(dizin string) string { return filepath.Join(dizin, kalkanDosyaAdi) }

// KalkanDosyasiAyarla — kalkanı diske bağlar ve varsa eski kayıtları yükler.
// main açılışta çağırır; testler kendi geçici dizinini verir.
//
// Yeni() İÇİNDE ÇAĞRILMAZ: ajan kurmak diske dokunmak demek olmamalı (testler
// ve e2e kurulumları gerçek kullanıcı dizinine yazmasın).
func (a *Ajan) KalkanDosyasiAyarla(yol string) {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	a.kalkanYolu = yol
	a.kalkanYukle()
}

// kalkanYukle — kayitKilit TUTULURKEN çağrılır.
func (a *Ajan) kalkanYukle() {
	ham, err := os.ReadFile(a.kalkanYolu)
	if err != nil {
		return // dosya yok (ilk çalıştırma) → boş kalkan
	}
	var d kalkanDosya
	if err := json.Unmarshal(ham, &d); err != nil {
		gunluk.Yaz("çift baskı kalkanı dosyası okunamadı, sıfırdan başlanıyor: %v", err)
		return
	}
	sinir := time.Now().Add(-kalkanSaklamaSuresi)
	sayi := 0
	for _, k := range d.Basildi {
		if k.Zaman.Before(sinir) {
			continue
		}
		a.basildiKayit[k.IsID] = k.Zaman
		sayi++
	}
	for _, k := range d.Supheli {
		if k.Zaman.Before(sinir) {
			continue
		}
		a.supheliKayit[k.IsID] = k.Zaman
		sayi++
	}
	a.bildirilmedi = append(a.bildirilmedi, d.Bildirilmedi...)
	if sayi > 0 || len(d.Bildirilmedi) > 0 {
		gunluk.Yaz("çift baskı kalkanı yüklendi: %d basılmış iş, %d bildirilmemiş sonuç", sayi, len(d.Bildirilmedi))
	}
}

// kalkanKaydet — kayitKilit TUTULURKEN çağrılır. Atomik yazım (tmp + rename):
// yazım sırasında çökme yarım dosya bırakmaz.
func (a *Ajan) kalkanKaydet() {
	if a.kalkanYolu == "" {
		return // diske bağlanmamış (test/eski davranış) — yalnız bellekte
	}
	d := kalkanDosya{
		Basildi:      make([]kalkanKayit, 0, len(a.basildiKayit)),
		Bildirilmedi: a.bildirilmedi,
		Supheli:      make([]kalkanKayit, 0, len(a.supheliKayit)),
	}
	for id, t := range a.basildiKayit {
		d.Basildi = append(d.Basildi, kalkanKayit{IsID: id, Zaman: t})
	}
	for id, t := range a.supheliKayit {
		d.Supheli = append(d.Supheli, kalkanKayit{IsID: id, Zaman: t})
	}
	ham, err := json.Marshal(d)
	if err != nil {
		return
	}
	gecici := a.kalkanYolu + ".tmp"
	if err := os.WriteFile(gecici, ham, 0o600); err != nil {
		return
	}
	if err := os.Rename(gecici, a.kalkanYolu); err != nil {
		_ = os.Remove(gecici)
	}
}

// bildirilmedigineEkle — sunucuya ULAŞMAYAN sonuçları kuyruğa alır; sıradaki
// turda yeniden POST edilir. Kuyruk diske de yazılır ki yeniden başlatma
// sonrası da bildirilsin.
func (a *Ajan) bildirilmedigineEkle(sonuclar []api.Sonuc) {
	if len(sonuclar) == 0 {
		return
	}
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	for _, s := range sonuclar {
		varmi := false
		for _, mevcut := range a.bildirilmedi {
			if mevcut.IsID == s.IsID {
				varmi = true
				break
			}
		}
		if !varmi {
			a.bildirilmedi = append(a.bildirilmedi, s)
		}
	}
	// Sonsuz büyümesin: en yeni 200 kayıt yeter.
	if len(a.bildirilmedi) > 200 {
		a.bildirilmedi = a.bildirilmedi[len(a.bildirilmedi)-200:]
	}
	a.kalkanKaydet()
}

// bekleyenSonuclar — bildirilememiş sonuçları alır ve kuyruğu boşaltır.
func (a *Ajan) bekleyenSonuclar() []api.Sonuc {
	a.kayitKilit.Lock()
	defer a.kayitKilit.Unlock()
	if len(a.bildirilmedi) == 0 {
		return nil
	}
	out := a.bildirilmedi
	a.bildirilmedi = nil
	a.kalkanKaydet()
	return out
}
