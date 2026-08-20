; Hizmetra Yazıcı — ESKİ WINDOWS installer (Windows 7 SP1 / 8 / 8.1 uyumlu).
;
; NEDEN AYRI (2026-08-20): Ana installer (hizmetra-yazici.iss) Go 1.21+ ile
; derlenen ikiliyi taşır ve InitializeSetup'ta Windows 10 GEREKTİRİR (eski
; Windows'ta exe hiç açılmaz). Bu installer ise AYNI uygulamanın (cmd/hizmetra-
; kopru) Go **1.20** ile derlenmiş ikilisini taşır — Win7 SP1'den itibaren
; ÇALIŞIR ve Win10/11'de MODERN sürümle AYNI native pencereyi (WebView2 + tepsi
; ikonu) açar. WebView2 kurulu değilse (yalnız Win7/8) tarayıcı düşüşüne geçer.
; Bu yüzden:
;   - Windows 10 kapısı YOK (InitializeSetup sürüm kontrolü kaldırıldı),
;   - WebView2 kurulum adımı YOK (Win7/8'de WebView2 desteklenmez; tarayıcı düşüşü
;     zaten var — Win10/11'de kullanıcı isterse WebView2'yi ayrıca kurabilir),
;   - MinVersion=6.1sp1 (Win7 SP1) — daha eskisinde Inno zaten reddeder.
;
; TEKNİK KİMLİK ana installer'la AYNI: kurulan dosya hizmetra-kopru.exe,
; mutex Global\HizmetraKopruTekKopya, {app}={localappdata}\HizmetraYazici. Bir
; makine ana VEYA eski installer'dan yalnız birini kullanır (Win10+ ana, Win<10
; eski); ikisi aynı AppId'yi paylaşır → Denetim Masası'nda tek "Hizmetra Yazıcı".
;
; CI bunu .github/workflows/release.yml "legacy-installer" job'ında
; (windows-latest + innosetup 6.7.1) derler; eski ikililer AYNI dizine
; (installer\) indirilir.

#define MyAppName "Hizmetra Yazıcı"
#ifndef MyAppVersion
  #define MyAppVersion "0.8.0"
#endif
#define MyAppPublisher "Hizmetra"
#define MyAppExeName "hizmetra-kopru.exe"

[Setup]
; Ana installer ile AYNI AppId (tek ürün kimliği — makine birini kullanır).
AppId={{D0752452-C0F4-4F3D-90F9-35A529466B16}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName}
AppPublisher={#MyAppPublisher}
DefaultDirName={localappdata}\HizmetraYazici
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
Uninstallable=yes
OutputDir=Output
OutputBaseFilename=HizmetraYaziciKurulum-EskiWindows
SetupIconFile=..\assets\hizmetra.ico
UninstallDisplayIcon={app}\hizmetra.ico
; Windows 7 SP1 asgari. Inno bundan eskisinde (Vista/XP) kurulumu reddeder;
; zaten Go 1.20 ikilisi de Win7 SP1 altında güvenilir değildir.
MinVersion=6.1sp1
; Çalışan uygulamayı kapatma emniyet kemeri (RM + dosya kilidi); AppMutex
; KASITLI YOK (ana installer ile aynı gerekçe — bloke edici diyalog çıkmasın).
CloseApplications=yes
RestartApplications=no
ShowLanguageDialog=no

[Languages]
Name: "tr"; MessagesFile: "compiler:Languages\Turkish.isl"

[Tasks]
Name: "desktopicon"; Description: "Masaüstünde kısayol oluştur"; GroupDescription: "Ek görevler:"
Name: "startupicon"; Description: "Windows açılışında otomatik başlat"; GroupDescription: "Ek görevler:"

[Files]
; MİMARİYE GÖRE TEK İKİLİ (ana installer ile aynı desen): 64-bit uyumlu makineye
; amd64, değilse 32-bit ikili YAZILIR — her ikisi de {app}\hizmetra-kopru.exe adıyla.
; Eski ikililer CI'da installer\ dizinine indirilir (release.yml legacy-installer).
Source: "hizmetra-kopru-eski.exe"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion; Check: IsX64Compatible
Source: "hizmetra-kopru-eski_windows_386.exe"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion; Check: not IsX64Compatible
Source: "..\assets\hizmetra.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\hizmetra.ico"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; IconFilename: "{app}\hizmetra.ico"; Tasks: desktopicon

[Registry]
; Windows açılışında otomatik başlat (HKCU Run, "--autostart" → sessiz arka plan).
; Ana installer ile aynı değer adı ve deseni.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "HizmetraYazici"; ValueData: """{app}\{#MyAppExeName}"" --autostart"; Tasks: startupicon; Flags: uninsdeletevalue

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent
Filename: "{app}\{#MyAppExeName}"; Flags: nowait; Check: WizardSilent

[UninstallRun]
; Kaldırmanın İLK adımı (dosyalar silinmeden önce): cihaz kaydını sunucudan sil.
Filename: "{app}\{#MyAppExeName}"; Parameters: "--kaldir-sunucu"; RunOnceId: "KaldirSunucu"; Flags: runhidden

[UninstallDelete]
; Kaldırınca ayar/log/token klasörünü de sil → yeniden kur = temiz başlangıç.
Type: filesandordirs; Name: "{userappdata}\HizmetraKopru"

[Code]
// Çalışan kopyayı otomatik kapat (kilitli exe kaldırma/güncellemede silinebilsin).
// Ana installer'daki KapatCalisanUygulama'nın sadeleştirilmiş hâli — mevcut-kurulum
// paneli ve WebView2 adımı bu (eski Windows) installer'da YOK.
procedure KapatCalisanUygulama;
var
  SonucKodu: Integer;
begin
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/IM {#MyAppExeName} /T', '',
    SW_HIDE, ewWaitUntilTerminated, SonucKodu);
  Sleep(1200);
  Exec(ExpandConstant('{sys}\taskkill.exe'), '/F /IM {#MyAppExeName} /T', '',
    SW_HIDE, ewWaitUntilTerminated, SonucKodu);
  Sleep(400);
end;

// PrepareToInstall — dosya kopyalamadan hemen önce çalışan kopyayı kapat.
function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  KapatCalisanUygulama;
end;
