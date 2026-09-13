package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sampleFilesystemDocument() Document {
	started := time.Date(2026, 9, 13, 20, 0, 0, 0, time.UTC)
	completed := started.Add(1500 * time.Millisecond)
	return Document{
		SchemaVersion: SchemaVersion,
		App: AppInfo{Name: "StorVerity", Version: "0.1.0", Commit: "abc123"},
		Operation: OperationFilesystem,
		Status: StatusPassed,
		StartedAt: Timestamp(started),
		CompletedAt: Timestamp(completed),
		DurationMilliseconds: DurationMilliseconds(started, completed),
		Device: DeviceInfo{ID: "serial:USB1", Path: "/dev/sdb", DisplayName: "Example Flash", Serial: "USB1", Transport: "usb", CapacityBytes: 64 << 30},
		AdvertisedCapacityBytes: 64 << 30,
		TestedCapacityBytes: 256 << 20,
		Errors: []ErrorEntry{},
		Filesystem: &FilesystemResult{MountPoint: "/media/USB", RequestedBytes: 256 << 20, BytesWritten: 256 << 20, BytesVerified: 256 << 20, Regions: 16},
	}
}

func TestJSONHasStableTopLevelContract(t *testing.T) {
	payload, err := JSON(sampleFilesystemDocument())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	want := []string{"schemaVersion", "app", "operation", "status", "startedAt", "completedAt", "durationMilliseconds", "device", "advertisedCapacityBytes", "testedCapacityBytes", "errors", "filesystem"}
	for _, key := range want {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("JSON contract missing %q: %s", key, payload)
		}
	}
	if string(decoded["schemaVersion"]) != `"storverity.report.v1"` {
		t.Fatalf("schemaVersion=%s", decoded["schemaVersion"])
	}
	if string(decoded["errors"]) != "[]" {
		t.Fatalf("errors must serialize as [], got %s", decoded["errors"])
	}
}

func TestTextIsHumanReadable(t *testing.T) {
	payload, err := Text(sampleFilesystemDocument())
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	for _, want := range []string{"StorVerity Verification Report", "PASSED", "Example Flash", "/media/USB", "256.00 MiB", "Errors\n------\nNone"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text report missing %q:\n%s", want, text)
		}
	}
}

func TestRawDocumentRequiresResultCount(t *testing.T) {
	started := time.Now().UTC()
	doc := Document{
		SchemaVersion: SchemaVersion,
		App: AppInfo{Name: "StorVerity", Version: "0.1.0"},
		Operation: OperationRaw,
		Status: StatusSuspicious,
		StartedAt: Timestamp(started),
		CompletedAt: Timestamp(started),
		Device: DeviceInfo{ID: "serial:USB1", Path: "/dev/sdb", DisplayName: "USB", CapacityBytes: 1000},
		AdvertisedCapacityBytes: 1000,
		Errors: []ErrorEntry{},
		Raw: &RawResult{BlockBytes: 512, Samples: 2, Results: []RawSample{{Index: 0}}},
	}
	if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), "result count") {
		t.Fatalf("Validate() err=%v", err)
	}
}

func TestValidateRejectsNullErrors(t *testing.T) {
	doc := sampleFilesystemDocument()
	doc.Errors = nil
	if err := doc.Validate(); err == nil || !strings.Contains(err.Error(), "empty array") {
		t.Fatalf("Validate() err=%v", err)
	}
}
