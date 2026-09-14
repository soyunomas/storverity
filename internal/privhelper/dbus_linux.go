//go:build linux

package privhelper

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/soyunomas/storverity/internal/rawprobe"
)

const (
	polkitService   = "org.freedesktop.PolicyKit1"
	polkitPath      = dbus.ObjectPath("/org/freedesktop/PolicyKit1/Authority")
	polkitInterface = "org.freedesktop.PolicyKit1.Authority"
)

type DBusAuthorizer struct{ conn *dbus.Conn }

func NewDBusAuthorizer(conn *dbus.Conn) *DBusAuthorizer { return &DBusAuthorizer{conn: conn} }

type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}

type polkitResult struct {
	Authorized bool
	Challenge  bool
	Details    map[string]string
}

func (a *DBusAuthorizer) Authorize(ctx context.Context, caller, action string, details map[string]string) error {
	if a == nil || a.conn == nil || strings.TrimSpace(caller) == "" || strings.TrimSpace(action) == "" {
		return ErrUnauthorized
	}
	subject := polkitSubject{
		Kind: "system-bus-name",
		Details: map[string]dbus.Variant{
			"name": dbus.MakeVariant(caller),
		},
	}
	var result polkitResult
	call := a.conn.Object(polkitService, polkitPath).CallWithContext(
		ctx,
		polkitInterface+".CheckAuthorization",
		0,
		subject,
		action,
		details,
		uint32(1), // AllowUserInteraction.
		"",
	)
	if call.Err != nil {
		return fmt.Errorf("polkit check authorization: %w", call.Err)
	}
	if err := call.Store(&result); err != nil {
		return fmt.Errorf("decode polkit authorization result: %w", err)
	}
	if !result.Authorized {
		return ErrUnauthorized
	}
	return nil
}

type DBusService struct {
	conn   *dbus.Conn
	server *Server
}

func NewDBusService(conn *dbus.Conn, server *Server) *DBusService {
	return &DBusService{conn: conn, server: server}
}

func (s *DBusService) RunRawProbe(sender dbus.Sender, payload string) (string, *dbus.Error) {
	if s == nil || s.conn == nil || s.server == nil {
		return "", dbus.MakeFailedError(errors.New("privileged DBus service is not configured"))
	}
	var req RunRequest
	if err := json.Unmarshal([]byte(payload), &req); err != nil {
		response, _ := json.Marshal(responseError(ErrorInvalid, fmt.Errorf("decode request: %w", err)))
		return string(response), nil
	}
	response := s.server.RunRawProbe(context.Background(), string(sender), req, func(progress rawprobe.Progress) {
		encoded, err := json.Marshal(progress)
		if err != nil {
			return
		}
		_ = s.conn.Emit(dbus.ObjectPath(ObjectPath), Interface+".Progress", req.SessionID, string(encoded))
	})
	encoded, err := json.Marshal(response)
	if err != nil {
		return "", dbus.MakeFailedError(fmt.Errorf("encode privileged response: %w", err))
	}
	return string(encoded), nil
}

func (s *DBusService) CancelRawProbe(sender dbus.Sender, sessionID string) (bool, *dbus.Error) {
	if s == nil || s.server == nil {
		return false, dbus.MakeFailedError(errors.New("privileged DBus service is not configured"))
	}
	return s.server.Cancel(string(sender), sessionID), nil
}

func ExportSystemService(ctx context.Context, conn *dbus.Conn, server *Server) error {
	if conn == nil || server == nil {
		return errors.New("DBus connection and server are required")
	}
	if err := conn.Export(NewDBusService(conn, server), dbus.ObjectPath(ObjectPath), Interface); err != nil {
		return fmt.Errorf("export privileged DBus service: %w", err)
	}
	reply, err := conn.RequestName(ServiceName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return fmt.Errorf("request privileged DBus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		return fmt.Errorf("DBus name %s is already owned", ServiceName)
	}
	monitorCallerDisconnects(ctx, conn, server)
	return nil
}

func monitorCallerDisconnects(ctx context.Context, conn *dbus.Conn, server *Server) {
	options := []dbus.MatchOption{
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	}
	if err := conn.AddMatchSignal(options...); err != nil {
		return
	}
	signals := make(chan *dbus.Signal, 32)
	conn.Signal(signals)
	go func() {
		defer conn.RemoveSignal(signals)
		defer conn.RemoveMatchSignal(options...)
		for {
			select {
			case <-ctx.Done():
				return
			case signal, ok := <-signals:
				if !ok || signal == nil || len(signal.Body) != 3 {
					continue
				}
				name, ok1 := signal.Body[0].(string)
				oldOwner, ok2 := signal.Body[1].(string)
				newOwner, ok3 := signal.Body[2].(string)
				if ok1 && ok2 && ok3 && strings.HasPrefix(name, ":") && oldOwner != "" && newOwner == "" {
					server.CancelCaller(name)
				}
			}
		}
	}()
}

type Client struct{}

func NewSystemClient() *Client { return &Client{} }

func NewSessionID() (string, error) {
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(token[:]), nil
}

func (c *Client) RunRawProbe(ctx context.Context, req RunRequest, emit func(rawprobe.Progress)) (rawprobe.Report, error) {
	if c == nil {
		return rawprobe.Report{}, errors.New("privileged helper client is not configured")
	}
	if req.SessionID == "" {
		sessionID, err := NewSessionID()
		if err != nil {
			return rawprobe.Report{}, fmt.Errorf("create privileged session id: %w", err)
		}
		req.SessionID = sessionID
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return rawprobe.Report{}, fmt.Errorf("encode privileged request: %w", err)
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		return rawprobe.Report{}, fmt.Errorf("connect system DBus: %w", err)
	}
	options := []dbus.MatchOption{
		dbus.WithMatchObjectPath(dbus.ObjectPath(ObjectPath)),
		dbus.WithMatchInterface(Interface),
		dbus.WithMatchMember("Progress"),
		dbus.WithMatchArg(0, req.SessionID),
	}
	if err := conn.AddMatchSignal(options...); err != nil {
		return rawprobe.Report{}, fmt.Errorf("subscribe privileged progress: %w", err)
	}
	defer conn.RemoveMatchSignal(options...)
	signals := make(chan *dbus.Signal, 64)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	object := conn.Object(ServiceName, dbus.ObjectPath(ObjectPath))
	calls := make(chan *dbus.Call, 1)
	object.Go(Interface+".RunRawProbe", 0, calls, string(payload))

	cancel := ctx.Done()
	for {
		select {
		case <-cancel:
			cancel = nil
			_ = object.Call(Interface+".CancelRawProbe", 0, req.SessionID).Err
		case signal := <-signals:
			if signal == nil || len(signal.Body) != 2 {
				continue
			}
			progressPayload, ok := signal.Body[1].(string)
			if !ok {
				continue
			}
			var progress rawprobe.Progress
			if err := json.Unmarshal([]byte(progressPayload), &progress); err == nil && emit != nil {
				emit(progress)
			}
		case call := <-calls:
			if call == nil {
				return rawprobe.Report{}, errors.New("privileged helper returned no DBus call")
			}
			if call.Err != nil {
				return rawprobe.Report{}, fmt.Errorf("privileged raw probe DBus call: %w", call.Err)
			}
			var responsePayload string
			if err := call.Store(&responsePayload); err != nil {
				return rawprobe.Report{}, fmt.Errorf("decode privileged DBus response: %w", err)
			}
			var response RunResponse
			if err := json.Unmarshal([]byte(responsePayload), &response); err != nil {
				return rawprobe.Report{}, fmt.Errorf("decode privileged raw probe response: %w", err)
			}
			if response.Error == nil {
				return response.Report, nil
			}
			return response.Report, clientError(response.Error)
		}
	}
}

func clientError(runErr *RunError) error {
	if runErr == nil {
		return nil
	}
	switch runErr.Code {
	case ErrorCancelled:
		return context.Canceled
	case ErrorUnauthorized:
		return fmt.Errorf("%w: %s", ErrUnauthorized, runErr.Message)
	case ErrorBusy:
		return fmt.Errorf("%w: %s", ErrBusy, runErr.Message)
	case ErrorProtected:
		return fmt.Errorf("%w: %s", ErrProtected, runErr.Message)
	case ErrorIdentity:
		return fmt.Errorf("%w: %s", ErrIdentity, runErr.Message)
	case ErrorInvalid:
		return fmt.Errorf("%w: %s", ErrInvalid, runErr.Message)
	default:
		return errors.New(runErr.Message)
	}
}
