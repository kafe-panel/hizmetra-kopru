//go:build windows

package pencere

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jchv/go-webview2/webviewloader"
)

// Bu testler GERÇEK Windows'ta koşar (geliştirici makinesi) — CI Linux'ta bu
// dosya hiç derlenmez (bkz. .github/workflows/ci.yml: go test ./... Linux'ta,
// yalnız CGO_ENABLED=0 GOOS=windows çapraz DERLEMESİ ayrıca doğrulanıyor).

// TestOneGetirVeKapatPencereKapaliyken — pencere hiç açılmamışken OneGetir
// hata döner, Kapat panic atmadan no-op'tur. Paket başlangıç durumunu sınar;
// TestAcGercekPencereAcarKapatir'DAN ÖNCE çalışması için dosyada önce yazıldı
// (go test aynı dosyada kaynak sırasıyla koşar).
func TestOneGetirVeKapatPencereKapaliyken(t *testing.T) {
	if err := OneGetir(); err == nil {
		t.Error("pencere kapalıyken OneGetir hata dönmeliydi")
	}
	Kapat() // no-op olmalı, panic atmamalı
}

// TestAcGercekPencereAcarKapatir — WebView2 Runtime bu makinede kuruluysa
// GERÇEK bir native pencere açar (yerel httptest sunucusunu yükler), açık
// olduğunu doğrular, kapatır ve durumun temizlendiğini doğrular. Runtime
// yoksa (ör. kısıtlı CI imajı) atlanır — GetInstalledVersion çökmeden
// kontrol eder (bkz. pencere_windows.go üstteki yorum).
func TestAcGercekPencereAcarKapatir(t *testing.T) {
	if surum, err := webviewloader.GetInstalledVersion(); err != nil || surum == "" {
		t.Skip("WebView2 Runtime kurulu değil, atlanıyor")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html><body>pencere testi</body></html>"))
	}))
	defer srv.Close()

	sonuc := make(chan error, 1)
	go func() { sonuc <- Ac(srv.URL, 400, 300) }()

	select {
	case err := <-sonuc:
		if err != nil {
			t.Fatalf("Ac hata döndü: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Ac 15 saniyede dönmedi (pencere oluşturma takıldı)")
	}

	if err := OneGetir(); err != nil {
		t.Errorf("OneGetir hata döndü (pencere açık olmalıydı): %v", err)
	}

	// Aynı adrese ikinci Ac çağrısı YENİ pencere AÇMAMALI (tekil pencere
	// davranışı) — hatasız dönmesi yeterli kanıt (mevcut pencereyi öne getirir).
	if err := Ac(srv.URL, 400, 300); err != nil {
		t.Errorf("pencere açıkken ikinci Ac hata döndü: %v", err)
	}

	Kapat()

	deadline := time.Now().Add(5 * time.Second)
	for OneGetir() == nil {
		if time.Now().After(deadline) {
			t.Fatal("Kapat sonrası pencere durumu (5sn içinde) temizlenmedi")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
