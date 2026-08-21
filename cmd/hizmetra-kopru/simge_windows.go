//go:build windows

package main

import _ "embed"

// simgeVerisi — sistem tepsisi simgesi (Windows). Win32 tepsi API'si ICO ister;
// systray'in Windows arka ucu veriyi doğrudan HICON'a çevirir.
//
//go:embed simge.ico
var simgeVerisi []byte
