package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/AI4SciComp/asc-devtools/internal/app"
)

var (
	version   = "dev"
	commit    = ""
	buildDate = ""
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	code := app.Run(ctx, os.Args[1:], app.Dependencies{
		VersionInfo: app.VersionInfo{Version: version, Commit: commit, BuildDate: buildDate},
	})
	os.Exit(code)
}
