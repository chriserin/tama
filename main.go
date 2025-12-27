package main

import (
	"fmt"
	"os"

	"tama/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var TamaVersion = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "tama",
	Short: "An interactive Ollama REPL",
	Long:  `Tama is an interactive REPL for chatting with Ollama models.`,
	Run: func(cmd *cobra.Command, args []string) {
		runTUI()
	},
}

func init() {
	rootCmd.Version = TamaVersion
}

func runTUI() {
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

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
