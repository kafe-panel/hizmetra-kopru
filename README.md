# Hizmetra Yazıcı

Kafe bilgisayarındaki **fiş yazıcılarını Hizmetra Panel'e bağlayan** küçük bir Windows programı.

Panelden gelen sipariş/kasa fişlerini alır ve yazıcınıza basar. Bulut sunucusu kafenizin
yerel ağına **erişmez** — bağlantıyı her zaman bu program başlatır (giden HTTPS), bu yüzden
modem/router ayarı, port açma veya sabit IP **gerekmez**.

## Sistem gereksinimleri

Her işletim sistemi ve sürümü için bir paket var — panelde bilgisayarınıza uygun kart
işaretli gelir.

| İşletim sistemi | İndirilecek paket |
|---|---|
| **Windows 10 / 11**, Server 2016+ | `HizmetraYaziciKurulum.exe` |
| **Windows 7 SP1 / 8 / 8.1**, Server 2008 R2 – 2012 R2 | `HizmetraYaziciKurulum-EskiWindows.exe` |
| **macOS 10.15 Catalina ve üstü** (Intel + Apple Silicon) | `HizmetraYazici.dmg` |
| **macOS 10.13 High Sierra / 10.14 Mojave** (Intel) | `HizmetraYazici-EskiMac.dmg` |
| **Linux** — Debian/Ubuntu · Fedora/RHEL/openSUSE · Alpine · Arch | `.deb` · `.rpm` · `.apk` · `archlinux` |

| | |
|---|---|
| **Mimari** | Windows: 64-bit **ve** 32-bit (kurulum doğru olanı kendi seçer) · macOS: Intel + Apple Silicon · Linux: amd64, arm64, i386, armv7 (Raspberry Pi) |
| **İnternet** | Giden HTTPS yeterli; port açma/sabit IP gerekmez |
| **Windows** | Arayüz penceresi için Edge WebView2; yoksa kurulum sırasında otomatik kurulur (Win7/8'de WebView2 yoktur → arayüz tarayıcıda açılır) |
| **Linux** | `cups-client` + `xdg-utils` (paket kendisi kurar). GNOME'da tepsi simgesi için AppIndicator eklentisi gerekir; Ubuntu'da hazır gelir |

> **Neden iki ayrı Windows / macOS paketi?** Program Go ile derleniyor ve Go'nun taban
> işletim sistemi sürümü zamanla yükseldi: Go 1.21+ ile üretilen ikili Windows 10 /
> macOS 10.15 ve üstünü zorunlu kılıyor. Eski makineleri desteklemek için o kanal
> **Go 1.20** ile ayrıca derleniyor (`-X main.Kanal=eski`). Yanlış paketi indirirseniz
> kurulum baştan durur ve doğrusunu söyler. Eski paket yeni bir makinede çalışıyorsa
> ajan kendini otomatik olarak modern kanala terfi ettirir.

## Kurulum (3 dakika)

1. **[Programı indirin](https://github.com/kafe-panel/hizmetra-kopru/releases/latest)** → `hizmetra-kopru.exe`
2. Çift tıklayıp çalıştırın.
   > Windows "Bilgisayarınız korundu" uyarısı verirse: **Ek bilgi → Yine de çalıştır**.
   > (Program henüz dijital imzalı değil; imza başvurumuz sürüyor. Kaynak kodun tamamı bu depoda açık.)
3. Panelde **Ayarlar → Yazıcılar → Bilgisayar Programı** bölümündeki **6 haneli kurulum kodunu** girin.
4. Bitti. Panelde **Yeni Yazıcı → Bulunan Yazıcılar**'a tıklayın; bilgisayardaki yazıcılar listelenir, seçin.

Program bundan sonra bilgisayar açıldığında kendiliğinden çalışır ve saat yanındaki
sistem tepsisinde durur.

## Neler yapar

- Panelden bekleyen fişleri çeker (mutfak, kasa, paket, iade, test)
- **USB/yerel yazıcılara** Windows üzerinden ham (RAW) ESC/POS basar — sürücü render'ını atlar
- **Ağ yazıcılarına** doğrudan `IP:9100` ile basar — sürücü bile gerekmez
- Bilgisayardaki yazıcıları panele bildirir ("Bulunan Yazıcılar" listesi)
- Baskı gerçekten yapıldığında panele **dürüst** sonuç bildirir (basıldı / hata + sebep)

## Sistem tepsisi menüsü

| Menü | Ne yapar |
|---|---|
| Durum satırı | Bağlı mı, kaç yazıcı görüyor, son fiş ne zaman basıldı |
| Paneli Aç | Hizmetra Panel'i tarayıcıda açar |
| Günlüğü Aç | Teknik günlük dosyası (sorun bildirirken işe yarar) |
| Çıkış | Programı kapatır — **kapalıyken fiş basılmaz** |

## Sık karşılaşılanlar

**"Kod geçersiz" diyor.** Kodun ömrü 10 dakikadır ve tek kullanımlıktır. Panelden yeni kod alın.

**Yazıcı listede yok.** Yazıcının Windows'ta kurulu ve açık olduğundan emin olun, sonra
programı Çıkış'tan kapatıp yeniden açın. Ağ yazıcısı için panele `192.168.1.50:9100`
biçiminde elle yazabilirsiniz.

**Fişler basılmıyor.** Tepsi simgesindeki durum satırına bakın. "Bağlantı bekleniyor"
yazıyorsa internet, "⚠" yazıyorsa mesajdaki hata (kağıt bitti, yazıcı kapalı vb.) sebeptir.

**Bilgisayarı değiştirdim.** Eski bilgisayarı panelden **Kaldır** deyin (bağlantısı anında kesilir),
yenisinde yeni kurulum kodu ile kurun.

**Kurulum bitti ama program açılmıyor.** Sırayla kontrol edin:

1. **Yanlış paket olabilir.** `Windows tuşu + R` → `winver` → Enter. Sürüm **10/11**
   değilse (7, 8, 8.1) `HizmetraYaziciKurulum-EskiWindows.exe` paketini indirin —
   normal paket o sürümlerde açılmaz. Mac'te  → **Bu Mac Hakkında**: 10.13/10.14
   ise `HizmetraYazici-EskiMac.dmg` gerekir. Kurulum paketi yanlış paketi artık
   baştan reddedip doğrusunu söyler.
2. **Zaten çalışıyor olabilir.** Saatin yanındaki **gizli simgeler (^)** okunu açın;
   Hizmetra Yazıcı simgesi oradaysa program açık demektir — simgeye tıklayın.
3. **Antivirüs silmiş olabilir.** Program henüz dijital imzalı olmadığı için bazı
   antivirüsler kurulumdan sonra dosyayı karantinaya alıyor.
   `%LOCALAPPDATA%\HizmetraYazici\hizmetra-kopru.exe` yoksa sebep budur: antivirüste
   bu klasörü izin listesine ekleyip kurulumu tekrarlayın.
4. **Günlük dosyası.** `%APPDATA%\HizmetraKopru\` klasöründe günlük varsa program en az
   bir kez **açılmış** demektir — dosyanın son satırlarını destek ile paylaşın. Günlük hiç
   yoksa program hiç başlayamamış demektir (1-3. maddeler).

## Gizlilik

- Program yalnız **giden** bağlantı kurar; dışarıdan bilgisayarınıza erişilmez.
- Fiş içeriği **günlüğe yazılmaz** (yalnız iş numarası, hedef yazıcı ve bayt sayısı).
- Ayar dosyası yalnız Windows kullanıcınızın erişebildiği klasördedir:
  `%APPDATA%\HizmetraKopru\config.json`
- Cihaz anahtarı yalnız **baskı** yetkisi verir; satış/müşteri verisine erişemez.
  Panelden **Kaldır** dediğiniz anda geçersiz olur.

## Geliştirici

```bash
go test ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w -X main.Surum=$(git describe --tags)" -o dist/hizmetra-kopru.exe ./cmd/hizmetra-kopru
# 32-bit Windows icin:
CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-H windowsgui -s -w -X main.Surum=$(git describe --tags)" -o dist/hizmetra-kopru_windows_386.exe ./cmd/hizmetra-kopru
```

Farklı sunucuya bağlanmak için: `HIZMETRA_API=http://localhost:5002` ortam değişkeni.

## Lisans

MIT — bkz. [LICENSE](LICENSE).

---

## English summary

**Hizmetra Yazıcı** ("Hizmetra Printer"; the repo/exe keep the original technical name
`hizmetra-kopru`, "bridge") is a small Windows tray application that connects
thermal receipt printers on a café's local network to the Hizmetra Panel cloud POS.
It polls the server over outbound HTTPS for pending print jobs and writes raw ESC/POS
bytes either to a Windows printer (RAW spooler mode, bypassing driver rendering) or
directly to a network printer over TCP port 9100. No inbound connections, no router
configuration. Pairing is done with a one-time 6-digit code shown in the panel.
