package teshis

import "testing"

func TestPortSinifi(t *testing.T) {
	durumlar := []struct {
		port   string
		bekler string
	}{
		{"USB001", PortUSB},
		{"usb3", PortUSB},
		{"FILE:", PortDosya},
		{"file:", PortDosya},
		{"nul:", PortDosya},
		{"NUL:", PortDosya},
		{"PORTPROMPT:", PortDosya},
		{"portprompt:", PortDosya},
		{"XPSPort:1", PortDosya},
		{"SHRFAX:", PortDosya},
		{"IP_192.168.1.50", PortAg},
		{"192.168.1.50", PortAg},
		{"192.168.1.50:9100", PortAg},
		{"COM1:", PortDiger},
		{"LPT1:", PortDiger},
		{"", PortDiger},
	}
	for _, d := range durumlar {
		if got := PortSinifi(d.port); got != d.bekler {
			t.Errorf("PortSinifi(%q) = %q, beklenen %q", d.port, got, d.bekler)
		}
	}
}

// TestUsbPortOlu — canlı küme BOŞKEN asla "ölü" denmemeli (kapalı başarısızlık).
func TestUsbPortOlu(t *testing.T) {
	canli := map[string]string{"USB002": "USBPRINT\\ZJ-80", "USB003": "USBPRINT\\EPSON"}

	if olu, kanit := UsbPortOlu("USB001", canli); !olu || !kanit {
		t.Errorf("USB001 canlı kümede yok → ölü olmalı (olu=%v kanit=%v)", olu, kanit)
	}
	if olu, kanit := UsbPortOlu("usb002", canli); olu || !kanit {
		t.Errorf("USB002 canlı → ölü olmamalı (olu=%v kanit=%v)", olu, kanit)
	}
	if olu, kanit := UsbPortOlu("USB001", map[string]string{}); olu || kanit {
		t.Errorf("canlı küme boşken ASLA ölü denmemeli (olu=%v kanit=%v)", olu, kanit)
	}
	if olu, kanit := UsbPortOlu("USB001", nil); olu || kanit {
		t.Errorf("nil kümede ASLA ölü denmemeli (olu=%v kanit=%v)", olu, kanit)
	}
	if olu, kanit := UsbPortOlu("COM1:", canli); olu || kanit {
		t.Errorf("USB olmayan portta kural konuşmamalı (olu=%v kanit=%v)", olu, kanit)
	}
}

func TestHedefNormalize(t *testing.T) {
	if t_, ip, err := HedefNormalize("  192.168.1.50  "); err != nil || !ip || t_ != "192.168.1.50" {
		t.Errorf("portsuz IP tespit edilmeli: (%q,%v,%v)", t_, ip, err)
	}
	if t_, ip, err := HedefNormalize("192.168.1.50:9100"); err != nil || ip || t_ != "192.168.1.50:9100" {
		t.Errorf("portlu adres portsuz IP SAYILMAMALI: (%q,%v,%v)", t_, ip, err)
	}
	if _, _, err := HedefNormalize("POS-80\x00X"); err == nil {
		t.Error("NUL içeren hedef reddedilmeli (printers.Open panikliyor)")
	}
	if _, _, err := HedefNormalize("   "); err == nil {
		t.Error("boş hedef reddedilmeli")
	}
	if _, _, err := HedefNormalize(string(make([]rune, 300))); err == nil {
		t.Error("aşırı uzun hedef reddedilmeli")
	}
	if t_, _, err := HedefNormalize("Mutfak Yazıcı"); err != nil || t_ != "Mutfak Yazıcı" {
		t.Errorf("normal ad bozulmamalı: (%q,%v)", t_, err)
	}
}

// TestAgHedefiMi — iki nokta içeren YAZICI ADI ağ hedefi sayılmamalı.
func TestAgHedefiMi(t *testing.T) {
	durumlar := []struct {
		hedef  string
		bekler bool
	}{
		{"192.168.1.50:9100", true},
		{"printer.local:9100", true},
		{"10.0.0.5:515", true},
		{"Kasa:2", false}, // yazıcı adı — ağ adresi DEĞİL
		{"POS-80", false},
		{"", false},
		{"192.168.1.50:", false},
		{"192.168.1.50:abc", false},
		{"192.168.1.50:99999", false},
	}
	for _, d := range durumlar {
		if got := AgHedefiMi(d.hedef); got != d.bekler {
			t.Errorf("AgHedefiMi(%q) = %v, beklenen %v", d.hedef, got, d.bekler)
		}
	}
}

// TestAdEsitMi — panelde yazılan hedef ile Windows kuyruk adı arasındaki
// büyük/küçük harf ve boşluk farkı, "bu bilgisayarda böyle yazıcı yok"
// demeye sebep OLMAMALI.
func TestAdEsitMi(t *testing.T) {
	esit := [][2]string{
		{"POS-80", "pos-80"},
		{"POS-80", "POS-80 "},
		{" Kasa ", "kasa"},
		{"MUTFAK", "Mutfak"},
		{"ISIK", "ısık"}, // Türkçe I/ı tuzağı: aynı hedef sayılır
	}
	for _, c := range esit {
		if !AdEsitMi(c[0], c[1]) {
			t.Errorf("%q ile %q aynı yazıcı sayılmalıydı", c[0], c[1])
		}
	}
	farkli := [][2]string{
		{"POS-80", "POS-81"},
		{"Kasa", "Mutfak"},
		{"", "Kasa"},
	}
	for _, c := range farkli {
		if AdEsitMi(c[0], c[1]) {
			t.Errorf("%q ile %q farklı yazıcı olmalıydı", c[0], c[1])
		}
	}
}
