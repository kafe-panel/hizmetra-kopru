"""Hizmetra ikonundan tepsi/exe ICO'su: alpha-bbox KIRP + %10 pad + zemin (§KARAR).

Neden: kaynak ikon 4000x4000 tuvalde içerik yalnız ~%50 alan kaplıyordu; 16px'te
~8px kalıp şeffaf zeminle koyu tepside kayboluyordu (emre 2026-08-16, madde 4).
Çözüm: alfa sınır kutusuna kırp, %10 kenar payı bırak, beyaz yuvarlatılmış kare
zemine oturt (§KARAR: 2). Üretilen ICO 7 boyut taşır (16..256).

Kullanım (repo kökünden ya da herhangi bir yerden; çıktılar repo köküne yazılır):
    python assets/ikon_uret.py            # üret + 16px doluluk doğrulaması
Kaynak PNG repoya GİRMEZ (4000px, 1 MB); üretilen ICO'lar girer.
"""

from __future__ import annotations

import os
import sys

from PIL import Image, ImageDraw

SRC = r"C:\Users\ASUS\Desktop\hizmetra-logo\hizmetra-ikon-mavi kopyası.png"
ZEMIN = "yuvarlak"  # §KARAR: 'seffaf' | 'yuvarlak' | 'daire'
S = 256  # ana tuval; ICO katmanları bundan LANCZOS ile küçültülür
PAD_ORAN = 0.10  # içerik ile zemin kenarı arası pay
KOSE_ORAN = 0.22  # yuvarlatılmış karenin köşe yarıçapı (S'nin oranı, iOS ~%22)
BOYUTLAR = [(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)]
MIN_16PX_DOLULUK = 0.70  # 16x16 katmanda alpha>0 piksel oranı bunun ALTINA düşerse hata

KOK = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CIKTILAR = [
    os.path.join(KOK, "assets", "hizmetra.ico"),  # exe ikonu (winres RT_GROUP_ICON)
    os.path.join(KOK, "cmd", "hizmetra-kopru", "simge.ico"),  # tepsi ikonu (go:embed)
]


def uret(kaynak: str = SRC, zemin: str = ZEMIN) -> Image.Image:
    im = Image.open(kaynak).convert("RGBA")
    bbox = im.split()[3].getbbox()
    if bbox is None:
        raise SystemExit("kaynak tamamen şeffaf — kırpılacak içerik yok")
    im = im.crop(bbox)

    pad = int(S * PAD_ORAN)
    c = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(c)
    if zemin == "yuvarlak":
        d.rounded_rectangle([0, 0, S - 1, S - 1], radius=int(S * KOSE_ORAN), fill=(255, 255, 255, 255))
    elif zemin == "daire":
        d.ellipse([0, 0, S - 1, S - 1], fill=(255, 255, 255, 255))
    elif zemin != "seffaf":
        raise SystemExit(f"bilinmeyen ZEMIN: {zemin!r}")

    inner = S - 2 * pad
    w, h = im.size
    sc = min(inner / w, inner / h)
    r = im.resize((max(1, int(w * sc)), max(1, int(h * sc))), Image.LANCZOS)
    c.alpha_composite(r, ((S - r.size[0]) // 2, (S - r.size[1]) // 2))
    return c


def doluluk_16px(ico_yolu: str) -> float:
    """ICO'nun 16x16 katmanında alpha>0 piksel oranı (0..1)."""
    ico = Image.open(ico_yolu)
    ico.size = (16, 16)
    ico.load()
    a = ico.convert("RGBA").split()[3]
    return sum(1 for v in a.getdata() if v > 0) / (16 * 16)


def main() -> int:
    c = uret()
    for out in CIKTILAR:
        c.save(out, format="ICO", sizes=BOYUTLAR)
    hata = 0
    for out in CIKTILAR:
        ico = Image.open(out)
        boyutlar = ico.info.get("sizes")
        oran = doluluk_16px(out)
        durum = "ok" if (boyutlar == set(BOYUTLAR) and oran >= MIN_16PX_DOLULUK) else "HATA"
        if durum == "HATA":
            hata = 1
        print(f"{durum} {os.path.relpath(out, KOK)}: {len(boyutlar or [])} boyut, 16px doluluk {oran:.0%}")
    return hata


if __name__ == "__main__":
    sys.exit(main())
