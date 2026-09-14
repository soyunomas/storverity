package appservice

import (
	"reflect"
	"testing"

	"github.com/soyunomas/storverity/internal/device"
)

func TestParseMountInfoAndPreferWritableMounts(t *testing.T) {
	modes, err := parseMountInfo([]byte(
		"31 22 8:17 / /media/yo/VTOYEFI ro,nosuid,nodev,relatime - vfat /dev/sdb2 ro\n" +
			"32 22 8:1 / /media/yo/Ventoy rw,nosuid,nodev,relatime - exfat /dev/sdb1 rw\n",
	))
	if err != nil {
		t.Fatal(err)
	}

	d := device.Device{MountPoints: []string{"/media/yo/VTOYEFI", "/media/yo/Ventoy"}}
	applyMountModes(&d, modes)

	if want := []string{"/media/yo/Ventoy", "/media/yo/VTOYEFI"}; !reflect.DeepEqual(d.MountPoints, want) {
		t.Fatalf("mount order = %#v, want %#v", d.MountPoints, want)
	}
	if want := []string{"/media/yo/Ventoy"}; !reflect.DeepEqual(d.WritableMountPoints, want) {
		t.Fatalf("writable mounts = %#v, want %#v", d.WritableMountPoints, want)
	}
}

func TestParseMountInfoDecodesEscapedMountPoint(t *testing.T) {
	modes, err := parseMountInfo([]byte("41 22 8:1 / /media/user/My\\040Drive rw,relatime - exfat /dev/sdb1 rw\n"))
	if err != nil {
		t.Fatal(err)
	}
	mode, ok := modes["/media/user/My Drive"]
	if !ok || mode.readOnly {
		t.Fatalf("decoded mode = %+v, present=%v", mode, ok)
	}
}

func TestApplyMountModesTreatsUnknownMountAsNotWritable(t *testing.T) {
	d := device.Device{MountPoints: []string{"/media/USB"}}
	applyMountModes(&d, map[string]mountMode{})
	if len(d.WritableMountPoints) != 0 {
		t.Fatalf("writable mounts = %#v, want none", d.WritableMountPoints)
	}
	if want := []string{"/media/USB"}; !reflect.DeepEqual(d.MountPoints, want) {
		t.Fatalf("mount points = %#v, want %#v", d.MountPoints, want)
	}
}
