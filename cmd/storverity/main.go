package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/soyunomas/storverity/internal/appmeta"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "StorVerity:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 || args[0] != "version" {
		return errors.New("usage: storverity version")
	}
	fmt.Printf("%s %s\n", appmeta.Name, appmeta.Version)
	return nil
}
