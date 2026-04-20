package main

import (
	"context"
	"os"

	"github.com/charmbracelet/bubbletea"
	"github.com/shidaxi/iaws/internal/app"
	ilog "github.com/shidaxi/iaws/internal/log"
)

func main() {
	ilog.Init()
	ilog.Info("iaws starting")
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
