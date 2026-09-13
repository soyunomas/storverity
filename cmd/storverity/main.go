package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/soyunomas/storverity/internal/appmeta"
	"github.com/soyunomas/storverity/internal/device"
	"github.com/soyunomas/storverity/internal/safety"
)

type listedDevice struct {
	device.Device
	RawTest safety.Decision `json:"rawTest"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "StorVerity:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: storverity <list|version>")
	}
	if args[0] == "version" {
		fmt.Printf("%s %s\n", appmeta.Name, appmeta.Version)
		return nil
	}
	if args[0] != "list" {
		return errors.New("usage: storverity <list|version>")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	devices, err := device.NewScanner().List(ctx)
	if err != nil {
		return err
	}

	out := make([]listedDevice, 0, len(devices))
	for _, d := range devices {
		out = append(out, listedDevice{Device: d, RawTest: safety.EvaluateRawTest(d)})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}
