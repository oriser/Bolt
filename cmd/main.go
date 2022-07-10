package main

import (
	"fmt"
	"os"

	"github.com/oriser/bolt/cmd/run"
)

func main() {
	if err := run.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error running wolt bot: %v", err)
		os.Exit(1)
	}
}
