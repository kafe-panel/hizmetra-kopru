//go:build !darwin

package main

// macOS dışı platformlarda otomatik başlatma UYGULAMA TARAFINDAN kurulmaz:
//   - Windows: Inno Setup installer'ın [Registry] HKCU\Run girdisi yazar
//     (bkz. installer/hizmetra-yazici.iss, platform_windows.go notu).
//   - Linux: .deb'in kurduğu /etc/xdg/autostart/hizmetra-kopru.desktop yapar
//     (bkz. .goreleaser.yaml nfpms + packaging/linux/hizmetra-kopru.desktop).
//
// Bu yüzden bu platformlarda otomatikBaslatKur bilinçli bir NO-OP'tur; main.go
// çağrıyı koşulsuz yapabilsin diye (platform_diger.go stub deseniyle aynı) yalnız
// darwin'de gerçek iş yapan bir eşleniği vardır (autostart_darwin.go).
func otomatikBaslatKur() {}
