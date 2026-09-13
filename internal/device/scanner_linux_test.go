//go:build linux

package device

import (
	"reflect"
	"testing"
)

func TestParseLSBLKAggregatesNestedMetadata(t *testing.T) {
	input := []byte(`{
  "blockdevices": [
    {
      "name": "/dev/nvme0n1",
      "kname": "nvme0n1",
      "path": "/dev/nvme0n1",
      "type": "disk",
      "tran": "nvme",
      "rm": false,
      "ro": false,
      "size": 1000204886016,
      "model": "System SSD",
      "vendor": "",
      "serial": "SYS123",
      "mountpoints": [null],
      "fstype": null,
      "children": [
        {
          "name": "/dev/nvme0n1p3",
          "kname": "nvme0n1p3",
          "path": "/dev/nvme0n1p3",
          "type": "part",
          "rm": false,
          "ro": false,
          "size": 999000000000,
          "mountpoints": [null],
          "fstype": "crypto_LUKS",
          "children": [
            {
              "name": "/dev/mapper/cryptroot",
              "kname": "dm-0",
              "path": "/dev/mapper/cryptroot",
              "type": "crypt",
              "rm": false,
              "ro": false,
              "size": 998000000000,
              "mountpoints": [null],
              "fstype": "LVM2_member",
              "children": [
                {
                  "name": "/dev/mapper/vg-root",
                  "kname": "dm-1",
                  "path": "/dev/mapper/vg-root",
                  "type": "lvm",
                  "rm": false,
                  "ro": false,
                  "size": 900000000000,
                  "mountpoints": ["/"],
                  "fstype": "ext4"
                },
                {
                  "name": "/dev/mapper/vg-swap",
                  "kname": "dm-2",
                  "path": "/dev/mapper/vg-swap",
                  "type": "lvm",
                  "rm": false,
                  "ro": false,
                  "size": 16000000000,
                  "mountpoints": [null],
                  "fstype": "swap"
                }
              ]
            }
          ]
        }
      ]
    },
    {
      "name": "/dev/sdb",
      "kname": "sdb",
      "path": "/dev/sdb",
      "type": "disk",
      "tran": "usb",
      "rm": 0,
      "ro": "0",
      "size": "64000000000",
      "model": "Flash Drive",
      "vendor": "Example",
      "serial": "USB123",
      "mountpoints": [null],
      "fstype": null,
      "children": [
        {
          "name": "/dev/sdb1",
          "kname": "sdb1",
          "path": "/dev/sdb1",
          "type": "part",
          "rm": 0,
          "ro": 0,
          "size": "63990000000",
          "mountpoints": ["/media/user/USB"],
          "fstype": "exfat"
        }
      ]
    }
  ]
}`)

	devices, err := parseLSBLK(input)
	if err != nil {
		t.Fatalf("parseLSBLK() error = %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("len(devices) = %d, want 2", len(devices))
	}

	system := devices[0]
	if system.Path != "/dev/nvme0n1" || !system.SystemDisk || !system.ContainsSwap {
		t.Fatalf("system device = %+v", system)
	}
	if want := []string{"/"}; !reflect.DeepEqual(system.MountPoints, want) {
		t.Fatalf("system mount points = %#v, want %#v", system.MountPoints, want)
	}
	if want := []string{"LVM2_member", "crypto_LUKS", "ext4", "swap"}; !reflect.DeepEqual(system.FileSystems, want) {
		t.Fatalf("system filesystems = %#v, want %#v", system.FileSystems, want)
	}

	usb := devices[1]
	if usb.Path != "/dev/sdb" || usb.Removable || usb.Transport != "usb" || !usb.LikelyExternal {
		t.Fatalf("usb device = %+v", usb)
	}
	if usb.SystemDisk || usb.ContainsSwap {
		t.Fatalf("USB unexpectedly classified unsafe: %+v", usb)
	}
	if want := []string{"/media/user/USB"}; !reflect.DeepEqual(usb.MountPoints, want) {
		t.Fatalf("mount points = %#v, want %#v", usb.MountPoints, want)
	}
}

func TestParseLSBLKRejectsMalformedJSON(t *testing.T) {
	if _, err := parseLSBLK([]byte(`{"blockdevices": [`)); err == nil {
		t.Fatal("parseLSBLK() error = nil, want decode error")
	}
}

func TestBoolishRejectsUnknownValue(t *testing.T) {
	input := []byte(`{"blockdevices":[{"type":"disk","rm":2}]}`)
	if _, err := parseLSBLK(input); err == nil {
		t.Fatal("parseLSBLK() error = nil, want invalid boolean error")
	}
}

func TestLikelyExternal(t *testing.T) {
	tests := []struct {
		name      string
		removable bool
		transport string
		want      bool
	}{
		{"removable SATA", true, "sata", true},
		{"USB bridge", false, "usb", true},
		{"internal NVMe", false, "nvme", false},
		{"internal SATA", false, "sata", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := likelyExternal(tt.removable, tt.transport); got != tt.want {
				t.Fatalf("likelyExternal(%v, %q) = %v, want %v", tt.removable, tt.transport, got, tt.want)
			}
		})
	}
}
