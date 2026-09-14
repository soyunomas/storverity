package privhelper

import "github.com/soyunomas/storverity/internal/rawprobe"

const (
	ServiceName  = "io.github.soyunomas.StorVerity.Helper1"
	ObjectPath   = "/io/github/soyunomas/StorVerity/Helper1"
	Interface    = "io.github.soyunomas.StorVerity.Helper1"
	PolkitAction = "io.github.soyunomas.StorVerity.raw-probe"
)

type ErrorCode string

const (
	ErrorUnauthorized ErrorCode = "unauthorized"
	ErrorBusy         ErrorCode = "busy"
	ErrorProtected    ErrorCode = "protected"
	ErrorIdentity     ErrorCode = "identity-changed"
	ErrorCancelled    ErrorCode = "cancelled"
	ErrorInvalid      ErrorCode = "invalid-request"
	ErrorOpen         ErrorCode = "open-failed"
	ErrorProbe        ErrorCode = "probe-failed"
	ErrorInternal     ErrorCode = "internal"
)

type RunRequest struct {
	SessionID           string `json:"sessionId"`
	DeviceID            string `json:"deviceId"`
	ExpectedFingerprint string `json:"expectedFingerprint"`
	Samples             int    `json:"samples"`
	BlockBytes          uint64 `json:"blockBytes"`
}

type RunError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type RunResponse struct {
	Report rawprobe.Report `json:"report"`
	Error  *RunError       `json:"error,omitempty"`
}
