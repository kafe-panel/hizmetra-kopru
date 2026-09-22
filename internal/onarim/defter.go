// Package onarim — ajanın kullanıcının yazıcı ayarlarında yaptığı HER
// değişikliğin geri alınabilir kaydı.
//
// İLKE: kullanıcının kurulumunu izinsiz ve sessizce değiştirmek, hiç
// değiştirmemekten daha kötüdür. Bu yüzden her onarım ÖNCE buraya eski değeriyle
// yazılır, sonra uygulanır. Kayıt olmadan onarım YOK.
//
// DEVRE KESİCİ: aynı hedefte aynı işlem saatte 3 kezden fazla gerekiyorsa
// kullanıcı büyük ihtimalle bunu BİLEREK yapıyor (ör. kuyruğu kendi
// duraklatıyor). Dördüncüden itibaren onarım bırakılır, yalnız bildirilir.
//
// GİZLİLİK: dosyada PII YOKTUR — yalnız hedef adı, işlem adı ve bayrak
// değerleri. Fiş içeriği asla.
package onarim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SaattekiUstSinir — aynı hedef+işlem için saatlik onarım üst sınırı.
const SaattekiUstSinir = 3

// budamaSuresi — bundan eski kayıtlar atılır.
const budamaSuresi = 30 * 24 * time.Hour

// Kayit — tek bir onarım satırı (append-only).
type Kayit struct {
	ID         int64     `json:"id"`
	Zaman      time.Time `json:"zaman"`
	Hedef      string    `json:"hedef"`
	Islem      string    `json:"islem"`
	EskiDeger  string    `json:"eski_deger"`
	YeniDeger  string    `json:"yeni_deger"`
	Aktif      bool      `json:"aktif"`
	GeriAlindi time.Time `json:"geri_alindi,omitempty"`
}

// Defter — diske yazılan onarım geçmişi. Eşzamanlı kullanılabilir.
type Defter struct {
	kilit    sync.Mutex
	yol      string
	kayitlar []Kayit
	sonID    int64
}

// Ac — defteri açar. Dosya yoksa veya BOZUKSA boş defterle devam edilir:
// yarım bir JSON yüzünden ajan asla durmamalı.
func Ac(dizin string) *Defter {
	d := &Defter{yol: filepath.Join(dizin, "onarim.json")}
	d.yukle()
	return d
}

func (d *Defter) yukle() {
	ham, err := os.ReadFile(d.yol)
	if err != nil {
		return
	}
	var kayitlar []Kayit
	if err := json.Unmarshal(ham, &kayitlar); err != nil {
		// Bozuk dosya: sıfırdan başla (üzerine ilk yazımda temiz JSON gelir).
		return
	}
	sinir := time.Now().Add(-budamaSuresi)
	for _, k := range kayitlar {
		if k.Zaman.Before(sinir) {
			continue
		}
		d.kayitlar = append(d.kayitlar, k)
		if k.ID > d.sonID {
			d.sonID = k.ID
		}
	}
}

// kaydet — ATOMİK yazım (ayar.Kaydet deseni): temp'e yaz + rename. Yarım dosya
// bırakmaz; çökme anında bile defter okunabilir kalır.
func (d *Defter) kaydet() error {
	ham, err := json.MarshalIndent(d.kayitlar, "", "  ")
	if err != nil {
		return err
	}
	gecici := d.yol + ".tmp"
	if err := os.WriteFile(gecici, ham, 0o600); err != nil {
		return err
	}
	if err := os.Rename(gecici, d.yol); err != nil {
		_ = os.Remove(gecici)
		return err
	}
	return nil
}

// Izin — bu hedefte bu işlem şu an denenebilir mi? (devre kesici)
func (d *Defter) Izin(hedef, islem string) bool {
	if d == nil {
		return false
	}
	d.kilit.Lock()
	defer d.kilit.Unlock()
	sinir := time.Now().Add(-time.Hour)
	sayi := 0
	for _, k := range d.kayitlar {
		if k.Hedef == hedef && k.Islem == islem && k.Zaman.After(sinir) {
			sayi++
		}
	}
	return sayi < SaattekiUstSinir
}

// Yaz — onarımı UYGULAMADAN ÖNCE deftere işler ve kayıt kimliğini döndürür.
// Hata dönerse onarım YAPILMAMALIDIR (geri alınamaz değişiklik yasak).
func (d *Defter) Yaz(hedef, islem, eskiDeger, yeniDeger string) (int64, error) {
	if d == nil {
		return 0, os.ErrInvalid
	}
	d.kilit.Lock()
	defer d.kilit.Unlock()
	d.sonID++
	k := Kayit{
		ID: d.sonID, Zaman: time.Now(), Hedef: hedef, Islem: islem,
		EskiDeger: eskiDeger, YeniDeger: yeniDeger, Aktif: true,
	}
	d.kayitlar = append(d.kayitlar, k)
	if err := d.kaydet(); err != nil {
		return k.ID, err
	}
	return k.ID, nil
}

// GeriAl — kaydı pasife çeker ve ESKİ değeri döndürür (çağıran onu uygular).
// Kayıt yoksa veya zaten geri alınmışsa ikinci dönüş false olur.
func (d *Defter) GeriAl(id int64) (string, bool) {
	if d == nil {
		return "", false
	}
	d.kilit.Lock()
	defer d.kilit.Unlock()
	for i := range d.kayitlar {
		if d.kayitlar[i].ID != id || !d.kayitlar[i].Aktif {
			continue
		}
		d.kayitlar[i].Aktif = false
		d.kayitlar[i].GeriAlindi = time.Now()
		eski := d.kayitlar[i].EskiDeger
		_ = d.kaydet()
		return eski, true
	}
	return "", false
}

// Sonuncu — en son yapılan (hâlâ aktif) onarım; yoksa ikinci dönüş false.
// Durum penceresi "Son onarım: … + Geri Al" satırını bundan üretir.
func (d *Defter) Sonuncu() (Kayit, bool) {
	if d == nil {
		return Kayit{}, false
	}
	d.kilit.Lock()
	defer d.kilit.Unlock()
	for i := len(d.kayitlar) - 1; i >= 0; i-- {
		if d.kayitlar[i].Aktif {
			return d.kayitlar[i], true
		}
	}
	return Kayit{}, false
}

// Sayi — defterdeki kayıt sayısı (test/teşhis).
func (d *Defter) Sayi() int {
	if d == nil {
		return 0
	}
	d.kilit.Lock()
	defer d.kilit.Unlock()
	return len(d.kayitlar)
}
