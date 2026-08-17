// Package surum — sürüm dizgilerinin SEMANTİK karşılaştırması.
//
// Neden ayrı paket: "yeni sürüm var mı" kararı eskiden main.go'da düz DİZGİ
// eşitsizliğiyle veriliyordu (bilgi.Surum != Surum). Bu iki yönden hatalıydı:
//   - 0.10.0 ile 0.9.0'ı dizgi olarak karşılaştırınca "0.10.0" < "0.9.0"
//     çıkar ("1" < "9") → gerçek bir yükseltme "güncelle" olarak GÖSTERİLMEZ.
//   - Sunucu sürümü çalışandan ESKİ (downgrade) veya yalnızca biçimce farklı
//     olduğunda da "güncelle" şeridi belirir → kullanıcı yanıltılır.
//
// Karsilastir yalnız MAJOR.MINOR.PATCH sayısal çekirdeği kıyaslar; ön-sürüm
// (-rc1) ve derleme (+meta) ekleri KESİLİR (ajan sürümleri saf x.y.z; ekler
// bize kıyas anlamı katmaz). Çekirdek sayısal ayrıştırılamazsa güvenli düşüş:
// düz dizgi karşılaştırması (tanımsız durumda bile deterministik davranış).
package surum

import (
	"strconv"
	"strings"
)

// Karsilastir — a ve b sürümlerini semantik kıyaslar:
//
//	a < b  → -1
//	a == b →  0
//	a > b  → +1
//
// Baştaki "v" atılır ("v0.7.0" == "0.7.0"). Eksik bileşen 0 sayılır
// ("0.7" == "0.7.0"). Çekirdekte sayı-dışı bir bileşen varsa (ör. "abc")
// düz strings.Compare'e düşülür.
func Karsilastir(a, b string) int {
	pa, oka := ayristir(a)
	pb, okb := ayristir(b)
	if !oka || !okb {
		return strings.Compare(temizle(a), temizle(b))
	}
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

// YeniMi — hedef sürüm mevcut'tan KESİNLİKLE ileri mi (güncelleme sunulmalı mı).
// Eşit veya geri sürümde false döner — böylece "güncelle" şeridi yalnız gerçek
// bir ilerlemede belirir.
func YeniMi(hedef, mevcut string) bool {
	return Karsilastir(hedef, mevcut) > 0
}

// temizle — baştaki "v"/"V" ve etrafındaki boşlukları atar, ön-sürüm/derleme
// eklerini ("-rc1", "+meta") keser; sayısal çekirdeği bırakır.
func temizle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimLeft(s, "vV")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	return s
}

// ayristir — "x.y.z" çekirdeğini [3]int'e çevirir. En çok 3 bileşen okunur;
// eksikler 0'dır. Herhangi bir bileşen sayı değilse (false) çağıran düz dizgi
// kıyasına düşer.
func ayristir(s string) ([3]int, bool) {
	var p [3]int
	parcalar := strings.Split(temizle(s), ".")
	if len(parcalar) == 0 || parcalar[0] == "" {
		return p, false
	}
	for i := 0; i < len(parcalar) && i < 3; i++ {
		n, err := strconv.Atoi(parcalar[i])
		if err != nil || n < 0 {
			return p, false
		}
		p[i] = n
	}
	return p, true
}
