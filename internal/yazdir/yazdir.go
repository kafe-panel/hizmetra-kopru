// Package yazdir — ESC/POS baytlarını fiziksel yazıcıya iletir.
//
// İki yol var, hedefin ŞEKLİNDEN seçilir:
//   - "192.168.1.50:9100" → ham TCP soketi (JetDirect). Sürücü GEREKMEZ.
//   - "POS-80"            → Windows spooler'a RAW iş. Sürücü kurulu olmalı ama
//     RAW datatype sürücünün render'ını BYPASS eder → ESC/POS bozulmadan gider.
package yazdir

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// spoolerYaz — Windows spooler'a RAW yazan fonksiyon. Testte değiştirilebilsin
// diye değişken (windows dışı derlemede stub'a bağlanır).
var spoolerYaz = spoolerYazPlatform

// ErrBosHedef — yazıcıya hedef atanmamış (panelde yapılandırma eksik).
var ErrBosHedef = errors.New("yazdir: hedef boş — panelden yazıcıya hedef atayın")

const (
	agBaglantiTimeout = 5 * time.Second
	agYazmaTimeout    = 10 * time.Second
	// Klon yazıcılar (Zjiang vb.) soketi kapatınca son baytları düşürebiliyor;
	// kapatmadan önce kısa bekleme veriyoruz.
	agKapatmaBeklemesi = 300 * time.Millisecond
)

// AgHedefiMi — "host:port" biçimi mi? (port sayısal olmalı; "POS-80" gibi
// tire içeren yazıcı adları YANLIŞLIKLA ağ sanılmasın diye port kontrolü şart).
func AgHedefiMi(hedef string) bool {
	host, port, err := net.SplitHostPort(strings.TrimSpace(hedef))
	if err != nil || host == "" {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535
}

// Bas — baytları hedefe iletir. Hata dönerse iş 'hata' olarak bildirilir.
func Bas(hedef string, veri []byte) error {
	hedef = strings.TrimSpace(hedef)
	if hedef == "" {
		return ErrBosHedef
	}
	if AgHedefiMi(hedef) {
		return agaBas(hedef, veri)
	}
	return spoolerYaz(hedef, veri)
}

func agaBas(hedef string, veri []byte) error {
	baglanti, err := net.DialTimeout("tcp", hedef, agBaglantiTimeout)
	if err != nil {
		return fmt.Errorf("yazdir: %s bağlanılamadı (yazıcı kapalı veya IP yanlış): %w", hedef, err)
	}
	defer baglanti.Close()

	if err := baglanti.SetWriteDeadline(time.Now().Add(agYazmaTimeout)); err != nil {
		return fmt.Errorf("yazdir: yazma süresi ayarlanamadı: %w", err)
	}
	yazilan, err := baglanti.Write(veri)
	if err != nil {
		return fmt.Errorf("yazdir: %s yazılamadı: %w", hedef, err)
	}
	if yazilan != len(veri) {
		return fmt.Errorf("yazdir: eksik yazıldı (%d/%d bayt)", yazilan, len(veri))
	}
	// Yazıcı ACK vermez; baytların çıkması için kısa bekleme.
	time.Sleep(agKapatmaBeklemesi)
	return nil
}
