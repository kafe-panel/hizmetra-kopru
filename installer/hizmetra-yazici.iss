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
  #define MyAppVersion "0.7.0"
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
  KaldirSecildi: Boolean;

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
end;

procedure CancelButtonClick(CurPageID: Integer; var Cancel, Confirm: Boolean);
begin
  { Kaldır dalında WizardForm.Close çağrısı iptal akışını tetikler; "Emin misiniz?"
    onayını gösterme (kullanıcı zaten Kaldır'ı bilerek seçti). }
  if KaldirSecildi then
    Confirm := False;
end;
