// Command where lists shared people's cities and local times.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/icco/where/internal/cli"
)

// Version and CommitSHA are supplied by GoReleaser.
var Version = "dev"
var CommitSHA = "unknown"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := cli.New(Version, CommitSHA).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "where:", err)
		return 1
	}
	return 0
}
