//go:build windows

// Package pencere — ajanın durum/kurulum ekranlarını GERÇEK bir Windows
// penceresinde (WebView2/Edge motoru) gösterir; sistem tarayıcısında DEĞİL
// (emre 2026-08-16: "internette açıyor arayüzü, sakın ha böyle bir şey
// olmasın, bilgisayardaki uygulamada görünmesi gerek"). "Paneli Aç" tray
// menüsü BUNUN DIŞINDA kalır — o gerçekten dış web sitesini açar, kendi
// tarayicidaAc() işlevini kullanmaya devam eder (main.go, DEĞİŞMEDİ).
//
// Yalnız Windows derlemesinde etkindir (webview2 COM sarmalayıcısı Windows'a
// özgü, cgo GEREKTİRMEZ — golang.org/x/sys/windows syscall'larıyla, C
// derleyicisi olmadan CGO_ENABLED=0 ile çapraz derlenir; goreleaser'ın mevcut
// Windows pipeline'ı bu yüzden bozulmaz). Diğer platformlarda pencere_diger.go
// içindeki no-op'lar derlenir (CI Linux'ta koşar; ajan zaten Windows hedefli
// — platform_diger.go ile aynı desen).
package pencere

import (
	"errors"
	"runtime"
	"sync"
	"time"

	webview2 "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

// acOlusturZamanAsimi — pencere oluşturma bu sürede tamamlanmazsa Ac hata
// döner (çağıran tarayıcı düşüşüne geçer). WebView2 ortamı normalde
// milisaniyeler içinde hazır olur; bu yalnız GetMessageW döngüsünün
// (Embed içinde, kütüphane kodu) teorik olarak hiç geri çağrılmama
// ihtimaline karşı bir güvenlik ağı — tray tıklama goroutine'i sonsuza
// kadar kilitlenmesin diye.
const acOlusturZamanAsimi = 10 * time.Second

var (
	mu   sync.Mutex
	acik webview2.WebView // nil = pencere kapalı
	hwnd uintptr          // acik != nil iken geçerli pencere tutamağı
)

var (
	user32DLL           = windows.NewLazySystemDLL("user32.dll")
	procGetDpiForSystem = user32DLL.NewProc("GetDpiForSystem")
)

// dpiOlcek — sistem DPI ölçeği (1.0=%100, 1.5=%150…). GetDpiForSystem Win10
// 1607+ vardır; yoksa (win7/8) güvenli şekilde 1.0 döner.
//
// NEDEN GEREKLİ (emre 2026-08-18: "arayüz açılınca her şey görünmüyor, elle
// genişletmek gerekiyor"): manifest "system DPI aware" olduğundan Windows
// pencereyi DPI'ye göre OTOMATİK büyütmez; WebView2 içeriği ise sistem DPI'sinde
// render eder. Pencere FİZİKSEL boyutunu ölçekle ÇARPMAZSAK, %150 DPI'li bir
// ekranda 900px'lik pencerede içerik yalnız ~600 CSS px alan bulur → taşar,
// kullanıcı pencereyi elle büyütmek zorunda kalır. Ölçekle çarpınca içerik HER
// bilgisayarda AYNI CSS px alanını görür (istenen tasarım genişliği korunur).
// (Not: go-webview2 bugün DPI'yi kendi telafi etmiyor; ederse bu çarpım tek
// katman fazladan olurdu — sürüm yükseltirken kontrol et.)
func dpiOlcek() float64 {
	if procGetDpiForSystem.Find() != nil {
		return 1.0
	}
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return 1.0
	}
	return float64(dpi) / 96.0
}

// Ac bir pencere açar ve url'i yükler. Zaten açıksa YENİ PENCERE AÇMAZ: var
// olanı öne getirir ve aynı adrese yeniden yönlendirir — tekrar tıklama
// "öne getir" gibi davranır, kafa karıştırmaz.
//
// Şu anki çağıranlar (main.go: "Durumu Göster" tray tıklaması, kurulum
// akışları) tek bir goroutine'den SIRAYLA çağırır; eşzamanlı çağrı koruması
// bilerek eklenmedi (YAGNI — tasarım basit tutulur, bkz. plan Task C1).
//
// Pencere KENDİ iş parçacığında çalışır (goroutine + runtime.LockOSThread):
// Win32 pencere/mesaj döngüsü ömür boyu aynı OS iş parçacığını ister. Ac,
// pencere gerçekten açılana ya da açılamayacağı KESİNLEŞENE (hata ya da
// zaman aşımı) kadar bloklanır — çağıran böylece güvenle tarayıcı
// düşüşüne (main.go: tarayicidaAc) karar verebilir.
func Ac(url string, genislik, yukseklik int) error {
	mu.Lock()
	if acik != nil {
		w, h := acik, hwnd
		mu.Unlock()
		w.Dispatch(func() { w.Navigate(url) })
		_ = oneGetirHwnd(h)
		return nil
	}
	mu.Unlock()

	// Ön-kontrol: WebView2 Runtime kurulu değilse NewWithOptions'a HİÇ
	// girilmez. Kütüphane, ortam oluşturma async geri çağrısı hata dönerse
	// log.Fatal ile TÜM SÜRECİ kapatıyor (bkz. pkg/edge/chromium.go
	// EnvironmentCompleted/CreateCoreWebView2ControllerCompleted) — bu ajan
	// için kabul edilemez (arka planda sessiz çalışması gerekiyor).
	// GetInstalledVersion Microsoft'un belgelediği resmi ön-kontrol yoludur
	// ve HİÇBİR ZAMAN çökmez (dosya bulunamadı → boş sürüm + nil hata).
	if surum, err := webviewloader.GetInstalledVersion(); err != nil || surum == "" {
		return errors.New("WebView2 Runtime kurulu değil")
	}

	sonuc := make(chan error, 1)
	go pencereYasaDongusu(url, genislik, yukseklik, sonuc)
	select {
	case err := <-sonuc:
		return err
	case <-time.After(acOlusturZamanAsimi):
		return errors.New("pencere açma zaman aşımına uğradı")
	}
}

// pencereYasaDongusu — pencereyi oluşturur, sonucu sonuc'a yazar (Ac bunu
// bekliyor), sonra pencere kapanana kadar Win32 mesaj döngüsünde (Run)
// bloklanır. Bu goroutine'in iş parçacığı BİR DAHA KULLANILMAZ: LockOSThread
// hiç bırakılmaz — goroutine bitince Go çalışma zamanı iş parçacığını yok
// eder (belgelenmiş davranış, bkz. runtime.LockOSThread dokümantasyonu).
func pencereYasaDongusu(url string, genislik, yukseklik int, sonuc chan<- error) {
	runtime.LockOSThread()

	// go-webview2'nin paket init()'i (pkg/edge/corewebview2.go)
	// CoInitializeEx'i yalnız PROGRAMIN BAŞLANGIÇTAKİ iş parçacığında
	// çağırıyor — bu YENİ pencere iş parçacığında değil. WebView2'nin
	// tek-iş-parçacıklı-apartman (STA) beklentisiyle uyumlu olmak için
	// burada da çağrılır. Best-effort: kütüphanenin kendi init()'i de aynı
	// şekilde hatayı yalnız loglayıp (log.Printf, log.Fatal DEĞİL) devam
	// ediyor; biz de hatayı yutup deneriz — WebView2Loader kendi COM
	// ihtiyacını genelde ayrıca karşılar.
	_ = windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED)

	// DPI-duyarlı fiziksel boyut: içerik her ekranda 'genislik'×'yukseklik' CSS px
	// alan görsün (bkz. dpiOlcek — yüksek DPI'de taşma/elle-büyütme sorunu).
	olcek := dpiOlcek()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug: false, // sağ tık/DevTools KAPALI — üretim penceresi, "app" hissi
		WindowOptions: webview2.WindowOptions{
			Title:  "Hizmetra Yazıcı",
			Width:  uint(float64(genislik) * olcek),
			Height: uint(float64(yukseklik) * olcek),
			Center: true,
			// IconId=1: exe'ye winres/winres.json ile gömülen grup ikonun
			// SAYISAL kaynak kimliği. winres.json'da RT_GROUP_ICON anahtarı
			// "#1"dir — go-winres'te "#" öneki kaynağı SAYISAL ID yapar (öneksiz
			// "APP"/"1" gibi anahtarlar STRING ad olarak gömülür). go-webview2
			// pencere ikonunu LoadImageW + MAKEINTRESOURCE(IconId) ile yükler; bu
			// yalnız SAYISAL ID'yi çözer, STRING adı DEĞİL — bu yüzden anahtar
			// mutlaka "#1" olmalı. Bu olmadan pencere/görev-çubuğu Windows'un
			// jenerik varsayılan simgesini gösteriyordu (emre 2026-08-16: "dandik
			// bi uygulama gibi görünüyor, logomuz yok"). GERÇEK pencerede
			// WM_GETICON ile doğrulandı (0 → non-zero handle).
			IconId: 1,
		},
	})
	if w == nil {
		sonuc <- errors.New("webview penceresi oluşturulamadı")
		return
	}

	mu.Lock()
	acik = w
	hwnd = uintptr(w.Window())
	mu.Unlock()

	w.Navigate(url)
	sonuc <- nil

	w.Run() // pencere kapanana (Kapat ya da kullanıcı X'e basana) kadar bloklanır

	mu.Lock()
	acik = nil
	hwnd = 0
	mu.Unlock()
}

// OneGetir açık pencereyi (varsa simge durumundan) geri getirir ve ön plana
// alır. Pencere kapalıysa hata döner — çağıran (ör. ileride Task C3'ün
// /odaklan ucu) bunu "açık değil, normal başlangıca devam et" diye yorumlar.
func OneGetir() error {
	mu.Lock()
	h := hwnd
	acikMi := acik != nil
	mu.Unlock()
	if !acikMi {
		return errors.New("pencere açık değil")
	}
	return oneGetirHwnd(h)
}

// Kapat açık pencereyi kapatır. Kapalıysa no-op.
//
// w.Destroy() kullanılır, w.Terminate() DEĞİL — ikisi de "arka plan iş
// parçacığından güvenli" diye belgelense de yalnız Destroy GERÇEKTEN işe
// yarıyor: Win32'de PostQuitMessage (Terminate'in içi) HER ZAMAN ÇAĞIRANIN
// KENDİ iş parçacığının kuyruğuna postalanır — pencerenin sahibi olan iş
// parçacığına DEĞİL. Kapat, pencereYasaDongusu'nun çalıştığı iş parçacığından
// FARKLI bir goroutine'den çağrıldığında (ki normalde öyledir) Terminate()
// hiçbir şey yapmaz, Run() asla dönmez, acik/hwnd asla temizlenmez — bu,
// gerçek Windows'ta yazılan bir testle (pencere_windows_test.go) DENEYSEL
// OLARAK doğrulandı (Terminate ile 5sn içinde temizlenmedi). Destroy() ise
// PostMessageW(hwnd, WM_CLOSE, ...) yapar — Win32'nin mesaj kuyruğu HWND'nin
// sahibi iş parçacığına göre yönlendiği için HER ZAMAN doğru iş parçacığına
// ulaşır (wndproc WM_CLOSE→DestroyWindow→WM_DESTROY→w.Terminate() ZATEN
// doğru iş parçacığından, bu sefer gerçekten çalışır).
func Kapat() {
	mu.Lock()
	w := acik
	mu.Unlock()
	if w != nil {
		w.Destroy()
	}
}

// user32 doğrudan çağrıları — webview2.WebView arayüzü "öne getir" sağlamıyor
// (yalnız HWND'yi unsafe.Pointer olarak döndürüyor, bkz. Window()).
// golang.org/x/sys/windows bu ikisini sarmalamıyor; NewLazySystemDLL ile
// System32'den (DLL arama yolu ele geçirmeye kapalı) doğrudan yüklüyoruz.
var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

const swRestore = 9 // SW_RESTORE — simge durumuna küçültülmüşse geri getirir

// oneGetirHwnd — verilen pencere tutamağını geri getirir ve ön plana alır.
func oneGetirHwnd(h uintptr) error {
	if h == 0 {
		return errors.New("pencere tutamağı yok")
	}
	_, _, _ = procShowWindow.Call(h, uintptr(swRestore))
	_, _, _ = procSetForegroundWindow.Call(h)
	return nil
}
