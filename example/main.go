// Example application demonstrating the use of vimtea
// This opens itself and provides a Vim-like interface to edit the file
package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kujtimiihoxha/vimtea"
)

func main() {
	// Create a log file
	logFile, err := os.OpenFile("debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	// Set log output to the file
	log.SetOutput(logFile)

	file, err := os.Open("example/main.go")
	if err != nil {
		log.Fatalf("Failed to open example/main.go: %v", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		log.Fatalf("Failed to get file stat: %v", err)
	}
	// Read the file
	buf := make([]byte, stat.Size())
	_, err = file.Read(buf)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Create a new editor with the file contents
	// WithFileName is used for syntax highlighting
	editor := vimtea.NewEditor(
		vimtea.WithContent(string(buf)),
		vimtea.WithFileName("example/main.go"),
		vimtea.WithFullScreen(),
	)

	// Add a custom key binding for quitting with Ctrl+C
	editor.AddBinding(vimtea.KeyBinding{
		Key:         "ctrl+c",
		Mode:        vimtea.ModeNormal,
		Description: "Close the editor",
		Handler: func(b vimtea.Buffer) tea.Cmd {
			return tea.Quit
		},
	})

	// Add a custom command that can be invoked with :q
	editor.AddCommand("q", func(b vimtea.Buffer, _ []string) tea.Cmd {
		return tea.Quit
	})

	p := tea.NewProgram(editor, tea.WithAltScreen(), tea.WithMouseAllMotion())
	if _, err := p.Run(); err != nil {
		log.Printf("Error running program: %v", err)
	}
}
