package usb

import (
	"reflect"
	"strings"
	"testing"
)

// piWithStick ist ein Raspberry Pi mit Speicherkarte als System und einem
// eingesteckten Stick, so wie ein aktuelles lsblk ihn meldet: echte
// Wahrheitswerte, Zahlen und "mountpoints" als Feld mit null-Einträgen.
const piWithStick = `{
  "blockdevices": [
    {"name":"sda","path":"/dev/sda","rm":true,"hotplug":true,"tran":"usb","type":"disk","fstype":null,"label":null,"size":15931539456,"mountpoints":[null],"model":"Cruzer Blade","vendor":"SanDisk ",
     "children":[
       {"name":"sda1","path":"/dev/sda1","rm":true,"hotplug":true,"tran":null,"type":"part","fstype":"vfat","label":"PARTY","size":15929442304,"mountpoints":[null],"model":null,"vendor":null}
     ]},
    {"name":"mmcblk0","path":"/dev/mmcblk0","rm":false,"hotplug":false,"tran":null,"type":"disk","fstype":null,"label":null,"size":31914983424,"mountpoints":[null],"model":null,"vendor":null,
     "children":[
       {"name":"mmcblk0p1","path":"/dev/mmcblk0p1","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"vfat","label":"bootfs","size":536870912,"mountpoints":["/boot/firmware"],"model":null,"vendor":null},
       {"name":"mmcblk0p2","path":"/dev/mmcblk0p2","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"ext4","label":"rootfs","size":31373893632,"mountpoints":["/"],"model":null,"vendor":null}
     ]},
    {"name":"zram0","path":"/dev/zram0","rm":false,"hotplug":false,"tran":null,"type":"disk","fstype":"swap","label":null,"size":2147483648,"mountpoints":["[SWAP]"],"model":null,"vendor":null}
  ]
}`

// oldLsblkWithoutPartitionTable ist ein älteres lsblk: alles in
// Anführungszeichen, "1"/"0" statt true/false, keine Spalte PATH, die
// Einzahl "mountpoint". Der Stick trägt sein Dateisystem ohne
// Partitionstabelle direkt auf dem Gerät.
const oldLsblkWithoutPartitionTable = `{
  "blockdevices": [
    {"name":"mmcblk0","rm":"0","hotplug":"0","tran":null,"type":"disk","fstype":null,"label":null,"size":"31914983424","mountpoint":null,"model":null,"vendor":null,
     "children":[
       {"name":"mmcblk0p1","rm":"0","hotplug":"0","tran":null,"type":"part","fstype":"vfat","label":"boot","size":"268435456","mountpoint":"/boot","model":null,"vendor":null},
       {"name":"mmcblk0p2","rm":"0","hotplug":"0","tran":null,"type":"part","fstype":"ext4","label":"rootfs","size":"31646547968","mountpoint":"/","model":null,"vendor":null}
     ]},
    {"name":"sdb","rm":"1","hotplug":"1","tran":"usb","type":"disk","fstype":"exfat","label":"FOTOS","size":"8004304896","mountpoint":"/media/eventprint/FOTOS","model":"USB Flash Disk","vendor":"Generic "}
  ]
}`

// usbSSD ist eine ext4-SSD am USB, die sich weder als wechselbar noch als
// Hotplug meldet. Allein der Transportweg verrät sie. Die Partition ohne
// Dateisystem und die Auslagerung dürfen nicht erscheinen.
const usbSSD = `{
  "blockdevices": [
    {"name":"sda","path":"/dev/sda","rm":false,"hotplug":false,"tran":"usb","type":"disk","fstype":null,"label":null,"size":500107862016,"mountpoint":null,"model":"Extreme SSD","vendor":"SanDisk",
     "children":[
       {"name":"sda1","path":"/dev/sda1","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"ext4","label":null,"size":480000000000,"mountpoint":"/media/eventprint/3f1c","model":null,"vendor":null},
       {"name":"sda2","path":"/dev/sda2","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":null,"label":null,"size":1048576,"mountpoint":null,"model":null,"vendor":null},
       {"name":"sda3","path":"/dev/sda3","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"swap","label":null,"size":20000000000,"mountpoint":null,"model":null,"vendor":null}
     ]}
  ]
}`

// bootsFromUSB ist ein Pi 5, der von einer USB-SSD startet, mit einem
// Kartenleser am USB und einer internen NVMe. Die System-SSD darf trotz
// TRAN=usb nie erscheinen, auch nicht mit ihrer unbenutzten Partition.
const bootsFromUSB = `{
  "blockdevices": [
    {"name":"sda","path":"/dev/sda","rm":false,"hotplug":false,"tran":"usb","type":"disk","fstype":null,"label":null,"size":256060514304,"mountpoints":[null],"model":"T7","vendor":"Samsung",
     "children":[
       {"name":"sda1","path":"/dev/sda1","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"vfat","label":"bootfs","size":536870912,"mountpoints":["/boot/firmware"],"model":null,"vendor":null},
       {"name":"sda2","path":"/dev/sda2","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"ext4","label":"rootfs","size":200000000000,"mountpoints":["/"],"model":null,"vendor":null},
       {"name":"sda3","path":"/dev/sda3","rm":false,"hotplug":false,"tran":null,"type":"part","fstype":"exfat","label":"DATA","size":50000000000,"mountpoints":[null],"model":null,"vendor":null}
     ]},
    {"name":"sdb","path":"/dev/sdb","rm":true,"hotplug":true,"tran":"usb","type":"disk","fstype":null,"label":null,"size":64021856256,"mountpoints":[null],"model":"Card Reader","vendor":"Generic",
     "children":[
       {"name":"sdb1","path":"/dev/sdb1","rm":true,"hotplug":true,"tran":null,"type":"part","fstype":"ntfs","label":"Urlaub","size":64020807680,"mountpoints":[null],"model":null,"vendor":null}
     ]},
    {"name":"nvme0n1","path":"/dev/nvme0n1","rm":false,"hotplug":false,"tran":"nvme","type":"disk","fstype":null,"label":null,"size":512110190592,"mountpoints":[null],"model":"WD Blue","vendor":null,
     "children":[
       {"name":"nvme0n1p1","path":"/dev/nvme0n1p1","rm":false,"hotplug":false,"tran":"nvme","type":"part","fstype":"ext4","label":"daten","size":512109142016,"mountpoints":[null],"model":null,"vendor":null}
     ]}
  ]
}`

func TestParseLsblk(t *testing.T) {
	tests := []struct {
		name string
		json string
		want []Drive
	}{
		{
			name: "Pi mit Speicherkarte und Stick",
			json: piWithStick,
			want: []Drive{{
				Path: "/dev/sda1", Disk: "/dev/sda", Label: "PARTY", Vendor: "SanDisk", Model: "Cruzer Blade",
				Size: 15929442304, FSType: "vfat",
			}},
		},
		{
			name: "altes lsblk, Stick ohne Partitionstabelle",
			json: oldLsblkWithoutPartitionTable,
			want: []Drive{{
				Path: "/dev/sdb", Disk: "/dev/sdb", Label: "FOTOS", Vendor: "Generic", Model: "USB Flash Disk",
				Size: 8004304896, FSType: "exfat", MountPoint: "/media/eventprint/FOTOS",
			}},
		},
		{
			name: "ext4-SSD am USB",
			json: usbSSD,
			want: []Drive{{
				Path: "/dev/sda1", Disk: "/dev/sda", Vendor: "SanDisk", Model: "Extreme SSD",
				Size: 480000000000, FSType: "ext4", MountPoint: "/media/eventprint/3f1c",
			}},
		},
		{
			name: "System auf USB-SSD, Kartenleser, NVMe",
			json: bootsFromUSB,
			want: []Drive{{
				Path: "/dev/sdb1", Disk: "/dev/sdb", Label: "Urlaub", Vendor: "Generic", Model: "Card Reader",
				Size: 64020807680, FSType: "ntfs",
			}},
		},
		{
			name: "nichts eingesteckt",
			json: `{"blockdevices":[]}`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLsblk([]byte(tt.json))
			if err != nil {
				t.Fatalf("err = %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("\n got  %+v\n want %+v", got, tt.want)
			}
		})
	}
}

func TestParseLsblkRejectsGarbage(t *testing.T) {
	for _, in := range []string{`kaputt`, `{"blockdevices":[{"name":"sda","rm":"vielleicht"}]}`} {
		if _, err := parseLsblk([]byte(in)); err == nil {
			t.Errorf("%q wurde angenommen", in)
		}
	}
}

func TestParseMountOutput(t *testing.T) {
	tests := []struct {
		name     string
		out      string
		existing []string
		want     string
	}{
		{"älteres udisks mit Punkt", "Mounted /dev/sda1 at /media/eventprint/STICK.\n", nil, "/media/eventprint/STICK"},
		{"neueres udisks ohne Punkt", "Mounted /dev/sda1 at /media/eventprint/STICK\n", nil, "/media/eventprint/STICK"},
		{"Leerzeichen und at im Namen", "Mounted /dev/sda1 at /media/eventprint/Party at Home.\n", nil, "/media/eventprint/Party at Home"},
		{"Punkt gehört zum Namen", "Mounted /dev/sda1 at /media/eventprint/FOTOS.\n", []string{"/media/eventprint/FOTOS."}, "/media/eventprint/FOTOS."},
		{"Punkt gehört nicht zum Namen", "Mounted /dev/sda1 at /media/eventprint/FOTOS.\n", []string{"/media/eventprint/FOTOS", "/media/eventprint/FOTOS."}, "/media/eventprint/FOTOS"},
		{"Warnung davor", "(udisksctl:1234): GLib-WARNING **: something\nMounted /dev/sdb at /media/eventprint/X.\n", nil, "/media/eventprint/X"},
		{"unbekannte Meldung", "Error mounting /dev/sda1: GDBus.Error:org.freedesktop.UDisks2.Error.AlreadyMounted: Device /dev/sda1 is already mounted at `/media/eventprint/STICK'.\n", nil, ""},
		{"leer", "", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := func(p string) bool {
				for _, e := range tt.existing {
					if e == p {
						return true
					}
				}

				return false
			}

			if got := parseMountOutput(tt.out, exists); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"IMG_0001.jpg", "IMG_0001.jpg"},
		{"12:30 Torte.jpg", "12-30 Torte.jpg"},
		{"a/b\\c.jpg", "a-b-c.jpg"},
		{"was?.jpg", "was_.jpg"},
		{"CON.jpg", "_CON.jpg"},
		{"  .versteckt.jpg ", "versteckt.jpg"},
		{strings.Repeat("ä", 200) + ".jpeg", strings.Repeat("ä", 115) + ".jpeg"},
	}

	for _, tt := range tests {
		if got := sanitizeFileName(tt.in); got != tt.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{512, "512 B"},
		{1500, "1,5 kB"},
		{15931539456, "15,9 GB"},
		{500107862016, "500 GB"},
		{2_000_000_000_000, "2,0 TB"},
	}

	for _, tt := range tests {
		if got := FormatSize(tt.in); got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDriveTitle(t *testing.T) {
	tests := []struct {
		d    Drive
		want string
	}{
		{Drive{Label: "PARTY", Vendor: "SanDisk", Model: "Cruzer"}, "PARTY"},
		{Drive{Vendor: "SanDisk", Model: "Cruzer"}, "SanDisk Cruzer"},
		{Drive{Model: "Cruzer"}, "Cruzer"},
		{Drive{}, "USB-Stick"},
	}

	for _, tt := range tests {
		if got := tt.d.Title(); got != tt.want {
			t.Errorf("Title() = %q, want %q", got, tt.want)
		}
	}
}
