package app

import (
	"github.com/charmbracelet/bubbletea"
)

func (m *model) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(loadProfilesCmd(), spinnerTick())
}
