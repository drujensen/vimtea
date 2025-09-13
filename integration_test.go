package vimtea

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditorIntegration(t *testing.T) {
	initialContent := "Hello, world!"
	editor := NewEditor(WithContent(initialContent))

	assert.Equal(t, ModeNormal, editor.GetMode(), "Initial mode should be Normal")

	buffer := editor.GetBuffer()
	assert.Equal(t, initialContent, buffer.Text(), "Buffer content should match initial content")

	editor.SetMode(ModeInsert)
	assert.Equal(t, ModeInsert, editor.GetMode(), "Mode should be Insert after setting")

	model := editor.(*editorModel)
	model.buffer.insertAt(0, 13, " This is a test.")

	expectedContent := "Hello, world! This is a test."
	assert.Equal(t, expectedContent, buffer.Text(), "Buffer content should match expected after insertion")

	editor.SetMode(ModeNormal)
	assert.Equal(t, ModeNormal, editor.GetMode(), "Mode should be Normal after setting")

	testStatusMsg := "Test status"
	cmd := editor.SetStatusMessage(testStatusMsg)
	cmd()

	assert.Equal(t, testStatusMsg, model.statusMessage, "Status message should match set message")
}

func TestViewportIntegration(t *testing.T) {
	var content string
	for i := 0; i < 30; i++ {
		content += "Line " + string(rune('A'+i%26)) + "\n"
	}

	editor := NewEditor(WithContent(content))
	model := editor.(*editorModel)

	model.width = 80
	model.height = 20
	model.viewport.Width = 80
	model.viewport.Height = 20

	model.cursor = newCursor(25, 0)

	model.ensureCursorVisible()

	assert.GreaterOrEqual(t, model.cursor.Row, model.viewport.YOffset,
		"Cursor row should be within or after viewport start")
	assert.Less(t, model.cursor.Row, model.viewport.YOffset+model.viewport.Height,
		"Cursor row should be within viewport end")
}

func TestKeyBindingsIntegration(t *testing.T) {
	editor := NewEditor()
	model := editor.(*editorModel)

	testBindingCalled := false
	editor.AddBinding(KeyBinding{
		Key:         "ctrl+t",
		Mode:        ModeNormal,
		Description: "Test binding",
		Handler: func(b Buffer) tea.Cmd {
			testBindingCalled = true
			return nil
		},
	})

	iBinding := model.registry.FindExact("i", ModeNormal)
	assert.NotNil(t, iBinding, "Default binding for 'i' should exist")

	ctrlTBinding := model.registry.FindExact("ctrl+t", ModeNormal)
	require.NotNil(t, ctrlTBinding, "Custom binding for 'ctrl+t' should exist")

	_ = ctrlTBinding.Command(model)

	assert.True(t, testBindingCalled, "Custom binding command should have been executed")
}

func TestCommandsIntegration(t *testing.T) {
	editor := NewEditor()
	model := editor.(*editorModel)

	commandCalled := false
	editor.AddCommand("test", func(b Buffer, args []string) tea.Cmd {
		commandCalled = true
		if len(args) > 0 && args[0] == "arg" {
			return nil
		}
		return nil
	})

	model.commandBuffer = "test arg"
	model.Update(CommandMsg{Command: "test"})

	assert.True(t, commandCalled, "Command should have been executed")
}

func TestReadOnlyMode(t *testing.T) {
	editor := NewEditor(WithContent("Read-only content"), WithReadOnly(true))
	model := editor.(*editorModel)

	assert.True(t, model.readOnly, "Editor should be in read-only mode")

	// Check that editing bindings are not registered
	iBinding := model.registry.FindExact("i", ModeNormal)
	assert.Nil(t, iBinding, "Insert mode binding 'i' should not exist in read-only mode")

	xBinding := model.registry.FindExact("x", ModeNormal)
	assert.Nil(t, xBinding, "Delete binding 'x' should not exist in read-only mode")

	pBinding := model.registry.FindExact("p", ModeNormal)
	assert.Nil(t, pBinding, "Paste binding 'p' should not exist in read-only mode")

	// Check that navigation and visual bindings still exist
	vBinding := model.registry.FindExact("v", ModeNormal)
	assert.NotNil(t, vBinding, "Visual mode binding 'v' should exist in read-only mode")

	hBinding := model.registry.FindExact("h", ModeNormal)
	assert.NotNil(t, hBinding, "Navigation binding 'h' should exist in read-only mode")

	// Check that yank bindings exist
	yBinding := model.registry.FindExact("yy", ModeNormal)
	assert.NotNil(t, yBinding, "Yank binding 'yy' should exist in read-only mode")

	yiwBinding := model.registry.FindExact("yiw", ModeNormal)
	assert.NotNil(t, yiwBinding, "Yank inner word binding 'yiw' should exist in read-only mode")
}

func TestClearIntegration(t *testing.T) {
	// Create editor with initial content
	initialContent := "Line 1\nLine 2\nLine 3"
	editor := NewEditor(WithContent(initialContent))
	buffer := editor.GetBuffer()

	// Verify initial content
	assert.Equal(t, initialContent, buffer.Text(), "Buffer should have initial content")
	assert.Equal(t, 3, buffer.LineCount(), "Buffer should have 3 lines initially")

	// Set cursor to a non-zero position
	model := editor.(*editorModel)
	model.cursor = newCursor(1, 3)

	// Clear the buffer using the Clear method
	clearCmd := buffer.Clear()
	if clearCmd != nil {
		clearCmd()
	}

	// After clearing, the buffer should have a single empty line and cursor at 0,0
	assert.Equal(t, 1, buffer.LineCount(), "Buffer should have 1 line after clear")
	assert.Equal(t, "", buffer.Text(), "Buffer text should be empty")
	assert.Equal(t, 0, model.cursor.Row, "Cursor row should be reset to 0")
	assert.Equal(t, 0, model.cursor.Col, "Cursor column should be reset to 0")

	// Test undo functionality after clear
	undoCmd := buffer.Undo()
	undoResult := undoCmd().(UndoRedoMsg)

	assert.True(t, undoResult.Success, "Undo after clear should succeed")
	assert.Equal(t, initialContent, buffer.Text(), "Buffer should return to initial content after undo")
}

func TestResetIntegration(t *testing.T) {
	// Create editor with initial content
	initialContent := "Initial content"
	editor := NewEditor(WithContent(initialContent))
	buffer := editor.GetBuffer()

	// Verify initial content
	assert.Equal(t, initialContent, buffer.Text(), "Buffer should have initial content")

	// Make changes to the editor
	model := editor.(*editorModel)
	buffer.InsertAt(0, 0, "Modified ") // Modify the content
	model.cursor = newCursor(0, 9)     // Move cursor after "Modified "

	// Verify the changes were made
	assert.Equal(t, "Modified Initial content", buffer.Text(), "Buffer content should be modified")
	assert.Equal(t, 0, model.cursor.Row, "Cursor row should be 0")
	assert.Equal(t, 9, model.cursor.Col, "Cursor column should be 9")

	// Reset the editor
	resetCmd := editor.Reset()
	if resetCmd != nil {
		resetCmd()
	}

	// Verify the editor has been reset to initial state
	assert.Equal(t, initialContent, buffer.Text(), "Buffer should be reset to initial content")
	assert.Equal(t, 0, model.cursor.Row, "Cursor row should be reset to 0")
	assert.Equal(t, 0, model.cursor.Col, "Cursor column should be reset to 0")
	assert.Equal(t, ModeNormal, model.mode, "Editor mode should be reset to Normal")
	assert.Equal(t, "", model.yankBuffer, "Yank buffer should be empty")

	// Make more changes after reset
	buffer.InsertAt(0, 0, "New ")
	assert.Equal(t, "New Initial content", buffer.Text(), "Buffer should accept changes after reset")

	// Reset again
	resetCmd = editor.Reset()
	if resetCmd != nil {
		resetCmd()
	}

	// Verify reset again
	assert.Equal(t, initialContent, buffer.Text(), "Buffer should be reset to initial content again")
}
