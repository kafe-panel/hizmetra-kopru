package main

import _ "embed"

// simgeVerisi — sistem tepsisi simgesi. Windows'ta .ico şart (systray
// platforma göre ICO/PNG bekler); tek binary için gömülü tutuluyor.
//
//go:embed simge.ico
var simgeVerisi []byte
