//go:build darwin && cgo

// Package pencere — macOS'ta ajanın durum/kurulum ekranlarını GERÇEK bir
// uygulama penceresinde (WKWebView) gösterir; sistem tarayıcısında DEĞİL.
//
// emre 2026-08-21: "yazılım olarak indirdiği şey internette açılmasın,
// bilgisayarında açılsın". Windows'ta bu WebView2 ile zaten yapılıyordu;
// macOS'ta pencere paketi no-op olduğu için arayüz Safari sekmesinde açılıyordu.
// Artık her iki platform da native pencere kullanır (Linux'ta hâlâ tarayıcı
// düşüşü var — WebKitGTK cgo bağımlılığı statik Linux ikilisini bozardı).
//
// `cgo` build etiketi ŞART: CGO_ENABLED=0 ile darwin derlemesi yapıldığında
// (CI'nin hızlı çapraz-derleme adımı) bu dosya DIŞLANIR ve pencere_diger.go'daki
// tarayıcı-düşüşü stub'ı devreye girer.
package pencere

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>

void pencereAcObjC(const char *cURL, int genislik, int yukseklik);
void pencereOneGetirObjC(void);
void pencereKapatObjC(void);
*/
import "C"

import (
	"errors"
	"sync"
	"unsafe"
)

var (
	mu   sync.Mutex
	acik bool
)

// pencereKapandiGo — Objective-C vekili (windowWillClose) çağırır.
// Kurulum sihirbazı "kullanıcı pencereyi kapatıp vazgeçti mi" sorusunu bu
// duruma bakarak cevaplar.
//
//export pencereKapandiGo
func pencereKapandiGo() {
	mu.Lock()
	acik = false
	mu.Unlock()
}

// Ac — pencereyi açar (zaten açıksa adrese yönlendirip öne getirir).
//
// Windows sürümünden DAVRANIŞ FARKI: burada pencere oluşturma ana iş
// parçacığındaki AppKit döngüsüne devredilir ve BEKLENMEZ; çağrı hemen nil
// döner. Beklemek ölümcül olurdu: tepsi döngüsü henüz başlamadıysa ana kuyruk
// boşaltılmaz ve Ac sonsuza dek asılırdı. Bu yüzden main.go macOS'ta TÜM UI
// akışını systray.Run(onReady) sonrasına alır (bkz. uiTrayIcinde).
//
// WKWebView macOS 10.10'dan beri her sürümde vardır; oluşturma pratikte
// başarısız olmaz — bu yüzden "açılamadı" düşüşü (tarayıcı) gerekmez.
func Ac(url string, genislik, yukseklik int) error {
	cURL := C.CString(url)
	defer C.free(unsafe.Pointer(cURL))

	mu.Lock()
	acik = true
	mu.Unlock()

	C.pencereAcObjC(cURL, C.int(genislik), C.int(yukseklik))
	return nil
}

// OneGetir — açık pencereyi öne getirir. Pencere kapalıysa hata döner; çağıran
// bunu "kullanıcı vazgeçti / pencere yok" sinyali olarak kullanır.
func OneGetir() error {
	mu.Lock()
	acikMi := acik
	mu.Unlock()
	if !acikMi {
		return errors.New("pencere: açık değil")
	}
	C.pencereOneGetirObjC()
	return nil
}

// Kapat — açık pencereyi kapatır (kapalıysa no-op).
func Kapat() {
	C.pencereKapatObjC()
}
