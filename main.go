package main

import (
	"context"
	"embed"
	"log"
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

	emitVerification := func(progress appservice.VerificationProgress) {
		runtimeMu.RLock()
		ctx := runtimeCtx
		runtimeMu.RUnlock()
		if ctx != nil {
			runtime.EventsEmit(ctx, "verification:progress", progress)
		}
	}
	emitRawProbe := func(progress appservice.RawProbeProgress) {
		runtimeMu.RLock()
		ctx := runtimeCtx
		runtimeMu.RUnlock()
		if ctx != nil {
			runtime.EventsEmit(ctx, "rawprobe:progress", progress)
		}
	}

	app := appservice.NewLinuxDesktop(emitVerification, emitRawProbe)

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
