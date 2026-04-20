package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/charmbracelet/bubbletea"
	"github.com/shidaxi/iaws/internal/app"
	ilog "github.com/shidaxi/iaws/internal/log"
)

// Injected by goreleaser via -ldflags "-X main.version=... -X main.commit=... -X main.date=...".
// Fallback to debug.ReadBuildInfo for `go install github.com/shidaxi/iaws@vX.Y.Z`.
var (
	version = ""
	commit  = ""
	date    = ""
)

func resolveVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

func printVersion() {
	v := resolveVersion()
	fmt.Printf("iaws %s", v)
	if commit != "" {
		fmt.Printf(" (%s)", commit)
	}
	if date != "" {
		fmt.Printf(" built %s", date)
	}
	fmt.Println()
}

func main() {
	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit (shorthand)")
	flag.Parse()
	if showVersion {
		printVersion()
		return
	}

	ilog.Init()
	ilog.Info("iaws starting (version=%s)", resolveVersion())
	ctx := context.Background()
	m := app.New(ctx)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.AttachProgram(p)
	if _, err := p.Run(); err != nil {
		ilog.Error("program exited with error: %v", err)
		os.Exit(1)
	}
	ilog.Info("iaws exited normally")
}
