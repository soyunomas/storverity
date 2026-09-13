//go:build linux

package device

import "testing"

func TestParseLSBLKCapturesKernelDeviceNumber(t *testing.T) {
	input := []byte(`{"blockdevices":[{"name":"/dev/sdb","kname":"sdb","path":"/dev/sdb","maj:min":"8:16","type":"disk","tran":"usb","rm":true,"ro":false,"size":64000000,"mountpoints":[null]}]}`)
	devices, err := parseLSBLK(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].MajorMinor != "8:16" {
		t.Fatalf("devices=%+v", devices)
	}
}
