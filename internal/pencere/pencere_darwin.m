// Hizmetra Yazıcı — macOS native durum/kurulum penceresi (WKWebView).
//
// Dosya adındaki "_darwin" soneki sayesinde go aracı bu dosyayı YALNIZ macOS
// derlemesinde kullanır (Windows/Linux build'leri CGO_ENABLED=0 zaten .m
// dosyalarını hiç görmez).
//
// ⚠️ ANA İŞ PARÇACIĞI KURALI: AppKit/WebKit nesneleri YALNIZ ana iş parçacığında
// ve ÇALIŞAN bir run loop varken kurulabilir. Ajan, tepsi (systray) döngüsünü
// ana iş parçacığında çalıştırır; bu yüzden buradaki her işlem
// dispatch_async(dispatch_get_main_queue(), ...) ile o döngüye devredilir.
// Go tarafı (pencere_darwin.go) çağrıları bu yüzden ASLA bloklamaz.
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include "_cgo_export.h"

static NSWindow *gPencere = nil;
static WKWebView *gWeb = nil;

// Pencere kapanınca Go tarafındaki "açık mı" durumunu temizler. Kurulum
// sihirbazı bu bilgiyle "kullanıcı vazgeçti mi" sorusunu cevaplar (bkz.
// cmd/hizmetra-kopru/main.go: ilkKurulum → pencere.OneGetir yoklaması).
@interface HizmetraPencereVekili : NSObject <NSWindowDelegate>
@end

@implementation HizmetraPencereVekili
- (void)windowWillClose:(NSNotification *)bildirim {
  (void)bildirim;
  gPencere = nil;
  gWeb = nil;
  pencereKapandiGo();
}
@end

static HizmetraPencereVekili *gVekil = nil;

void pencereAcObjC(const char *cURL, int genislik, int yukseklik) {
  NSString *adresMetni = [NSString stringWithUTF8String:cURL];
  dispatch_async(dispatch_get_main_queue(), ^{
    NSURL *adres = [NSURL URLWithString:adresMetni];
    if (adres == nil) {
      return;
    }
    // Zaten açıksa YENİ pencere açma: aynı pencereyi adrese yönlendirip öne al
    // (Windows tarafındaki davranışla birebir aynı).
    if (gPencere != nil && gWeb != nil) {
      [gWeb loadRequest:[NSURLRequest requestWithURL:adres]];
      [NSApp activateIgnoringOtherApps:YES];
      [gPencere makeKeyAndOrderFront:nil];
      return;
    }

    NSRect kare = NSMakeRect(0, 0, genislik, yukseklik);
    NSUInteger stil = NSWindowStyleMaskTitled | NSWindowStyleMaskClosable |
                      NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable;
    gPencere = [[NSWindow alloc] initWithContentRect:kare
                                           styleMask:stil
                                             backing:NSBackingStoreBuffered
                                               defer:NO];
    [gPencere setTitle:@"Hizmetra Yazıcı"];
    // Kapanınca nesne serbest BIRAKILMAZ: vekil geri çağrısı hâlâ ona dokunur.
    [gPencere setReleasedWhenClosed:NO];
    // Kullanıcı küçültse bile içerik okunur kalsın (emre: "ölçüler kayıp durmasın").
    [gPencere setMinSize:NSMakeSize(420, 420)];
    [gPencere center];

    WKWebViewConfiguration *yapilandirma = [[WKWebViewConfiguration alloc] init];
    gWeb = [[WKWebView alloc] initWithFrame:kare configuration:yapilandirma];
    [gWeb setAutoresizingMask:NSViewWidthSizable | NSViewHeightSizable];
    [gPencere setContentView:gWeb];

    if (gVekil == nil) {
      gVekil = [[HizmetraPencereVekili alloc] init];
    }
    [gPencere setDelegate:gVekil];

    [gWeb loadRequest:[NSURLRequest requestWithURL:adres]];
    // Uygulama LSUIElement (Dock'ta görünmez) olduğu için pencereyi öne almak
    // açıkça istenmelidir; yoksa arkada açılıp kullanıcı hiç göremez.
    [NSApp activateIgnoringOtherApps:YES];
    [gPencere makeKeyAndOrderFront:nil];
  });
}

void pencereOneGetirObjC(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    if (gPencere == nil) {
      return;
    }
    [NSApp activateIgnoringOtherApps:YES];
    [gPencere makeKeyAndOrderFront:nil];
  });
}

void pencereKapatObjC(void) {
  dispatch_async(dispatch_get_main_queue(), ^{
    if (gPencere != nil) {
      [gPencere close];
    }
  });
}
