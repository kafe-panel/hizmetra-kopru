package main

// v0.4.0: bu dosya eskiden ("Kurulu kopya yönetimi", v0.3.0) exe'yi
// %LOCALAPPDATA%\HizmetraKopru\hizmetra-kopru.exe konumuna kopyalayan,
// Run anahtarını oraya yazan ve çalışan kopyaya devreden mantığı
// barındırıyordu. Bu mantık TAMAMEN KALDIRILDI: Inno Setup installer artık
// sabit bir konuma kurar, Denetim Masası'nda görünür ve kendi native
// "zaten kurulu → onar/kaldır" sihirbazını sağlar (bkz. plan Track C4) — Go
// tarafının "nerede kurulu olduğumu" bilmesine/yönetmesine GEREK KALMADI.
//
// bayrakVar genel amaçlı, tek bayrak kontrolü için (flag paketi kullanılmaz —
// systray/zenity argümanlarına karışmasın). Track C4'ün kaldırma akışı
// ("--kaldir-sunucu") bunu kullanır: installer'ın [UninstallRun] adımı,
// dosyalar silinmeden HEMEN ÖNCE exe'yi bu bayrakla çağırır.

import (
	"os"

	"github.com/kafe-panel/hizmetra-kopru/internal/api"
	"github.com/kafe-panel/hizmetra-kopru/internal/ayar"
	"github.com/kafe-panel/hizmetra-kopru/internal/gunluk"
)

// bayrakVar — komut satırında verilen bayrak var mı.
func bayrakVar(ad string) bool {
	for _, a := range os.Args[1:] {
		if a == ad {
			return true
		}
	}
	return false
}

// kaldirSunucudanCihazi — "--kaldir-sunucu" bayrağının davranışı (Track C4).
// Kayıtlı bir token varsa sunucudaki cihaz kaydını siler (POST
// /api/kopru/kaldir, Track B'nin ucu) — kullanıcı panelden elle silmek
// zorunda kalmaz. SESSİZ ve HIZLI: pencere/diyalog AÇMAZ; token yokluğu,
// bağlantı hatası, 401, 404 (uç henüz o sunucuda yayında değilse — ör.
// bugün production'da) dahil HER durumda sessizce döner. Çağıran main()
// bundan hemen sonra os.Exit(0) yapar; burada panic/uzun bekleme YOKTUR.
func kaldirSunucudanCihazi() {
	dizin, err := ayar.Dizin()
	if err != nil {
		return
	}
	gunluk.Baslat(dizin)
	defer gunluk.Kapat()

	a, err := ayar.Yukle()
	if err != nil || a.Token == "" {
		gunluk.Yaz("--kaldir-sunucu: kayıtlı token yok, sunucuya gidilmiyor")
		return
	}
	istemci := api.New(a.SunucuAdresi())
	istemci.Token = a.Token
	if err := istemci.Kaldir(); err != nil {
		gunluk.Yaz("--kaldir-sunucu: sunucu çağrısı yok sayıldı: %v", err)
		return
	}
	gunluk.Yaz("--kaldir-sunucu: cihaz kaydı sunucudan silindi")
}
