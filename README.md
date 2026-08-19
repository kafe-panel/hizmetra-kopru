# Hizmetra Yazıcı

Kafe bilgisayarındaki **fiş yazıcılarını Hizmetra Panel'e bağlayan** küçük bir Windows programı.

Panelden gelen sipariş/kasa fişlerini alır ve yazıcınıza basar. Bulut sunucusu kafenizin
yerel ağına **erişmez** — bağlantıyı her zaman bu program başlatır (giden HTTPS), bu yüzden
modem/router ayarı, port açma veya sabit IP **gerekmez**.

## Sistem gereksinimleri

| | |
|---|---|
| **Windows** | **10 veya 11** (Windows 7 / 8 / 8.1 desteklenmez) |
| **Mimari** | 64-bit **ve** 32-bit — kurulum paketi ikisini de taşır, doğru olanı kendi seçer |
| **İnternet** | Giden HTTPS yeterli; port açma/sabit IP gerekmez |
| **Diğer** | Arayüz penceresi için Edge WebView2; yoksa kurulum sırasında otomatik kurulur |

> **Windows 7/8/8.1 neden desteklenmiyor?** Program Go ile derleniyor; Go 1.21'den bu yana
> üretilen çalıştırılabilir dosyalar Windows 10 / Server 2016 ve üstünü zorunlu kılıyor.
> Bu sürümlerde kurulum baştan durur ve nedenini söyler (sessizce açılmayan bir program
> kurmaktansa açık bir uyarı vermek daha doğru).

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

1. **Windows sürümü.** `Windows tuşu + R` → `winver` → Enter. Sürüm **10** veya **11**
   değilse program çalışmaz (bkz. Sistem gereksinimleri). Kurulum paketi bunu artık
   baştan söyler; daha eski bir paketle kurduysanız sessizce açılmamış olabilir.
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
