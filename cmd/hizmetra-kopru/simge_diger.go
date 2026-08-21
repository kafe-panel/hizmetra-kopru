//go:build !windows

package main

import _ "embed"

// simgeVerisi — sistem tepsisi simgesi (Linux / macOS): PNG.
//
// NEDEN AYRI DOSYA (2026-08-21 denetimi): Eskiden TÜM platformlara simge.ico
// gömülüyordu. fyne systray'in unix arka ucu ikonu image.Decode ile çözer ve
// YALNIZ image/png kayıtlıdır (ICO çözücü yok) → "image: unknown format" ile
// sessizce boş ikon çizilirdi; Linux'ta tepsi simgesi HİÇ görünmüyordu.
// Linux'ta tepsi tek kalıcı arayüz olduğu için kullanıcı "kurdum ama ekranda
// hiçbir şey yok" diyordu. macOS'un NSImage'ı da PNG ile en güvenilir çalışır.
// simge.png = assets/hizmetra.png (256px) kopyasıdır.
//
//go:embed simge.png
var simgeVerisi []byte
