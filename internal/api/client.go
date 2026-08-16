// Package api — Hizmetra Panel köprü protokolü istemcisi.
//
// Protokol sözleşmesi sunucuda DONMUŞTUR (backend/app/api/kopru.py):
//
//	POST /api/kopru/eslestir      {kod,ad,makine,surum,os} -> {token,cihaz_id,isletme_ad}
//	POST /api/kopru/nabiz         {yazicilar,surum}        -> {ok,poll_sn}
//	GET  /api/kopru/isler?bekle=N                          -> [{is_id,hedef,tip,icerik_b64}]
//	POST /api/kopru/isler/sonuc   {sonuclar:[...]}         -> {islenen}
//
// Kimlik: X-Kopru-Token başlığı. Sunucu cevabı daima {success,data,message}.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrYetkisiz — token geçersiz/iptal edilmiş (401). Yeniden eşleşme gerekir;
// retry ETMEK ANLAMSIZ (ajan tray'de "Eşleştirme geçersiz" gösterir).
var ErrYetkisiz = errors.New("kopru: yetkisiz (token geçersiz veya cihaz iptal edildi)")

// ErrKodGecersiz — eşleştirme kodu yanlış/süresi dolmuş/kullanılmış (404).
var ErrKodGecersiz = errors.New("kopru: kurulum kodu geçersiz")

// Yazici — ajanın bulduğu bir yazıcı (nabızla sunucuya bildirilir).
type Yazici struct {
	Ad    string `json:"ad"`
	Hedef string `json:"hedef"` // Windows yazıcı adı VEYA "ip:port"
	Tip   string `json:"tip"`   // "windows" | "ag"
	Durum string `json:"durum"` // "online" | "offline"
}

// Is — sunucudan çekilen bir baskı işi.
type Is struct {
	IsID      int64  `json:"is_id"`
	Hedef     string `json:"hedef"`
	Tip       string `json:"tip"`
	IcerikB64 string `json:"icerik_b64"` // ESC/POS baytları — ASLA loglanmaz (PII)
}

// Sonuc — bir işin baskı sonucu.
type Sonuc struct {
	IsID  int64  `json:"is_id"`
	Durum string `json:"durum"` // "basildi" | "hata"
	Hata  string `json:"hata,omitempty"`
}

// Client — protokol istemcisi. Token boşsa yalnız Eslestir çağrılabilir.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New — varsayılan istemci. Timeout long-poll'de çağrı başına ayarlanır.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// zarf — sunucunun api_response(...) çıktısı.
type zarf struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

func (c *Client) istek(metot, yol string, govde any, cikti any, timeout time.Duration) error {
	var okuyucu io.Reader
	if govde != nil {
		b, err := json.Marshal(govde)
		if err != nil {
			return fmt.Errorf("kopru: gövde kodlanamadı: %w", err)
		}
		okuyucu = bytes.NewReader(b)
	}
	istek, err := http.NewRequest(metot, c.BaseURL+yol, okuyucu)
	if err != nil {
		return fmt.Errorf("kopru: istek kurulamadı: %w", err)
	}
	if govde != nil {
		istek.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		istek.Header.Set("X-Kopru-Token", c.Token)
	}
	istemci := c.HTTP
	if timeout > 0 {
		kopya := *c.HTTP
		kopya.Timeout = timeout
		istemci = &kopya
	}
	cevap, err := istemci.Do(istek)
	if err != nil {
		return fmt.Errorf("kopru: bağlantı: %w", err)
	}
	defer cevap.Body.Close()

	if cevap.StatusCode == http.StatusUnauthorized {
		return ErrYetkisiz
	}
	ham, err := io.ReadAll(io.LimitReader(cevap.Body, 8<<20)) // 8MB tavan
	if err != nil {
		return fmt.Errorf("kopru: cevap okunamadı: %w", err)
	}
	var z zarf
	if err := json.Unmarshal(ham, &z); err != nil {
		return fmt.Errorf("kopru: cevap çözümlenemedi (HTTP %d)", cevap.StatusCode)
	}
	if !z.Success {
		if cevap.StatusCode == http.StatusNotFound {
			return ErrKodGecersiz
		}
		return fmt.Errorf("kopru: sunucu hatası (HTTP %d): %s", cevap.StatusCode, z.Message)
	}
	if cikti != nil && len(z.Data) > 0 {
		if err := json.Unmarshal(z.Data, cikti); err != nil {
			return fmt.Errorf("kopru: veri çözümlenemedi: %w", err)
		}
	}
	return nil
}

// EslestirCevap — POST /eslestir dönüşü.
type EslestirCevap struct {
	Token      string `json:"token"`
	CihazID    int64  `json:"cihaz_id"`
	IsletmeAd  string `json:"isletme_ad"`
}

// Eslestir — 6 haneli kurulum kodunu kalıcı cihaz token'ına çevirir.
// Başarıda Client.Token da doldurulur.
func (c *Client) Eslestir(kod, ad, makine, surum, isletimSistemi string) (*EslestirCevap, error) {
	var cevap EslestirCevap
	err := c.istek(http.MethodPost, "/api/kopru/eslestir", map[string]string{
		"kod": kod, "ad": ad, "makine": makine, "surum": surum, "os": isletimSistemi,
	}, &cevap, 20*time.Second)
	if err != nil {
		return nil, err
	}
	c.Token = cevap.Token
	return &cevap, nil
}

// NabizCevap — sunucunun ölçek düğmesi. Ajan bu değerlere UYMAK ZORUNDA:
// sunucu cihaz sayısı arttığında PollSn'i büyütüp long-poll'u kısaltarak
// ajanı güncellemeden kısa-poll'a geçirebilir (master plan §0, V1→V2→V3).
type NabizCevap struct {
	OK     bool `json:"ok"`
	PollSn int  `json:"poll_sn"`
	BekleSn *int `json:"bekle_sn,omitempty"` // opsiyonel; yoksa PollSn kullanılır
}

// Nabiz — "hayattayım" + gördüğüm yazıcılar. son_gorulme_at'i tazeler.
func (c *Client) Nabiz(yazicilar []Yazici, surum string) (*NabizCevap, error) {
	if yazicilar == nil {
		yazicilar = []Yazici{}
	}
	var cevap NabizCevap
	err := c.istek(http.MethodPost, "/api/kopru/nabiz", map[string]any{
		"yazicilar": yazicilar, "surum": surum,
	}, &cevap, 20*time.Second)
	if err != nil {
		return nil, err
	}
	return &cevap, nil
}

// Isler — long-poll ile bekleyen işleri ATOMİK sahiplenerek çeker.
// bekleSn=0 → beklemeden döner. HTTP timeout'u bekleme + 10sn pay.
func (c *Client) Isler(bekleSn int) ([]Is, error) {
	if bekleSn < 0 {
		bekleSn = 0
	}
	yol := "/api/kopru/isler?bekle=" + url.QueryEscape(fmt.Sprint(bekleSn))
	var isler []Is
	err := c.istek(http.MethodGet, yol, nil, &isler, time.Duration(bekleSn+10)*time.Second)
	if err != nil {
		return nil, err
	}
	return isler, nil
}

// SonucBildir — baskı sonuçlarını yazar. 'basildi' DÜRÜSTTÜR: ajan baytları
// yazıcıya fiilen yazdı (PrintNode'daki "iletildi" belirsizliği burada YOK).
func (c *Client) SonucBildir(sonuclar []Sonuc) (int, error) {
	if len(sonuclar) == 0 {
		return 0, nil
	}
	var cevap struct {
		Islenen int `json:"islenen"`
	}
	err := c.istek(http.MethodPost, "/api/kopru/isler/sonuc", map[string]any{
		"sonuclar": sonuclar,
	}, &cevap, 20*time.Second)
	if err != nil {
		return 0, err
	}
	return cevap.Islenen, nil
}

// SurumBilgi — GET /api/kopru/surum (güncelleme uyarısı için).
type SurumBilgi struct {
	Surum      string `json:"surum"`
	IndirmeURL string `json:"indirme_url"`
}

// Surum — sunucudaki güncel ajan sürümü. GitHub API'ye GİTMEZ (anonim 60/saat
// IP limiti 1000 ajanda patlar) — kendi backend'imiz proxy'dir.
func (c *Client) Surum() (*SurumBilgi, error) {
	var bilgi SurumBilgi
	if err := c.istek(http.MethodGet, "/api/kopru/surum", nil, &bilgi, 15*time.Second); err != nil {
		return nil, err
	}
	return &bilgi, nil
}

// Kaldir — POST /api/kopru/kaldir (X-Kopru-Token). Installer'ın kaldırma
// akışı ("--kaldir-sunucu", bkz. cmd/hizmetra-kopru/kurulum.go) çağırır:
// sunucudaki cihaz kaydını siler ve token'ı geçersiz kılar — kullanıcı
// panelden elle silmek zorunda kalmaz. Gövde yok, cevap gövdesi kullanılmaz;
// `istek` zaten 401'i ErrYetkisiz'e çevirir. Çağıran taraf HER hatayı
// (401, 404 — ör. bu uç henüz yayında olmayan bir sunucuda, bağlantı hatası…)
// sessizce yutar; burada özel bir dal YOKTUR.
func (c *Client) Kaldir() error {
	return c.istek(http.MethodPost, "/api/kopru/kaldir", nil, nil, 15*time.Second)
}
