package appservice

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/soyunomas/storverity/internal/rawprobe"
	storagereport "github.com/soyunomas/storverity/internal/report"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

func TestReportManagerExportsJSONWithSuggestedName(t *testing.T) {
	var saved ReportSaveRequest
	manager := NewReportManager(func(req ReportSaveRequest) (string, error) {
		saved = req
		return "/tmp/report.json", nil
	})
	started := time.Date(2026, 9, 13, 20, 0, 0, 0, time.UTC)
	manager.Set(storagereport.Document{
		SchemaVersion: storagereport.SchemaVersion,
		App: storagereport.AppInfo{Name: "StorVerity", Version: "0.1.0"},
		Operation: storagereport.OperationFilesystem,
		Status: storagereport.StatusPassed,
		StartedAt: storagereport.Timestamp(started),
		CompletedAt: storagereport.Timestamp(started.Add(time.Second)),
		DurationMilliseconds: 1000,
		Device: storagereport.DeviceInfo{ID: "serial:USB", Path: "/dev/sdb", DisplayName: "USB", CapacityBytes: 1024},
		AdvertisedCapacityBytes: 1024,
		TestedCapacityBytes: 512,
		Errors: []storagereport.ErrorEntry{},
		Filesystem: &storagereport.FilesystemResult{MountPoint: "/media/USB", RequestedBytes: 512, BytesWritten: 512, BytesVerified: 512, Regions: 1},
	})
	path, err := manager.Export("json")
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/report.json" || !strings.HasSuffix(saved.SuggestedFilename, ".json") {
		t.Fatalf("path=%q saved=%+v", path, saved)
	}
	var decoded map[string]any
	if err := json.Unmarshal(saved.Data, &decoded); err != nil {
		t.Fatalf("exported JSON: %v", err)
	}
	if decoded["schemaVersion"] != storagereport.SchemaVersion {
		t.Fatalf("schemaVersion=%v", decoded["schemaVersion"])
	}
}

func TestReportManagerRejectsUnsupportedFormat(t *testing.T) {
	manager := NewReportManager(func(ReportSaveRequest) (string, error) { return "", nil })
	manager.Set(storagereport.Document{})
	if _, err := manager.Export("xml"); err == nil {
		t.Fatal("Export(xml) error = nil")
	}
}

func TestRecordFilesystemReportCapturesIdentityAndFailure(t *testing.T) {
	desktop := &Desktop{reports: NewReportManager(nil)}
	card := DeviceCard{ID: "serial:USB1", Path: "/dev/sdb", DisplayName: "Example Flash", Serial: "USB1", Transport: "usb", CapacityBytes: 64 << 30}
	started := time.Date(2026, 9, 13, 20, 0, 0, 0, time.UTC)
	regionErr := &verifyfs.RegionError{Region: 3, Kind: verifyfs.FailureCorrupt, Err: errors.New("mismatch")}
	desktop.recordFilesystemReport(card, VerificationRequest{DeviceID: card.ID, MountPoint: "/media/USB", TotalBytes: 256 << 20}, verifyfs.Report{BytesWritten: 256 << 20, BytesVerified: 48 << 20, Regions: 16}, regionErr, started, started.Add(time.Second))

	payload, err := desktop.LastReportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"status": "failed"`) || !strings.Contains(payload, `"region": 3`) || !strings.Contains(payload, `"capacityBytes": 68719476736`) {
		t.Fatalf("payload=%s", payload)
	}
}

func TestRecordRawReportMarksSuspiciousCapacity(t *testing.T) {
	desktop := &Desktop{reports: NewReportManager(nil)}
	card := DeviceCard{ID: "serial:USB1", Path: "/dev/sdb", DisplayName: "Example Flash", CapacityBytes: 2 << 40}
	started := time.Now().UTC()
	result := rawprobe.Report{
		AdvertisedBytes: 2 << 40,
		BlockBytes: 4096,
		Samples: 2,
		ValidSamples: 1,
		CorruptSamples: 1,
		SuspectFakeCapacity: true,
		Restored: true,
		Results: []rawprobe.SampleResult{
			{Index: 0, OffsetBytes: 4 << 20, Outcome: rawprobe.OutcomeValid, Restored: true},
			{Index: 1, OffsetBytes: 1 << 40, Outcome: rawprobe.OutcomeCorrupt, Error: "verification pattern mismatch", Restored: true},
		},
	}
	desktop.recordRawReport(card, RawProbeRequest{BlockBytes: 4096}, result, nil, started, started.Add(time.Second))
	payload, err := desktop.LastReportJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"status": "suspicious"`) || !strings.Contains(payload, `"suspectFakeCapacity": true`) || !strings.Contains(payload, `"code": "corrupt"`) {
		t.Fatalf("payload=%s", payload)
	}
}
