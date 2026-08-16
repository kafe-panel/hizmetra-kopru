package main

// v0.4.0: bu dosya eskiden ("Kurulu kopya yönetimi", v0.3.0) exe'yi
// %LOCALAPPDATA%\HizmetraKopru\hizmetra-kopru.exe konumuna kopyalayan,
// Run anahtarını oraya yazan ve çalışan kopyaya devreden mantığı
// barındırıyordu. Bu mantık TAMAMEN KALDIRILDI: Inno Setup installer artık
// sabit bir konuma kurar, Denetim Masası'nda görünür ve kendi native
// "zaten kurulu → onar/kaldır" sihirbazını sağlar (bkz. plan Track C4) — Go
// tarafının "nerede kurulu olduğumu" bilmesine/yönetmesine GEREK KALMADI.
//
// bayrakVar tek başına kalıyor: genel amaçlı, tek bayrak kontrolü için
// (flag paketi kullanılmaz — systray/zenity argümanlarına karışmasın).
// Track C4'ün kaldırma akışı ("--kaldir-sunucu") bunu kullanacak.

import "os"

// bayrakVar — komut satırında verilen bayrak var mı.
func bayrakVar(ad string) bool {
	for _, a := range os.Args[1:] {
		if a == ad {
			return true
		}
	}
	return false
}
