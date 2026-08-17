//go:build !darwin

// Bu paketin somut mantığı YALNIZ darwin'dedir (guncelleci_darwin.go: DmgKur).
// Diğer platformlarda paket BOŞTUR ama var OLMALI: aksi halde `go build ./...` /
// `go test ./...` (linux/windows) "build constraints exclude all Go files"
// hatası verir — CI goreleaser job'ı ubuntu'da `go test ./...` koşar, bu da
// release'i öldürürdü. Windows installer + Linux .deb güncelleme mantığı main
// paketinde (guncelle_windows.go / guncelle_linux.go), systray.Quit ile iç içe
// olduğundan orada kalır; darwin'in dmg-kurulumu ise cgo'suz olduğu için buraya
// alınıp Windows'ta bile GOOS=darwin ile typecheck edilebilir kılındı.
package guncelleci
