//go:build !windows && !darwin

package main

import (
	"os/exec"
	"strings"
)

// moderniDestekliyor — Linux'ta AYRI bir "eski" kanal YOKTUR: ikili
// CGO_ENABLED=0 ile tam statik derlenir, glibc sürümünden bağımsızdır ve
// eski/yeni dağıtımların hepsinde aynı paket çalışır. Bu yüzden daima modern
// kanal geçerlidir (bkz. platform_ek_windows.go — oradaki sürüm kapısının
// gerekçesi Go'nun Windows/macOS taban yükseltmeleridir, Linux'ta böyle bir
// taban kırılması yok).
func moderniDestekliyor() bool { return true }

// gizliKomut — Linux'ta gizlenecek konsol penceresi yok; imza ortak kalsın diye.
func gizliKomut(ad string, arg ...string) *exec.Cmd {
	return exec.Command(ad, arg...)
}

// panoOku — masaüstü Linux'ta pano aracı GARANTİ DEĞİLDİR (xclip/xsel çoğu
// kurulumda yok, Wayland'de wl-paste gerekir). Sırayla denenir; hiçbiri yoksa
// boş dönülür ve kullanıcı 6 haneli kodu elle yazar (zararsız düşüş).
func panoOku() string {
	adaylar := [][]string{
		{"wl-paste", "--no-newline"},
		{"xclip", "-selection", "clipboard", "-o"},
		{"xsel", "--clipboard", "--output"},
	}
	for _, a := range adaylar {
		out, err := exec.Command(a[0], a[1:]...).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

// uiTrayIcinde — Linux'ta arayüz sistem tarayıcısında açılır (native pencere
// WebKitGTK cgo bağımlılığı isterdi ve statik ikiliyi bozardı); tepsi döngüsüne
// bağımlılık yok.
const uiTrayIcinde = false
