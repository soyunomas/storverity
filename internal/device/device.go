package device

// Device describes a whole-disk block device that StorVerity may present to
// the application layer. Discovery itself is strictly read-only.
type Device struct {
	Name           string   `json:"name"`
	KernelName     string   `json:"kernelName"`
	Path           string   `json:"path"`
	MajorMinor     string   `json:"majorMinor,omitempty"`
	Type           string   `json:"type"`
	Transport      string   `json:"transport,omitempty"`
	Removable      bool     `json:"removable"`
	ReadOnly       bool     `json:"readOnly"`
	SizeBytes      uint64   `json:"sizeBytes"`
	Model          string   `json:"model,omitempty"`
	Vendor         string   `json:"vendor,omitempty"`
	Serial         string   `json:"serial,omitempty"`
	MountPoints    []string `json:"mountPoints,omitempty"`
	FileSystems    []string `json:"fileSystems,omitempty"`
	ContainsSwap   bool     `json:"containsSwap"`
	SystemDisk     bool     `json:"systemDisk"`
	LikelyExternal bool     `json:"likelyExternal"`
}
