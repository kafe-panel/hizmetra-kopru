; Hizmetra Yazıcı — Inno Setup installer (v0.7.0).
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
; v0.7.0 EKLERİ:
;   - Arayüz TÜRKÇE ([Languages] Turkish.isl — Inno 6.5+ RESMİ dili; CI 6.7.1'e
;     pinli, bkz. release.yml). ShowLanguageDialog=no + tek dil = tr.
;   - Mevcut kurulum bulununca ilk sayfada "Güncelle / Onar / Kaldır" paneli
;     ([Code], yalnız ETKİLEŞİMLİ modda; sessiz güncellemede ATLANIR).
;   - Sessiz oto-güncelleme: guncelle_windows.go installer'ı /VERYSILENT ...
;     /CLOSEAPPLICATIONS ile çağırır → kilitli exe RM ile serbest bırakılır;
;     kurulum sonrası [Run] "Check: WizardSilent" yeni sürümü DETERMİNİSTİK açar
;     (RestartApplications=no — RM yeniden başlatmasına GÜVENMEYİZ, biz hızlı
;     çıktığımız için RM kapatacak süreç bulamayabilir).
;
; CI bu dosyayı .github/workflows/release.yml'deki "installer" job'ında
; (windows-latest + chocolatey innosetup 6.7.1) derler; hizmetra-kopru.exe bu
; script ile AYNI dizine indirilir (bkz. [Files] Source altında, alt dizin YOK).

#define MyAppName "Hizmetra Yazıcı"
; CI etiketten "iscc /DMyAppVersion=X.Y.Z ..." ile geçer (bkz. release.yml,
; installer job) — /D zaten tanımlarsa #ifndef bu satırı atlar (redefinition
; hatası yok). Yerel ad-hoc derlemede (/D verilmeden) 0.7.0'a düşer.
#ifndef MyAppVersion
  #define MyAppVersion "0.8.0"
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
; AppMutex KASITLI YOK (emre 2026-08-18): AppMutex set edilseydi Inno, kurulu
; uygulama ÇALIŞIRKEN installer açılınca sihirbazdan ÖNCE "Uygulama çalışıyor,
; lütfen kapatın → Tamam/İptal" (SetupAppRunningError) bloke edici diyaloğunu
; gösterirdi. Kullanıcı bunu istemiyor: "uygulama yüklüyken böyle saçma bir şey
; çıkmasın, elle kapatmakla uğraşmayayım". Bunun yerine: mevcut kurulum bulununca
; DOĞRUDAN Güncelle/Onar/Kaldır paneli ([Code]) gelir; Güncelle/Onar/Kaldır
; seçilince [Code] çalışan uygulamayı OTOMATİK kapatır (taskkill graceful→force,
; bkz. KapatCalisanUygulama). CloseApplications de emniyet kemeri (RM, dosya kilidi).
CloseApplications=yes
; Yeniden başlatmayı RM'ye BIRAKMA (biz uygulamayı kendimiz kapatıyoruz → RM
; kapatacak süreç bulamaz); yeniden açmayı sessizde [Run] "Check: WizardSilent",
; etkileşimlide bitiş sayfasındaki "çalıştır" onay kutusu yapar.
RestartApplications=no
; Tek dil (Türkçe) → dil seçme diyaloğu gösterme.
ShowLanguageDialog=no

[Languages]
; Türkçe, Inno Setup 6.5+ ile gelen RESMİ çeviridir (compiler:Languages\Turkish.isl).
; Tüm yerleşik mesajlar (İleri/Geri, "uygulama çalışıyor", bitiş sayfası…) Türkçe.
Name: "tr"; MessagesFile: "compiler:Languages\Turkish.isl"

[CustomMessages]
; Projeye özel Türkçe metinler (mevcut-kurulum paneli — bkz. [Code]).
MevcutBaslik=Mevcut kurulum bulundu
MevcutAciklama=Hizmetra Yazıcı bu bilgisayarda zaten kurulu. Ne yapmak istersiniz?
SecGuncelle=Güncelle — yeni sürümü mevcut kurulumun üzerine kur
SecOnar=Onar — aynı sürümü yeniden kur
SecKaldir=Kaldır — Hizmetra Yazıcı'yı bu bilgisayardan sil

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
; MİMARİYE GÖRE TEK İKİLİ. Kurulum paketi hem 64-bit hem 32-bit ikiliyi TAŞIR,
; kurarken YALNIZ birini {app}\hizmetra-kopru.exe adıyla yazar. Öncesinde yalnız
; amd64 gömülüydü; 32-bit Windows'ta kurulum sorunsuz bitiyor ama exe HİÇ
; açılmıyordu ("Bu uygulama bilgisayarınızda çalıştırılamıyor") — kullanıcı
; bunu "kurdum ama açılmıyor" diye yaşıyordu (2026-08-20 canlı olay).
;
; IsX64Compatible (Inno 6.3+) = "x64 ikili bu makinede KOŞAR mı": gerçek x64
; makinelerde ve x64 öykünmesi olan ARM64 Windows'ta True. Win10 ARM64 yalnız
; x86 öykünür → orada False döner ve 32-bit ikili kurulur, ki doğrusu odur.
; Kurulan DOSYA ADI her iki dalda da AYNI (hizmetra-kopru.exe): mutex, Run
; anahtarı, kısayollar, --kaldir-sunucu ve oto-güncelleme bu ada bağlı.
Source: "hizmetra-kopru.exe"; DestDir: "{app}"; Flags: ignoreversion; Check: IsX64Compatible
Source: "hizmetra-kopru_windows_386.exe"; DestDir: "{app}"; DestName: "{#MyAppExeName}"; Flags: ignoreversion; Check: not IsX64Compatible
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
; ValueData'ya "--autostart" eklendi (v0.7.4): Windows açılışında OTOMATİK
; başlatmada exe bu bayrakla çağrılır → uygulama sessiz tepside kalır (pencere
; pop-up olmaz). Kullanıcı Başlat menüsü/masaüstü kısayoluna ELLE tıklayınca
; bayrak YOKTUR → durum penceresi açılır (emre 2026-08-18: "elle açtım, arayüz
; gelmedi"). Kısayollar ([Icons]) bayrak taşımaz; yalnız bu Run anahtarı taşır.
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "HizmetraYazici"; ValueData: """{app}\{#MyAppExeName}"" --autostart"; Tasks: startupicon; Flags: uninsdeletevalue

[Run]
; Etkileşimli kurulum bitişi: "Hizmetra Yazıcı'yı çalıştır" onay kutusu (sessizde
; atlanır — skipifsilent).
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#MyAppName}}"; Flags: nowait postinstall skipifsilent
; SESSİZ güncelleme yeniden-başlatması: /VERYSILENT ile kurulduğunda (oto-güncelleme
; yolu, guncelle_windows.go) yeni sürümü DETERMİNİSTİK aç. WizardSilent yerleşik
; support fonksiyonudur; yalnız sessiz modda True döner → etkileşimli kurulumda bu
; satır çalışmaz (onu üstteki postinstall onay kutusu yönetir).
Filename: "{app}\{#MyAppExeName}"; Flags: nowait; Check: WizardSilent

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

[Code]
// Mevcut kurulum paneli (emre 2026-08-17): installer İngilizceydi ve çalışan
// kopya bulununca yalnız "kapat + Tamam" diyordu; seçenek yoktu. Artık kurulu bir
// sürüm bulununca ETKİLEŞİMLİ kurulumun ilk sayfasında Güncelle/Onar/Kaldır
// radyo paneli çıkar. SESSİZ oto-güncelleme (/VERYSILENT) bu paneli ATLAR — hiçbir
// sayfa gösterilmez, yanlış dal (Kaldır) tetiklenmesin diye panel hiç KURULMAZ.

var
  MevcutPage: TInputOptionWizardPage;
  IndirmePage: TDownloadWizardPage;
  KaldirSecildi: Boolean;

// WebView2 Runtime'ın Evergreen sürüm anahtarı (Microsoft'un belgelediği GUID).
// Makine geneli kurulumda HKLM 32-bit görünümünde (64-bit Windows'ta
// WOW6432Node, 32-bit Windows'ta düz SOFTWARE — Inno'nun HKLM32'si ikisini de
// doğru çözer), kullanıcıya özel kurulumda HKCU altında durur.
const
  WV2Anahtar = 'SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  WV2AnahtarHKCU = 'Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}';
  // Evergreen Bootstrapper — Microsoft'un KALICI kısa bağlantısı (~2 MB indirir,
  // gerisini kendi çeker). Yönetici hakkı yoksa kullanıcıya özel kurulum yapar,
  // yani PrivilegesRequired=lowest ile uyumludur.
  WV2Bootstrapper = 'https://go.microsoft.com/fwlink/p/?LinkId=2124703';

// EnAzWindows10 — Windows sürüm kapısı.
//
// NEDEN VAR: uygulama Go ile derleniyor ve Go 1.21'den beri üretilen ikililer
// Windows 10 / Server 2016 ve üstünü ZORUNLU kılıyor (Windows 7/8/8.1 desteği
// Go tarafında kaldırıldı). Eski Windows'ta kurulum SORUNSUZ bitiyor, sonra exe
// hiç açılmıyor — kullanıcıya hiçbir açıklama gitmiyordu (2026-08-20 canlı
// olay: "kurulumu yaptım ama uygulama açılmıyor"). Artık kurulum en başta
// durur ve NEDENİNİ söyler.
//
// GetWindowsVersionEx GERÇEK sürümü verir (Inno'nun manifesti Windows 10/11
// uyumluluğunu beyan ettiği için GetVersionEx yalanına düşmez).
function EnAzWindows10: Boolean;
var
  Sur: TWindowsVersion;
begin
  GetWindowsVersionEx(Sur);
  Result := Sur.Major >= 10;
end;

function WebView2Kurulu: Boolean;
var
  Deger: string;
begin
  Result :=
    (RegQueryStringValue(HKLM32, WV2Anahtar, 'pv', Deger) and (Deger <> '') and (Deger <> '0.0.0.0')) or
    (RegQueryStringValue(HKCU, WV2AnahtarHKCU, 'pv', Deger) and (Deger <> '') and (Deger <> '0.0.0.0'));
end;

// WebView2Kur — arayüz motoru yoksa Microsoft'un bootstrapper'ını indirip
// SESSİZCE kurar. Uygulama arayüzünü WebView2 penceresinde gösteriyor; motor
// yoksa tarayıcı düşüşüne geçiyor (bkz. internal/pencere) — yani program
// ÇALIŞIR ama kullanıcı beklediği "bilgisayardaki uygulama" penceresini
// göremez. Windows 11 ve güncel Windows 10'da motor zaten kuruludur; eksik
// olduğu yerler LTSC/N sürümleri ve uzun süredir güncellenmemiş Windows 10
// kurulumları — tam da sahada sorun çıkan makineler.
//
// HER HATA YUTULUR ve kurulum DEVAM EDER: internet yoksa/indirme düşerse
// program yine kurulur, yalnız tarayıcı düşüşüyle çalışır. Motor kurulamadı
// diye kurulumu iptal etmek çok daha kötü olurdu.
procedure WebView2Kur;
var
  SonucKodu: Integer;
begin
  if WizardSilent then
    Exit; // sessiz oto-güncelleme: ilk kurulumda zaten halledildi, indirme yapma
  if WebView2Kurulu then
    Exit;
  IndirmePage.Clear;
  IndirmePage.Add(WV2Bootstrapper, 'MicrosoftEdgeWebview2Setup.exe', '');
  IndirmePage.Show;
  try
    try
      IndirmePage.Download;
      Exec(ExpandConstant('{tmp}\MicrosoftEdgeWebview2Setup.exe'), '/silent /install',
        '', SW_HIDE, ewWaitUntilTerminated, SonucKodu);
    except
      // indirilemedi/kurulamadı → sessizce devam (tarayıcı düşüşü çalışır)
    end;
  finally
    IndirmePage.Hide;
  end;
end;

function InitializeSetup: Boolean;
begin
  Result := EnAzWindows10;
  if not Result then
    // 2026-08-21: Artık ESKİ WINDOWS KURULUMU VAR — mesaj onu söylemek ZORUNDA.
    // Önceden yalnız "Windows'unu yükselt / başka bilgisayar kullan" diyordu;
    // panelde iki kart yan yana dursa da GitHub release'inden ya da eski bir
    // bağlantıdan modern kurulumu indiren Win7/8 kullanıcısı burada çıkmaza
    // düşüyordu. Doğru çözüm tek tık uzakta: eski kurulumun indirme adresi.
    MsgBox('Bu bilgisayarda Windows 10''dan eski bir sürüm var.' + #13#10 + #13#10 +
      'İNDİRDİĞİNİZ KURULUM bu Windows''ta ÇALIŞMAZ — ama sizin için AYRI bir' + #13#10 +
      'sürümümüz var:' + #13#10 + #13#10 +
      '  HizmetraYaziciKurulum-EskiWindows.exe' + #13#10 + #13#10 +
      'Windows 7 SP1, 8 ve 8.1 ile uyumludur. İndirmek için:' + #13#10 +
      '  • Hizmetra panelinde Yazıcılar > Bilgisayar Programı bölümünde' + #13#10 +
      '    "Windows 7 / 8 / 8.1" kartına tıklayın, ya da' + #13#10 +
      '  • Şu adresi tarayıcınıza yazın:' + #13#10 +
      '    https://github.com/kafe-panel/hizmetra-kopru/releases/latest' + #13#10 + #13#10 +
      'Yardım: destek@hizmetra.com', mbCriticalError, MB_OK);
end;

// AppId + "_is1" = Inno'nun kaldırma kayıt alt-anahtarı. PrivilegesRequired=lowest
// + localappdata → kurulum HKCU altındadır. (NOT: Pascal yorumunda { } kullanılmaz;
// GUID'in kapanış küme parantezi yorumu erken kapatırdı.)
const
  UninstAltAnahtar = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{D0752452-C0F4-4F3D-90F9-35A529466B16}_is1';

function MevcutKaldirmaKomutu(var Komut: string): Boolean;
begin
  Result := RegQueryStringValue(HKCU, UninstAltAnahtar, 'UninstallString', Komut)
    and (Komut <> '');
end;

// KapatCalisanUygulama — çalışan Hizmetra Yazıcı kopyalarını OTOMATİK kapatır ki
// kullanıcı elle uğraşmasın (AppMutex diyaloğunun yerini alan asıl mekanizma).
// Önce nazik (WM_CLOSE), tepsi uygulaması yanıt vermezse /F ile zorla; /T alt
// süreçleri de kapatır. taskkill.exe tüm Windows'larda {sys}'te vardır. Çıkışta
// dosya kilidi açılır → installer exe'nin üstüne yazabilir, [Run] yeniden açar.
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

procedure InitializeWizard;
var
  Komut, KuruluSurum: string;
begin
  { İndirme sayfası HER ZAMAN kurulur (WebView2 adımı kullanır); aşağıdaki
    erken Exit'lerin ÜSTÜNDE olmalı, yoksa PrepareToInstall nil sayfaya
    dokunur. }
  IndirmePage := CreateDownloadPage(SetupMessage(msgWizardPreparing),
    SetupMessage(msgPreparingDesc), nil);

  { Sessiz kurulumda (oto-güncelleme) sayfa gösterilmez; paneli hiç kurma. }
  if WizardSilent then
    Exit;
  if not MevcutKaldirmaKomutu(Komut) then
    Exit;
  RegQueryStringValue(HKCU, UninstAltAnahtar, 'DisplayVersion', KuruluSurum);

  MevcutPage := CreateInputOptionPage(wpWelcome,
    ExpandConstant('{cm:MevcutBaslik}'),
    ExpandConstant('{cm:MevcutAciklama}'),
    '', True, False);
  MevcutPage.Add(ExpandConstant('{cm:SecGuncelle}'));
  MevcutPage.Add(ExpandConstant('{cm:SecOnar}'));
  MevcutPage.Add(ExpandConstant('{cm:SecKaldir}'));

  { Kurulu sürüm == kurulacak sürüm ise varsayılan "Onar", değilse "Güncelle". }
  if KuruluSurum = '{#MyAppVersion}' then
    MevcutPage.SelectedValueIndex := 1
  else
    MevcutPage.SelectedValueIndex := 0;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
var
  Komut: string;
  SonucKodu: Integer;
begin
  Result := True;
  if (MevcutPage <> nil) and (CurPageID = MevcutPage.ID) and (MevcutPage.SelectedValueIndex = 2) then
  begin
    { Kaldır: çalışan uygulamayı kapat (kaldırıcı kilitli exe'yi silebilsin),
      kaldırıcıyı sessizce çalıştır, sonra kurulumu iptal et. }
    KapatCalisanUygulama;
    if MevcutKaldirmaKomutu(Komut) then
    begin
      Komut := RemoveQuotes(Komut);
      Exec(Komut, '/VERYSILENT', '', SW_SHOW, ewWaitUntilTerminated, SonucKodu);
    end;
    KaldirSecildi := True;
    WizardForm.Close;
    Result := False;
  end;
end;

// PrepareToInstall — kurulum dosya kopyalama AŞAMASINDAN hemen önce koşar.
// Güncelle/Onar seçildiyse çalışan uygulamayı BURADA kapat: kullanıcı sihirbazda
// gezerken uygulama fişleri basmaya devam etsin, yalnız son anda (kurulum başlarken)
// kapansın → en kısa kesinti. Fresh kurulumda (MevcutPage=nil) veya sessizde
// (panel yok) hiçbir şey yapmaz. Kaldır dalı buraya ULAŞMAZ (setup iptal edildi).
function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  if (MevcutPage <> nil) and (MevcutPage.SelectedValueIndex <> 2) then
    KapatCalisanUygulama;
  { Arayüz motoru (WebView2) eksikse burada kurulur — dosyalar kopyalanmadan
    önce, kullanıcı zaten bekliyorken. Hata olursa kurulum devam eder. }
  WebView2Kur;
end;

procedure CancelButtonClick(CurPageID: Integer; var Cancel, Confirm: Boolean);
begin
  { Kaldır dalında WizardForm.Close çağrısı iptal akışını tetikler; "Emin misiniz?"
    onayını gösterme (kullanıcı zaten Kaldır'ı bilerek seçti). }
  if KaldirSecildi then
    Confirm := False;
end;
