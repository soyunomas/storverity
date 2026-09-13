package appservice

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/soyunomas/storverity/internal/appmeta"
	"github.com/soyunomas/storverity/internal/rawprobe"
	"github.com/soyunomas/storverity/internal/report"
	"github.com/soyunomas/storverity/internal/verifyfs"
)

type ReportSaveRequest struct {
	SuggestedFilename string
	Title             string
	DisplayName       string
	Pattern           string
	Data              []byte
}

type ReportSaver func(ReportSaveRequest) (string, error)

type ReportManager struct {
	mu    sync.RWMutex
	last  *report.Document
	saver ReportSaver
}

func NewReportManager(saver ReportSaver) *ReportManager {
	return &ReportManager{saver: saver}
}

func (m *ReportManager) Set(doc report.Document) {
	if m == nil {
		return
	}
	m.mu.Lock()
	copy := doc
	m.last = &copy
	m.mu.Unlock()
}

func (m *ReportManager) document() (report.Document, error) {
	if m == nil {
		return report.Document{}, errors.New("report service is not configured")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.last == nil {
		return report.Document{}, errors.New("no verification report is available yet")
	}
	return *m.last, nil
}

func (m *ReportManager) JSON() (string, error) {
	doc, err := m.document()
	if err != nil {
		return "", err
	}
	payload, err := report.JSON(doc)
	return string(payload), err
}

func (m *ReportManager) Text() (string, error) {
	doc, err := m.document()
	if err != nil {
		return "", err
	}
	payload, err := report.Text(doc)
	return string(payload), err
}

func (m *ReportManager) Export(format string) (string, error) {
	doc, err := m.document()
	if err != nil {
		return "", err
	}
	if m.saver == nil {
		return "", errors.New("report export is not available in this runtime")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	var payload []byte
	var req ReportSaveRequest
	timestamp := doc.CompletedAt
	if parsed, parseErr := time.Parse(time.RFC3339Nano, doc.CompletedAt); parseErr == nil {
		timestamp = parsed.UTC().Format("20060102T150405Z")
	} else {
		timestamp = strings.NewReplacer(":", "-", "/", "-").Replace(timestamp)
	}
	base := fmt.Sprintf("storverity-%s-%s", doc.Operation, timestamp)
	switch format {
	case "json":
		payload, err = report.JSON(doc)
		req = ReportSaveRequest{SuggestedFilename: base + ".json", Title: "Save StorVerity JSON report", DisplayName: "JSON report (*.json)", Pattern: "*.json"}
	case "text", "txt":
		payload, err = report.Text(doc)
		req = ReportSaveRequest{SuggestedFilename: base + ".txt", Title: "Save StorVerity text report", DisplayName: "Text report (*.txt)", Pattern: "*.txt"}
	default:
		return "", fmt.Errorf("unsupported report format %q", format)
	}
	if err != nil {
		return "", err
	}
	req.Data = payload
	return m.saver(req)
}

func (d *Desktop) snapshotReportDevice(deviceID string) DeviceCard {
	fallback := DeviceCard{ID: strings.TrimSpace(deviceID), Path: "unavailable", DisplayName: "Unavailable device"}
	if d == nil || d.devices == nil {
		return fallback
	}
	timeout := d.listTimeout
	if timeout <= 0 {
		timeout = defaultListTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cards, err := d.devices.ListDevices(ctx)
	if err != nil {
		return fallback
	}
	for _, card := range cards {
		if card.ID == deviceID {
			return card
		}
	}
	return fallback
}

func reportDevice(card DeviceCard) report.DeviceInfo {
	return report.DeviceInfo{
		ID: card.ID, Path: card.Path, DisplayName: card.DisplayName, Vendor: card.Vendor,
		Model: card.Model, Serial: card.Serial, Transport: card.Transport, CapacityBytes: card.CapacityBytes,
	}
}

func appReportInfo() report.AppInfo {
	return report.AppInfo{Name: appmeta.Name, Version: appmeta.Version, Commit: appmeta.Commit, BuildDate: appmeta.BuildDate}
}

func (d *Desktop) recordFilesystemReport(card DeviceCard, req VerificationRequest, result verifyfs.Report, runErr error, started, completed time.Time) {
	if d == nil || d.reports == nil {
		return
	}
	status := report.StatusPassed
	if errors.Is(runErr, context.Canceled) {
		status = report.StatusCancelled
	} else if runErr != nil {
		status = report.StatusFailed
	}
	errorsList := make([]report.ErrorEntry, 0, 1)
	if runErr != nil {
		entry := report.ErrorEntry{Code: "operation-error", Message: runErr.Error()}
		var regionErr *verifyfs.RegionError
		if errors.As(runErr, &regionErr) {
			region := regionErr.Region
			entry.Code = string(regionErr.Kind)
			entry.Region = &region
		}
		errorsList = append(errorsList, entry)
	}
	tested := uint64(0)
	if result.BytesVerified > 0 {
		tested = uint64(result.BytesVerified)
	}
	doc := report.Document{
		SchemaVersion:            report.SchemaVersion,
		App:                      appReportInfo(),
		Operation:                report.OperationFilesystem,
		Status:                   status,
		StartedAt:                report.Timestamp(started),
		CompletedAt:              report.Timestamp(completed),
		DurationMilliseconds:     report.DurationMilliseconds(started, completed),
		Device:                   reportDevice(card),
		AdvertisedCapacityBytes: card.CapacityBytes,
		TestedCapacityBytes:      tested,
		Errors:                   errorsList,
		Filesystem: &report.FilesystemResult{
			MountPoint: req.MountPoint, RequestedBytes: req.TotalBytes, BytesWritten: result.BytesWritten,
			BytesVerified: result.BytesVerified, Regions: result.Regions,
		},
	}
	d.reports.Set(doc)
}

func (d *Desktop) recordRawReport(card DeviceCard, req RawProbeRequest, result rawprobe.Report, runErr error, started, completed time.Time) {
	if d == nil || d.reports == nil {
		return
	}
	status := report.StatusPassed
	if errors.Is(runErr, context.Canceled) {
		status = report.StatusCancelled
	} else if runErr != nil || result.RestoreErrors > 0 || (result.Samples > 0 && !result.Restored) {
		status = report.StatusFailed
	} else if result.SuspectFakeCapacity {
		status = report.StatusSuspicious
	}
	errorsList := make([]report.ErrorEntry, 0, result.CorruptSamples+result.ReadErrors+result.WriteErrors+result.RestoreErrors+1)
	samples := make([]report.RawSample, len(result.Results))
	for i, sample := range result.Results {
		samples[i] = report.RawSample{
			Index: sample.Index, OffsetBytes: sample.OffsetBytes, Outcome: string(sample.Outcome), Error: sample.Error,
			Restored: sample.Restored, RestoreError: sample.RestoreError,
		}
		if sample.Error != "" {
			index, offset := sample.Index, sample.OffsetBytes
			errorsList = append(errorsList, report.ErrorEntry{Code: string(sample.Outcome), Message: sample.Error, Sample: &index, OffsetBytes: &offset})
		}
		if sample.RestoreError != "" {
			index, offset := sample.Index, sample.OffsetBytes
			errorsList = append(errorsList, report.ErrorEntry{Code: "restore-error", Message: sample.RestoreError, Sample: &index, OffsetBytes: &offset})
		}
	}
	if runErr != nil {
		errorsList = append(errorsList, report.ErrorEntry{Code: "operation-error", Message: runErr.Error()})
	}
	advertised := result.AdvertisedBytes
	if advertised == 0 {
		advertised = card.CapacityBytes
	}
	blockBytes := result.BlockBytes
	if blockBytes == 0 {
		blockBytes = req.BlockBytes
		if blockBytes == 0 {
			blockBytes = rawprobe.DefaultBlockBytes
		}
	}
	tested := uint64(result.Samples) * blockBytes
	doc := report.Document{
		SchemaVersion:            report.SchemaVersion,
		App:                      appReportInfo(),
		Operation:                report.OperationRaw,
		Status:                   status,
		StartedAt:                report.Timestamp(started),
		CompletedAt:              report.Timestamp(completed),
		DurationMilliseconds:     report.DurationMilliseconds(started, completed),
		Device:                   reportDevice(card),
		AdvertisedCapacityBytes: card.CapacityBytes,
		TestedCapacityBytes:      tested,
		Errors:                   errorsList,
		Raw: &report.RawResult{
			AdvertisedBytes: advertised, BlockBytes: blockBytes, Samples: result.Samples,
			ValidSamples: result.ValidSamples, CorruptSamples: result.CorruptSamples, ReadErrors: result.ReadErrors,
			WriteErrors: result.WriteErrors, RestoreErrors: result.RestoreErrors, ValidatedThroughBytes: result.ValidatedThroughBytes,
			SuspectFakeCapacity: result.SuspectFakeCapacity, Restored: result.Restored, Results: samples,
		},
	}
	d.reports.Set(doc)
}

func (d *Desktop) LastReportJSON() (string, error) {
	if d == nil || d.reports == nil {
		return "", errors.New("report service is not configured")
	}
	return d.reports.JSON()
}

func (d *Desktop) LastReportText() (string, error) {
	if d == nil || d.reports == nil {
		return "", errors.New("report service is not configured")
	}
	return d.reports.Text()
}

func (d *Desktop) ExportLastReport(format string) (string, error) {
	if d == nil || d.reports == nil {
		return "", errors.New("report service is not configured")
	}
	return d.reports.Export(format)
}
