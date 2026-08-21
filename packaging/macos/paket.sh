#!/usr/bin/env bash
# Hizmetra Yazıcı — macOS .app bundle + .dmg paketleyici.
#
# release.yml'deki "macos" job'ında (macos-latest) çalışır; lipo/sips/iconutil/
# hdiutil YALNIZ macOS'ta vardır — bu script Windows/Linux'ta ÇALIŞMAZ, orada
# yalnız YAML/mantık incelemesiyle doğrulanır, runtime CI'da (Mac runner) koşar.
#
# Girdi: aynı dizinde daha önce derlenmiş darwin ikilileri
#   hizmetra-kopru_darwin_amd64  ve  hizmetra-kopru_darwin_arm64
# (release.yml'in "Darwin ikililerini derle" adımı üretir).
# Çıktı: dist-macos/$DMG_ADI  (imzasız — bkz. packaging/macos/BENI-OKU.txt)
#
# Kullanım:  bash packaging/macos/paket.sh <surum> [amd64_ikili] [arm64_ikili]
#
# ORTAM DEĞİŞKENLERİ (2026-08-21 — iki kanal):
#   MIN_MACOS : Info.plist LSMinimumSystemVersion (varsayılan 10.15)
#   DMG_ADI   : çıktı .dmg adı (varsayılan HizmetraYazici.dmg)
# MODERN:  MIN_MACOS=10.15  DMG_ADI=HizmetraYazici.dmg          (amd64+arm64)
# ESKİ  :  MIN_MACOS=10.13  DMG_ADI=HizmetraYazici-EskiMac.dmg  (YALNIZ amd64)
# arm64 ikilisi verilmezse paket Intel-only üretilir — ESKİ kanal için DOĞRUDUR:
# macOS 10.13/10.14 yalnız Intel Mac'lerde vardır, Apple Silicon tabanı Big Sur 11.
set -euo pipefail
# Türkçe dosya adları ("Hizmetra Yazıcı.app") için UTF-8 yerel ayar şart:
# GitHub macOS runner'ında LANG bazen ayarsız (C locale) gelir ve non-ASCII
# dosya adlarını bozar. en_US.UTF-8 macOS'ta her zaman kuruludur.
export LC_ALL=en_US.UTF-8 LANG=en_US.UTF-8

VER="${1:?kullanım: paket.sh <surum> [amd64_ikili] [arm64_ikili]}"
AMD64_BIN="${2:-hizmetra-kopru_darwin_amd64}"
ARM64_BIN="${3:-hizmetra-kopru_darwin_arm64}"
MIN_MACOS="${MIN_MACOS:-10.15}"
DMG_ADI="${DMG_ADI:-HizmetraYazici.dmg}"
# IMZA_KIMLIGI boşsa AD-HOC imza kullanılır (Gatekeeper uyarısı KALKMAZ, yalnız
# "uygulama zarar görmüş" hatası engellenir). Apple Developer Program üyeliği
# alınıp CI'ya sertifika secret'ı eklenince buraya "Developer ID Application:
# ... (TEAMID)" gelir → gerçek imza + hardened runtime + güvenli zaman damgası,
# ki notarization ancak böyle kabul edilir.
IMZA_KIMLIGI="${IMZA_KIMLIGI:-}"

# Repo kökü (bu script packaging/macos/ altında).
KOK="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CIKTI="dist-macos"
BUNDLE="Hizmetra Yazıcı.app"
CONTENTS="$CIKTI/$BUNDLE/Contents"

[ -f "$AMD64_BIN" ] || { echo "HATA: darwin amd64 ikilisi bulunamadı: $AMD64_BIN" >&2; exit 1; }
# arm64 OPSİYONEL (eski kanal Intel-only üretir).
ARM64_VAR=0
[ -f "$ARM64_BIN" ] && ARM64_VAR=1

rm -rf "$CIKTI"
mkdir -p "$CONTENTS/MacOS" "$CONTENTS/Resources"

# 1) Evrensel (universal2) ikili: TEK .app hem Intel (amd64) hem Apple Silicon
#    (arm64) Mac'lerde çalışsın — kafe sahibi mimari seçmek zorunda kalmasın.
echo "==> lipo: evrensel ikili"
if [ "$ARM64_VAR" = "1" ]; then
  echo "    evrensel (amd64 + arm64)"
  lipo -create "$AMD64_BIN" "$ARM64_BIN" -output "$CONTENTS/MacOS/hizmetra-kopru"
else
  echo "    tek mimari: Intel (amd64) — eski macOS kanalı"
  cp "$AMD64_BIN" "$CONTENTS/MacOS/hizmetra-kopru"
fi
chmod +x "$CONTENTS/MacOS/hizmetra-kopru"

# AD-HOC KOD İMZASI — lipo'DAN SONRA yapılmak ZORUNDA (2026-08-21).
# Apple Silicon'da çekirdek imzasız arm64 kodu ÇALIŞTIRMAZ (SIGKILL); Finder bunu
# "uygulama zarar görmüş, Çöp'e taşıyın" diye gösterir ve kullanıcı uygulamayı
# siler. Go bağlayıcısı darwin/arm64'ü derlerken otomatik ad-hoc imzalar, AMA
# `lipo -create` ikiliyi yeniden yazdığı için o mühür geçersizleşir.
# `--deep` KULLANILMAZ (Apple DTS önermiyor): önce çalıştırılabilir, sonra bundle.
# Bu, Gatekeeper uyarısını KALDIRMAZ (onun için Developer ID + notarization
# gerekir) ama "damaged" hatasını engeller ve "Yine de Aç" yolunu AÇAR.
if [ -n "$IMZA_KIMLIGI" ]; then
  # GERÇEK imza: notarization için hardened runtime (--options runtime) ve
  # güvenli zaman damgası (--timestamp) ZORUNLUDUR; ikisi olmadan Apple
  # notarization'ı reddeder.
  echo "==> Developer ID imzası (hardened runtime)"
  codesign --force --options runtime --timestamp     --sign "$IMZA_KIMLIGI" "$CONTENTS/MacOS/hizmetra-kopru"
else
  echo "==> ad-hoc kod imzası (imzasız dağıtım)"
  codesign --force --sign - "$CONTENTS/MacOS/hizmetra-kopru"
fi

# 2) Info.plist — sürüm şablondan doldurulur.
echo "==> Info.plist ($VER)"
sed -e "s/__VERSION__/${VER}/g" -e "s/__MINOS__/${MIN_MACOS}/g"   "$KOK/packaging/macos/Info.plist.template" > "$CONTENTS/Info.plist"

# 3) icon.icns — assets/hizmetra.png'den (256px) iconset üret + iconutil.
#    Kaynak 256px olduğu için 512/1024 (@2x üstü) katmanlar ATLANIR; iconutil
#    kısmi iconset'i kabul eder. Daha keskin Retina için kaynak vektör/1024px PNG
#    gerekir (bkz. assets/ikon_uret.py — orijinal 4000px logo repoda değil).
echo "==> icon.icns"
ICONSET="$CIKTI/icon.iconset"
rm -rf "$ICONSET"; mkdir -p "$ICONSET"
SRC_PNG="$KOK/assets/hizmetra.png"
[ -f "$SRC_PNG" ] || { echo "HATA: kaynak PNG yok: $SRC_PNG" >&2; exit 1; }
sips -z 16 16   "$SRC_PNG" --out "$ICONSET/icon_16x16.png"      >/dev/null
sips -z 32 32   "$SRC_PNG" --out "$ICONSET/icon_16x16@2x.png"   >/dev/null
sips -z 32 32   "$SRC_PNG" --out "$ICONSET/icon_32x32.png"      >/dev/null
sips -z 64 64   "$SRC_PNG" --out "$ICONSET/icon_32x32@2x.png"   >/dev/null
sips -z 128 128 "$SRC_PNG" --out "$ICONSET/icon_128x128.png"    >/dev/null
sips -z 256 256 "$SRC_PNG" --out "$ICONSET/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "$SRC_PNG" --out "$ICONSET/icon_256x256.png"    >/dev/null
iconutil -c icns "$ICONSET" -o "$CONTENTS/Resources/icon.icns"

# 4) PkgInfo — Finder için standart (isteğe bağlı ama gelenek).
printf 'APPL????' > "$CONTENTS/PkgInfo"

# 4b) BUNDLE AD-HOC İMZASI — tüm içerik (Info.plist, icon.icns, PkgInfo)
# yerleştikten SONRA. Bundle mührü, .app'in aktarım sırasında bozulmadığını
# doğrular; imzasız bundle Apple Silicon'da "damaged" hatasına yol açar.
if [ -n "$IMZA_KIMLIGI" ]; then
  echo "==> bundle Developer ID imzası"
  codesign --force --options runtime --timestamp     --sign "$IMZA_KIMLIGI" "$CIKTI/$BUNDLE"
else
  echo "==> bundle ad-hoc imzası"
  codesign --force --sign - "$CIKTI/$BUNDLE"
fi

# 5) .dmg — "Applications'a sürükle" arayüzü: .app + Applications symlink + not.
echo "==> .dmg"
DMGROOT="$CIKTI/dmgroot"
rm -rf "$DMGROOT"; mkdir -p "$DMGROOT"
cp -R "$CIKTI/$BUNDLE" "$DMGROOT/"
ln -s /Applications "$DMGROOT/Applications"
cp "$KOK/packaging/macos/BENI-OKU.txt" "$DMGROOT/BENI-OKU.txt"
hdiutil create -volname "Hizmetra Yazıcı" -srcfolder "$DMGROOT" \
  -ov -format UDZO "$CIKTI/$DMG_ADI"

# Doğrulama — varsayıma güvenme (CI'da sessiz bozulmayı yakalar).
echo "==> doğrulama"
lipo -info "$CONTENTS/MacOS/hizmetra-kopru" || true
codesign -dv "$CIKTI/$BUNDLE" 2>&1 | head -5 || true

echo "==> TAMAM: $CIKTI/$DMG_ADI (minimum macOS $MIN_MACOS)"
