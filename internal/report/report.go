package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const SchemaVersion = "storverity.report.v1"

type Operation string

type Status string

const (
	OperationFilesystem Operation = "filesystem"
	OperationRaw        Operation = "raw"

	StatusPassed     Status = "passed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
	StatusSuspicious Status = "suspicious"
)

type AppInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"buildDate,omitempty"`
}

type DeviceInfo struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	DisplayName   string `json:"displayName"`
	Vendor        string `json:"vendor,omitempty"`
	Model         string `json:"model,omitempty"`
	Serial        string `json:"serial,omitempty"`
	Transport     string `json:"transport,omitempty"`
	CapacityBytes uint64 `json:"capacityBytes"`
}

type ErrorEntry struct {
	Code        string  `json:"code"`
	Message     string  `json:"message"`
	Region      *int    `json:"region,omitempty"`
	Sample      *int    `json:"sample,omitempty"`
	OffsetBytes *uint64 `json:"offsetBytes,omitempty"`
}

type FilesystemResult struct {
	MountPoint    string `json:"mountPoint"`
	RequestedBytes int64  `json:"requestedBytes"`
	BytesWritten   int64  `json:"bytesWritten"`
	BytesVerified  int64  `json:"bytesVerified"`
	Regions        int    `json:"regions"`
}

type RawSample struct {
	Index        int    `json:"index"`
	OffsetBytes  uint64 `json:"offsetBytes"`
	Outcome      string `json:"outcome"`
	Error        string `json:"error,omitempty"`
	Restored     bool   `json:"restored"`
	RestoreError string `json:"restoreError,omitempty"`
}

type RawResult struct {
	AdvertisedBytes       uint64      `json:"advertisedBytes"`
	BlockBytes            uint64      `json:"blockBytes"`
	Samples               int         `json:"samples"`
	ValidSamples          int         `json:"validSamples"`
	CorruptSamples        int         `json:"corruptSamples"`
	ReadErrors            int         `json:"readErrors"`
	WriteErrors           int         `json:"writeErrors"`
	RestoreErrors         int         `json:"restoreErrors"`
	ValidatedThroughBytes uint64      `json:"validatedThroughBytes"`
	SuspectFakeCapacity   bool        `json:"suspectFakeCapacity"`
	Restored              bool        `json:"restored"`
	Results               []RawSample `json:"results"`
}

type Document struct {
	SchemaVersion       string            `json:"schemaVersion"`
	App                 AppInfo           `json:"app"`
	Operation           Operation         `json:"operation"`
	Status              Status            `json:"status"`
	StartedAt           string            `json:"startedAt"`
	CompletedAt         string            `json:"completedAt"`
	DurationMilliseconds int64             `json:"durationMilliseconds"`
	Device              DeviceInfo        `json:"device"`
	AdvertisedCapacityBytes uint64         `json:"advertisedCapacityBytes"`
	TestedCapacityBytes uint64             `json:"testedCapacityBytes"`
	Errors              []ErrorEntry      `json:"errors"`
	Filesystem          *FilesystemResult `json:"filesystem,omitempty"`
	Raw                 *RawResult        `json:"raw,omitempty"`
}

func Timestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func DurationMilliseconds(started, completed time.Time) int64 {
	if completed.Before(started) {
		return 0
	}
	return completed.Sub(started).Milliseconds()
}

func (d Document) Validate() error {
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported report schema %q", d.SchemaVersion)
	}
	if strings.TrimSpace(d.App.Name) == "" || strings.TrimSpace(d.App.Version) == "" {
		return errors.New("report app name and version are required")
	}
	if strings.TrimSpace(d.Device.ID) == "" {
		return errors.New("report device id is required")
	}
	if d.Device.CapacityBytes != d.AdvertisedCapacityBytes {
		return errors.New("device and advertised capacities disagree")
	}
	started, err := time.Parse(time.RFC3339Nano, d.StartedAt)
	if err != nil {
		return fmt.Errorf("parse startedAt: %w", err)
	}
	completed, err := time.Parse(time.RFC3339Nano, d.CompletedAt)
	if err != nil {
		return fmt.Errorf("parse completedAt: %w", err)
	}
	if completed.Before(started) {
		return errors.New("completedAt precedes startedAt")
	}
	if d.DurationMilliseconds < 0 {
		return errors.New("duration must not be negative")
	}
	switch d.Status {
	case StatusPassed, StatusFailed, StatusCancelled, StatusSuspicious:
	default:
		return fmt.Errorf("unsupported report status %q", d.Status)
	}
	switch d.Operation {
	case OperationFilesystem:
		if d.Filesystem == nil || d.Raw != nil {
			return errors.New("filesystem report must contain only filesystem details")
		}
		if d.Filesystem.RequestedBytes <= 0 || d.Filesystem.BytesWritten < 0 || d.Filesystem.BytesVerified < 0 || d.Filesystem.Regions < 0 {
			return errors.New("filesystem report contains invalid counters")
		}
	case OperationRaw:
		if d.Raw == nil || d.Filesystem != nil {
			return errors.New("raw report must contain only raw details")
		}
		if d.Raw.BlockBytes == 0 || d.Raw.Samples < 0 || d.Raw.ValidSamples < 0 || d.Raw.CorruptSamples < 0 || d.Raw.ReadErrors < 0 || d.Raw.WriteErrors < 0 || d.Raw.RestoreErrors < 0 {
			return errors.New("raw report contains invalid counters")
		}
		if len(d.Raw.Results) != d.Raw.Samples {
			return fmt.Errorf("raw sample result count %d does not match samples %d", len(d.Raw.Results), d.Raw.Samples)
		}
	default:
		return fmt.Errorf("unsupported report operation %q", d.Operation)
	}
	if d.Errors == nil {
		return errors.New("errors must be an empty array instead of null")
	}
	return nil
}

func JSON(d Document) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return json.MarshalIndent(d, "", "  ")
}

func Text(d Document) ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	line := func(label, value string) {
		fmt.Fprintf(&out, "%-24s %s\n", label+":", value)
	}
	out.WriteString("StorVerity Verification Report\n")
	out.WriteString(strings.Repeat("=", 31) + "\n\n")
	line("Schema", d.SchemaVersion)
	line("Application", d.App.Name+" "+d.App.Version)
	if d.App.Commit != "" {
		line("Commit", d.App.Commit)
	}
	line("Operation", string(d.Operation))
	line("Status", strings.ToUpper(string(d.Status)))
	line("Started", d.StartedAt)
	line("Completed", d.CompletedAt)
	line("Duration", fmt.Sprintf("%d ms", d.DurationMilliseconds))
	out.WriteByte('\n')
	line("Device", d.Device.DisplayName)
	line("Device ID", d.Device.ID)
	line("Path", d.Device.Path)
	if d.Device.Serial != "" {
		line("Serial", d.Device.Serial)
	}
	if d.Device.Transport != "" {
		line("Transport", d.Device.Transport)
	}
	line("Advertised capacity", humanBytes(d.AdvertisedCapacityBytes))
	line("Tested data", humanBytes(d.TestedCapacityBytes))

	if d.Filesystem != nil {
		out.WriteByte('\n')
		out.WriteString("Filesystem verification\n")
		out.WriteString("-----------------------\n")
		line("Mount point", d.Filesystem.MountPoint)
		line("Requested", humanBytes(uint64(d.Filesystem.RequestedBytes)))
		line("Written", humanBytes(uint64(d.Filesystem.BytesWritten)))
		line("Verified", humanBytes(uint64(d.Filesystem.BytesVerified)))
		line("Regions", strconv.Itoa(d.Filesystem.Regions))
	}
	if d.Raw != nil {
		out.WriteByte('\n')
		out.WriteString("Raw capacity probe\n")
		out.WriteString("------------------\n")
		line("Samples", strconv.Itoa(d.Raw.Samples))
		line("Block size", humanBytes(d.Raw.BlockBytes))
		line("Valid samples", strconv.Itoa(d.Raw.ValidSamples))
		line("Corrupt samples", strconv.Itoa(d.Raw.CorruptSamples))
		line("Read errors", strconv.Itoa(d.Raw.ReadErrors))
		line("Write errors", strconv.Itoa(d.Raw.WriteErrors))
		line("Restore errors", strconv.Itoa(d.Raw.RestoreErrors))
		line("Validated through", humanBytes(d.Raw.ValidatedThroughBytes))
		line("Fake capacity suspected", strconv.FormatBool(d.Raw.SuspectFakeCapacity))
		line("Touched blocks restored", strconv.FormatBool(d.Raw.Restored))
	}

	out.WriteByte('\n')
	out.WriteString("Errors\n")
	out.WriteString("------\n")
	if len(d.Errors) == 0 {
		out.WriteString("None\n")
	} else {
		errs := append([]ErrorEntry(nil), d.Errors...)
		sort.SliceStable(errs, func(i, j int) bool {
			if errs[i].Code != errs[j].Code {
				return errs[i].Code < errs[j].Code
			}
			return errs[i].Message < errs[j].Message
		})
		for _, entry := range errs {
			fmt.Fprintf(&out, "- [%s] %s", entry.Code, entry.Message)
			if entry.Region != nil {
				fmt.Fprintf(&out, " (region %d)", *entry.Region)
			}
			if entry.Sample != nil {
				fmt.Fprintf(&out, " (sample %d)", *entry.Sample)
			}
			if entry.OffsetBytes != nil {
				fmt.Fprintf(&out, " @ %d bytes", *entry.OffsetBytes)
			}
			out.WriteByte('\n')
		}
	}
	return out.Bytes(), nil
}

func humanBytes(value uint64) string {
	const unit = uint64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := unit, 0
	for n := value / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
