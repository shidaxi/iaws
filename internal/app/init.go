package app

import (
	"github.com/charmbracelet/bubbletea"
)

func (m *model) Init() tea.Cmd {
	return loadProfilesCmd()
}
