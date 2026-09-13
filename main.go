package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/soyunomas/storverity/internal/appservice"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	var runtimeMu sync.RWMutex
	var runtimeCtx context.Context

	currentContext := func() context.Context {
		runtimeMu.RLock()
		defer runtimeMu.RUnlock()
		return runtimeCtx
	}
	emitVerification := func(progress appservice.VerificationProgress) {
		if ctx := currentContext(); ctx != nil {
			runtime.EventsEmit(ctx, "verification:progress", progress)
		}
	}
	emitRawProbe := func(progress appservice.RawProbeProgress) {
		if ctx := currentContext(); ctx != nil {
			runtime.EventsEmit(ctx, "rawprobe:progress", progress)
		}
	}
	saveReport := func(req appservice.ReportSaveRequest) (string, error) {
		ctx := currentContext()
		if ctx == nil {
			return "", errors.New("desktop runtime is not ready")
		}
		path, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
			Title:           req.Title,
			DefaultFilename: req.SuggestedFilename,
			Filters: []runtime.FileFilter{{DisplayName: req.DisplayName, Pattern: req.Pattern}},
		})
		if err != nil {
			return "", fmt.Errorf("choose report destination: %w", err)
		}
		if path == "" {
			return "", nil
		}
		if err := writeReportAtomically(path, req.Data); err != nil {
			return "", err
		}
		return path, nil
	}

	app := appservice.NewLinuxDesktopWithReportSaver(emitVerification, emitRawProbe, saveReport)

	err := wails.Run(&options.App{
		Title:            "StorVerity",
		Width:            1180,
		Height:           760,
		MinWidth:         900,
		MinHeight:        620,
		BackgroundColour: &options.RGBA{R: 10, G: 15, B: 24, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: func(ctx context.Context) {
			runtimeMu.Lock()
			runtimeCtx = ctx
			runtimeMu.Unlock()
		},
		OnShutdown: func(context.Context) {
			app.CancelVerification()
			app.CancelRawProbe()
			runtimeMu.Lock()
			runtimeCtx = nil
			runtimeMu.Unlock()
		},
		Bind: []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

func writeReportAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".storverity-report-*")
	if err != nil {
		return fmt.Errorf("create temporary report: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set report permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write report: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("flush report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close report: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publish report: %w", err)
	}
	return nil
}
