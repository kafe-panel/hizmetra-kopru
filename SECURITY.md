# Güvenlik Politikası / Security Policy

## Güvenlik açığı bildirimi

Bir güvenlik açığı bulduysanız lütfen **herkese açık issue açmayın**.
Bunun yerine doğrudan e-posta gönderin: **guvenlik@hizmetra.com**

Bildiriminize şunları eklerseniz çok yardımcı olur:
- Etkilenen sürüm (`hizmetra-kopru.exe` sürümü — tepsi menüsünde görünür)
- Yeniden üretme adımları
- Etkisi (ne yapılabiliyor)

İlk yanıtı **3 iş günü** içinde vermeyi hedefliyoruz. Doğrulanan açıklar için
düzeltme yayınlandıktan sonra, isterseniz teşekkür bölümünde adınıza yer veririz.

## Güvenlik modeli (bilinmesi gerekenler)

- Program yalnız **giden** HTTPS bağlantısı kurar; dinleyen bir port **açmaz**.
- Cihaz anahtarı (`%APPDATA%\HizmetraKopru\config.json`) yalnız **baskı işi çekme ve
  sonuç bildirme** yetkisi verir. Satış, müşteri veya ödeme verisine erişemez.
- Anahtar dosya izni `0600`'dür (yalnız o Windows kullanıcısı). Panelden cihaz
  "Kaldır" edildiğinde anahtar **anında** geçersiz olur.
- Fiş içeriği günlük dosyasına **yazılmaz** (KVKK: fişte müşteri adı/adres/telefon olabilir).

## Desteklenen sürümler

En son yayınlanan sürüm desteklenir. Lütfen güncel sürümü kullanın.

---

## English

Please report security issues privately to **guvenlik@hizmetra.com** rather than
opening a public issue. We aim to respond within 3 business days.
