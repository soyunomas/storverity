//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/soyunomas/storverity/internal/privhelper"
)

func main() {
	if os.Geteuid() != 0 {
		log.Fatal("storverity-helper must run as root via the system DBus service")
	}
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(fmt.Errorf("connect system DBus: %w", err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	authorizer := privhelper.NewDBusAuthorizer(conn)
	server := privhelper.NewLinuxServer(authorizer)
	if err := privhelper.ExportSystemService(ctx, conn, server); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("helper stopped: %v", err)
	}
}
