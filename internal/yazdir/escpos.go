package yazdir

// ESC/POS "gerçek zamanlı durum" sorgusu — DLE EOT n (0x10 0x04 n).
//
// NEDEN: ağ (ip:9100) yazıcısına bayt yazmak HER ZAMAN başarılı görünür —
// soket kabul eder, yazıcıda kağıt olmasa bile. Kağıt bitmişken fişleri
// göndermeye devam edersek yazıcı onları tamponlar ve kullanıcı rulo takınca
// 20 fiş birden dökülür (canlı vaka). DLE EOT, yazıcının o ANDAKİ hâlini
// SORAR ve tek bayt cevap alır; kağıt bittiyse fişi HİÇ göndermeyiz.
//
// Bu dosya SAF: soket/zaman aşımı işi çağırana aittir, burada yalnız bayt
// üretimi ve bit çözümü vardır — macOS'ta birim testiyle sınanır.

// DLE EOT sorgu numaraları.
const (
	DleEotYaziciDurumu    byte = 1 // n=1: genel yazıcı durumu (çevrimdışı mı)
	DleEotCevrimdisiSebep byte = 2 // n=2: çevrimdışı sebebi (kapak açık, kağıt sonu)
	DleEotKagitSensoru    byte = 4 // n=4: kağıt sensörü (kağıt bitti mi)
)

// DleEotSorgu — 0x10 0x04 n dizisini üretir.
func DleEotSorgu(n byte) []byte {
	return []byte{0x10, 0x04, n}
}

// DurumBiti — DLE EOT cevabının tek baytını çözer.
//
// Geçerli bir cevapta bit0=0, bit1=1, bit4=1, bit7=0'dır (Epson ESC/POS
// sözleşmesi). Bunlar tutmuyorsa gelen bayt cevabımız DEĞİLDİR (yazıcı
// yankı yapıyor, ya da hiç konuşmuyor) → gecerli=false ve HİÇBİR sonuç
// çıkarılmaz. "Cevap yok" TEK BAŞINA hata sayılmaz: klon yazıcıların bir
// kısmı DLE EOT'u hiç desteklemez ve onlarda baskı normal çalışır.
func DurumBiti(n byte, cevap byte) (kagitYok, kapakAcik, cevrimdisi bool, gecerli bool) {
	if cevap&0x93 != 0x12 {
		return false, false, false, false
	}
	switch n {
	case DleEotYaziciDurumu:
		// bit3: 1 = çevrimdışı
		return false, false, cevap&0x08 != 0, true
	case DleEotCevrimdisiSebep:
		// bit2: kapak açık, bit5: kağıt bitti (besleme durdu), bit6: hata
		kapakAcik = cevap&0x04 != 0
		kagitYok = cevap&0x20 != 0
		cevrimdisi = kapakAcik || kagitYok || cevap&0x40 != 0
		return kagitYok, kapakAcik, cevrimdisi, true
	case DleEotKagitSensoru:
		// bit2+bit3: kağıt azaldı (uyarı — baskı yapılır)
		// bit5+bit6: kağıt BİTTİ (baskı yapılmaz)
		kagitYok = cevap&0x60 == 0x60
		return kagitYok, false, kagitYok, true
	}
	return false, false, false, false
}
