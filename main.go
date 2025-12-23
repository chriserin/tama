package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"tama/internal/tui"
)

func main() {
	p := tea.NewProgram(
		tui.InitialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
		tea.WithReportFocus(),
	)

	// Set the send function on the model for later use
	go func() {
		p.Send(tui.SetSendFuncMsg{Send: p.Send})
	}()

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
