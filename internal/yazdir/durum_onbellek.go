package yazdir

import (
	"sync"
	"time"
)

// Yazıcı kuyruklarının PAYLAŞIMLI önbelleği.
//
// NEDEN: durum penceresi 3 saniyede bir özet topluyor ve her seferinde sistemin
// tüm yazıcılarını yeniden tarıyordu (Windows'ta EnumPrinters). Baskı öncesi
// kontrol de eklenince bu tarama iş başına bir kez daha çalışacaktı. Üç
// tüketici (baskı ön kontrolü, keşif/nabız, durum penceresi) artık AYNI
// önbellekten okur; 10 saniyelik tazelik fiş yazıcıları için fazlasıyla yeter.
//
// Tarama fonksiyonu bir DEĞİŞKEN arkasındadır: böylece önbellek davranışı
// (kaç gerçek tarama yapıldığı) macOS'ta sahte tarayıcıyla test edilebilir.

// YaziciDurumu — bir yazıcı kuyruğu hakkında bildiğimiz her şey.
type YaziciDurumu struct {
	Ad     string
	Port   string // Windows: "USB001" / "FILE:" / "IP_192.168.1.50"; CUPS: aygıt URI'si
	Surucu string
	// CevrimdisiIsaretli — Windows'ta "Yazıcıyı Çevrimdışı Kullan" işareti
	// (PRINTER_ATTRIBUTE_WORK_OFFLINE) veya CUPS'ta kuyruğun devre dışı olması.
	// Bu işaretliyken spooler işi KABUL EDER ama kağıda HİÇ dökmez.
	CevrimdisiIsaretli bool
	// Duraklatildi — kuyruk duraklatılmış (CUPS 'disabled' / Windows PAUSED).
	Duraklatildi bool
}

var (
	onbellekKilit  sync.Mutex
	onbellekVeri   map[string]YaziciDurumu
	onbellekZaman  time.Time
	onbellekTazeSn = 10 * time.Second
	// yaziciTarayici — gerçek sistem taraması (platform dosyalarında).
	yaziciTarayici = taraPlatform
	// taramaSayaci — YALNIZ test/teşhis: kaç GERÇEK tarama yapıldı.
	taramaSayaci int
)

// YazicilariOku — kuyruk adı → durum haritası (10 sn önbellekli).
// Harita KOPYA döner; çağıran üzerinde oynayabilir.
func YazicilariOku() (map[string]YaziciDurumu, error) {
	onbellekKilit.Lock()
	defer onbellekKilit.Unlock()
	if onbellekVeri != nil && time.Since(onbellekZaman) < onbellekTazeSn {
		return onbellekKopyala(onbellekVeri), nil
	}
	taramaSayaci++
	veri, err := yaziciTarayici()
	if err != nil {
		// Tarama başarısız: ESKİ önbellek varsa onu kullan (bir anlık hata
		// yüzünden "yazıcı yok" deyip HEDEF_YOK üretmeyelim).
		if onbellekVeri != nil {
			return onbellekKopyala(onbellekVeri), err
		}
		return nil, err
	}
	onbellekVeri = veri
	onbellekZaman = time.Now()
	return onbellekKopyala(veri), nil
}

// OnbellegiTemizle — bir sonraki okumada gerçek tarama yapılsın (onarımdan
// sonra durum değiştiği için çağrılır; testler de kullanır).
func OnbellegiTemizle() {
	onbellekKilit.Lock()
	onbellekVeri = nil
	onbellekZaman = time.Time{}
	onbellekKilit.Unlock()
}

func onbellekKopyala(m map[string]YaziciDurumu) map[string]YaziciDurumu {
	out := make(map[string]YaziciDurumu, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// CanliUsbPortlari — sistemde GERÇEKTEN takılı olan USB yazıcı portları
// (Windows'ta kayıt defterinden; diğer platformlarda BOŞ). Boş küme,
// teshis.UsbPortOlu'nun "kanıt yok → ASLA ölü deme" kuralını tetikler.
//
// İkinci dönüş tamListe: tarama SIRASINDA hiç hata olmadıysa true. false ise
// küme EKSİK olabilir; çağıran "port ölü" kuralını hiç işletmemelidir.
func CanliUsbPortlari() (map[string]string, bool) { return usbPortlariOkuPlatform() }
