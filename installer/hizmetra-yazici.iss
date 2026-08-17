; Hizmetra Yazıcı — Inno Setup installer (v0.4.0, plan Track C4).
;
; Tasarım ilkeleri:
;   - Yönetici hakkı GEREKMEZ (PrivilegesRequired=lowest) — mevcut "3 dakikada
;     kurulum" ilkesi korunur (bkz. cmd/hizmetra-kopru/platform_windows.go).
;   - TEKNİK KİMLİK değişmiyor: kurulan dosyanın adı hâlâ hizmetra-kopru.exe
;     (mutex adı Global\HizmetraKopruTekKopya de öyle) — yalnız GÖRÜNEN adlar
;     (kurulum dizini, Start Menu, Denetim Masası) "Hizmetra Yazıcı".
;   - Autostart artık BURADA yazılıyor ([Registry], startupicon task'ına bağlı)
;     — Go tarafındaki eski otomatikBaslatKur/Kaldir/Yolu fonksiyonları bu
;     yüzden kaldırıldı (bkz. platform_windows.go).
;   - Kaldırma [UninstallRun] TANIM GEREĞİ kaldırmanın İLK adımı olarak çalışır
;     (dosyalar silinmeden ÖNCE) — exe "--kaldir-sunucu" ile çağrılır, sunucudaki
;     cihaz kaydını sessizce siler (bkz. main.go, internal/api/client.go Kaldir()).
;
; CI bu dosyayı .github/workflows/release.yml'deki "installer" job'ında
; (windows-latest + chocolatey innosetup) derler; hizmetra-kopru.exe bu script
; ile AYNI dizine indirilir (bkz. [Files] Source altında, alt dizin YOK).

#define MyAppName "Hizmetra Yazıcı"
; CI etiketten "iscc /DMyAppVersion=X.Y.Z ..." ile geçer (bkz. release.yml,
; installer job) — /D zaten tanımlarsa #ifndef bu satırı atlar (redefinition
; hatası yok). Yerel ad-hoc derlemede (/D verilmeden) 0.4.0'a düşer.
#ifndef MyAppVersion
  #define MyAppVersion "0.4.0"
#endif
#define MyAppPublisher "Hizmetra"
#define MyAppExeName "hizmetra-kopru.exe"

[Setup]
AppId={{D0752452-C0F4-4F3D-90F9-35A529466B16}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
; AppVerName varsayılanı "{AppName} version {AppVersion}" — Denetim Masası'nda
; DisplayName'i bu üretir (empirik doğrulandı, yerel derleme). emre'nin kabul
; testi TAM "Hizmetra Yazıcı" arıyor (bkz. plan Task C4 Step 6) — sürüm eki
; OLMADAN sabitleniyor.
AppVerName={#MyAppName}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\HizmetraYazici
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
Uninstallable=yes
OutputDir=Output
OutputBaseFilename=HizmetraYaziciKurulum
SetupIconFile=..\assets\hizmetra.ico
; Denetim Masası → Programlar listesinde kaldırma girdisinin SİMGESİ. Bu satır
; olmadan Inno jenerik bir simge gösteriyordu (emre 2026-08-16: "denetim
; masasında logosu yok"). {app}\hizmetra.ico [Files] ile kopyalanır (aşağıda).
UninstallDisplayIcon={app}\hizmetra.ico
; AppMutex — uygulamanın tuttuğu tek-kopya mutex'iyle BİREBİR aynı ad
; (cmd/hizmetra-kopru/platform_windows.go: mutexAdi = "Global\HizmetraKopruTekKopya").
; Böylece installer (kurulum VEYA in-app güncelleme sırasında) çalışan uygulamayı
; ALGILAR ve "Hizmetra Yazıcı çalışıyor, lütfen kapatın" der — exe dosyasını
; kilitliyken üstüne yazmaya çalışıp başarısız olmaz. In-app güncelleme akışında
; (guncelle(), main.go) uygulama installer'ı başlatmadan HEMEN ÖNCE mutex'i bırakıp
; kendini kapattığı için bu kontrol geçilir; kalan tek risk elle kurulumdur.
AppMutex=Global\HizmetraKopruTekKopya
; Kayıt+kısayol dosyaları küçük; sıkıştırma ayarı gerekmez (Inno varsayılanı yeterli).

[Tasks]
Name: "desktopicon"; Description: "Masaüstünde kısayol oluştur"; GroupDescription: "Ek görevler:"
; desktopicon artık VARSAYILAN İŞARETLİ (Flags: unchecked KALDIRILDI) — emre
; 2026-08-16: "kısayol masaüstümde görünmüyor". Kullanıcı istemezse sihirbazda
; işareti kaldırır. Flags YOK = Inno varsayılanı "checked" (işaretli) — "checked" diye ayrı bir
; bayrak YOKTUR (Inno docs: [Tasks] Flags yalnız checkablealone/checkedonce/
; dontinheritcheck/exclusive/restart/unchecked tanır). checkedonce de burada
; KASITLI kullanılmadı: o, "önceki sürüm bulunduysa İLK AÇILIŞTA unchecked"
; anlamına gelir — autostart gibi KALICI bir kullanıcı tercihi için yanlış
; olurdu (her güncellemede sessizce KAPATIRDI). Boş bırakmak hem ilk kurulumda
; işaretli başlatır HEM upgrade'de Inno'nun önceki seçimi hatırlamasına izin verir.
Name: "startupicon"; Description: "Windows açılışında otomatik başlat"; GroupDescription: "Ek görevler:"

[Files]
Source: "hizmetra-kopru.exe"; DestDir: "{app}"; Flags: ignoreversion
; hizmetra.ico'yu {app}'e kopyala → kısayol + Denetim Masası simgeleri BUNDAN
; okunur (gömülü exe simgesine güvenmek yerine, ki Windows Search önbelleği
; bazen jenerik gösteriyordu — emre "logosu yok"). Tek doğruluk kaynağı.
Source: "..\assets\hizmetra.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\hizmetra.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\hizmetra.ico"; Tasks: desktopicon

[Registry]
; v0.3'te Go'nun otomatikBaslatKur'unun yazdığı AYNI desen (HKCU Run, tırnaklı
; exe yolu) — artık Go değil Inno yazıyor. Değer adı bilinçli olarak
; "HizmetraYazici" (v0.1-v0.3'ün Go'da yazdığı "HizmetraKopru" değeri İLE
; ÇAKIŞMAZ — teknik kimlik mutex adında zaten korunuyor, Run değer ADI
; kozmetiktir). uninsdeletevalue: Kaldır'da bu satır da temizlenir.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "HizmetraYazici"; ValueData: """{app}\{#MyAppExeName}"""; Tasks: startupicon; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; [UninstallRun] kaldırmanın İLK adımı olarak çalışır (dosyalar silinmeden
; ÖNCE) — Inno Setup docs, "[Run] & [UninstallRun] sections": UninstallRun
; entries execute "as the first step of uninstallation." Bu yüzden exe hâlâ
; diskte var olduğu an çağrılıyor. runhidden: pencere/konsol AÇILMAZ — sessiz,
; hızlı arka plan çağrısı (main.go: hata/token yokluğu dahil her durumda
; sessizce os.Exit(0)). RunOnceId: aynı kaldırmada birden fazla tetiklenmesin.
Filename: "{app}\{#MyAppExeName}"; Parameters: "--kaldir-sunucu"; RunOnceId: "KaldirSunucu"; Flags: runhidden

[UninstallDelete]
; Kaldırınca ayar/log klasörünü de sil (%APPDATA%\HizmetraKopru): token + kod +
; günlük. Böylece KALDIR sonrası YENİDEN KUR = TEMİZ başlangıç (boş token →
; program kod sorar). Bu olmadan eski/GEÇERSİZ token kalıyor ve program
; "Eşleştirme geçersiz"de takılıp kod sormuyordu (emre 2026-08-17). Not:
; GÜNCELLEME (installer'ın üstüne kurulumu) kaldırma çalıştırmaz → eşleşme KORUNUR;
; yalnız gerçek KALDIR temizler — ki o zaten --kaldir-sunucu ile cihazı da siler.
Type: filesandordirs; Name: "{userappdata}\HizmetraKopru"
