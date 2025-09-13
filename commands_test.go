package vimtea

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.design/x/clipboard"
)

func TestCommandExecution(t *testing.T) {
	editor := NewEditor(WithContent("Line 1\nLine 2\nLine 3"))
	model := editor.(*editorModel)

	// Test yank line command (find binding in registry)
	binding := model.registry.FindExact("yy", ModeNormal)
	require.NotNil(t, binding, "Binding for 'yy' not found")

	model.cursor = newCursor(1, 0)
	binding.Command(model)

	assert.Contains(t, model.yankBuffer, "Line 2", "yankLine should set yankBuffer to contain 'Line 2'")

	// Test delete line command
	deleteBinding := model.registry.FindExact("dd", ModeNormal)
	require.NotNil(t, deleteBinding, "Binding for 'dd' not found")

	model.cursor = newCursor(1, 0)
	deleteBinding.Command(model)

	assert.Equal(t, 2, model.buffer.lineCount(), "deleteLine should remove a line")
	assert.Equal(t, "Line 3", model.buffer.Line(1), "After deletion, line 1 should be 'Line 3'")
}

func TestPasteCommands(t *testing.T) {
	err := clipboard.Init()
	if err != nil {
		t.Skip("Clipboard not available, skipping test")
	}

	editor := NewEditor(WithContent("Line 1\nLine 2\nLine 3"))
	model := editor.(*editorModel)

	// Set up yankBuffer
	model.yankBuffer = "Yanked content"
	clipboard.Write(clipboard.FmtText, []byte("Yanked content"))

	// Test paste after command
	pasteAfterBinding := model.registry.FindExact("p", ModeNormal)
	require.NotNil(t, pasteAfterBinding, "Binding for 'p' not found")

	model.cursor = newCursor(0, 5)
	pasteAfterBinding.Command(model)

	expectedContent := "Line 1Yanked content\nLine 2\nLine 3"
	assert.Equal(t, expectedContent, model.buffer.text(), "pasteAfter should insert at cursor position")

	// Test paste before command
	pasteBeforeBinding := model.registry.FindExact("P", ModeNormal)
	require.NotNil(t, pasteBeforeBinding, "Binding for 'P' not found")

	// Reset buffer
	model.buffer.lines = []string{"Line 1", "Line 2", "Line 3"}
	model.cursor = newCursor(0, 5)
	pasteBeforeBinding.Command(model)

	expectedContent = "Line Yanked content1\nLine 2\nLine 3"
	assert.Equal(t, expectedContent, model.buffer.text(), "pasteBefore should insert at cursor position")

	// Test line-wise paste
	// Reset buffer
	model.buffer.lines = []string{"Line 1", "Line 2", "Line 3"}
	model.yankBuffer = "\nYanked line"
	clipboard.Write(clipboard.FmtText, []byte("\nYanked line"))
	model.cursor = newCursor(1, 0)
	pasteAfterBinding.Command(model)

	expectedContent = "Line 1\nLine 2\nYanked line\nLine 3"
	assert.Equal(t, expectedContent, model.buffer.text(), "pasteAfter with line-wise content should insert as new line")
}

func TestInsertModeCommands(t *testing.T) {
	editor := NewEditor(WithContent("Line 1\nLine 2"))
	model := editor.(*editorModel)

	// Test insert at beginning of line (I command)
	insertStartBinding := model.registry.FindExact("I", ModeNormal)
	require.NotNil(t, insertStartBinding, "Binding for 'I' not found")

	model.cursor = newCursor(1, 2)
	insertStartBinding.Command(model)

	assert.Equal(t, 0, model.cursor.Col, "I command should move cursor to col 0")
	assert.Equal(t, ModeInsert, model.mode, "I command should switch to insert mode")

	// Test insert at end of line (A command)
	appendEndBinding := model.registry.FindExact("A", ModeNormal)
	require.NotNil(t, appendEndBinding, "Binding for 'A' not found")

	model.mode = ModeNormal
	model.cursor = newCursor(1, 2)
	appendEndBinding.Command(model)

	assert.Equal(t, 6, model.cursor.Col, "A command should move cursor to end of line")
	assert.Equal(t, ModeInsert, model.mode, "A command should switch to insert mode")

	// Test insert new line below (o command)
	openBelowBinding := model.registry.FindExact("o", ModeNormal)
	require.NotNil(t, openBelowBinding, "Binding for 'o' not found")

	model.mode = ModeNormal
	model.cursor = newCursor(0, 0)
	openBelowBinding.Command(model)

	assert.Equal(t, 3, model.buffer.lineCount(), "o command should add a new line")
	assert.Equal(t, 1, model.cursor.Row, "o command should position cursor at new line row")
	assert.Equal(t, 0, model.cursor.Col, "o command should position cursor at start of new line")
}

func TestCursorMovementCommands(t *testing.T) {
	editor := NewEditor(WithContent("Line 1\nLine 2\nLine 3"))
	model := editor.(*editorModel)

	// Test move down (j)
	downBinding := model.registry.FindExact("j", ModeNormal)
	require.NotNil(t, downBinding, "Binding for 'j' not found")

	model.cursor = newCursor(0, 0)
	downBinding.Command(model)

	assert.Equal(t, 1, model.cursor.Row, "j command should increase row by 1")

	// Test move up (k)
	upBinding := model.registry.FindExact("k", ModeNormal)
	require.NotNil(t, upBinding, "Binding for 'k' not found")

	upBinding.Command(model)

	assert.Equal(t, 0, model.cursor.Row, "k command should decrease row by 1")

	// Test move right (l)
	rightBinding := model.registry.FindExact("l", ModeNormal)
	require.NotNil(t, rightBinding, "Binding for 'l' not found")

	rightBinding.Command(model)

	assert.Equal(t, 1, model.cursor.Col, "l command should increase col by 1")

	// Test move left (h)
	leftBinding := model.registry.FindExact("h", ModeNormal)
	require.NotNil(t, leftBinding, "Binding for 'h' not found")

	leftBinding.Command(model)

	assert.Equal(t, 0, model.cursor.Col, "h command should decrease col by 1")
}

func TestWrappedMovementCommands(t *testing.T) {
	editor := NewEditor(WithContent("Line 1\nLine 2\nLine 3"))
	model := editor.(*editorModel)

	// Test move to beginning of line (0)
	startBinding := model.registry.FindExact("0", ModeNormal)
	require.NotNil(t, startBinding, "Binding for '0' not found")

	model.cursor = newCursor(1, 3)
	startBinding.Command(model)

	assert.Equal(t, 0, model.cursor.Col, "0 command should set col to 0")

	// Test move to end of line ($)
	endBinding := model.registry.FindExact("$", ModeNormal)
	require.NotNil(t, endBinding, "Binding for '$' not found")

	endBinding.Command(model)

	assert.Equal(t, 5, model.cursor.Col, "$ command should move to end of line")

	// Test space (advance cursor)
	spaceBinding := model.registry.FindExact(" ", ModeNormal)
	require.NotNil(t, spaceBinding, "Binding for space not found")

	// Position cursor at second-to-last position of first line
	model.cursor = newCursor(0, 4)
	spaceBinding.Command(model)

	// Should move to last column
	assert.Equal(t, 5, model.cursor.Col, "Space should move right by 1")

	// One more space should wrap to next line
	spaceBinding.Command(model)

	assert.Equal(t, 1, model.cursor.Row, "Space at end of line should wrap to next line (row)")
	assert.Equal(t, 0, model.cursor.Col, "Space at end of line should wrap to col 0")
}

func TestJumpCommands(t *testing.T) {
	editor := NewEditor(WithContent("Line 1\nLine 2\nLine 3\nLine 4\nLine 5"))
	model := editor.(*editorModel)

	// Test move to first line (gg)
	startDocBinding := model.registry.FindExact("gg", ModeNormal)
	require.NotNil(t, startDocBinding, "Binding for 'gg' not found")

	model.cursor = newCursor(3, 0)
	startDocBinding.Command(model)

	assert.Equal(t, 0, model.cursor.Row, "gg command should set row to 0")

	// Test move to last line (G)
	endDocBinding := model.registry.FindExact("G", ModeNormal)
	require.NotNil(t, endDocBinding, "Binding for 'G' not found")

	endDocBinding.Command(model)

	assert.Equal(t, 4, model.cursor.Row, "G command should move to last line (4)")
}

func TestCommandLineCommands(t *testing.T) {
	editor := NewEditor()
	model := editor.(*editorModel)

	// Register test command
	cmdExecuted := false
	model.commands.Register("test", func(m *editorModel) tea.Cmd {
		cmdExecuted = true
		return nil
	})

	// Set up command mode
	model.mode = ModeCommand
	model.commandBuffer = "test"

	// Get the execute command binding
	execBinding := model.registry.FindExact("enter", ModeCommand)
	require.NotNil(t, execBinding, "Binding for 'enter' in command mode not found")

	cmd := execBinding.Command(model)
	model.Update(cmd())

	assert.True(t, cmdExecuted, "Command execution should run registered command")
	assert.Equal(t, ModeNormal, model.mode, "After command execution, mode should be Normal")
}
