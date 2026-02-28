// Package vimtea provides a Vim-like text editor component for terminal applications
package vimtea

// Cursor represents a position in the text buffer with row and column coordinates
type Cursor struct {
	Row int // Zero-based line index
	Col int // Zero-based column index
}

// Clone creates a copy of the cursor
func (c Cursor) Clone() Cursor {
	return Cursor{Row: c.Row, Col: c.Col}
}

// newCursor creates a new cursor at the specified position
func newCursor(row, col int) Cursor {
	return Cursor{Row: row, Col: col}
}

// ensureCursorVisible scrolls the viewport to make sure the cursor is visible
// This is called whenever the cursor moves or the window is resized
// Note: This assumes 1 buffer line = 1 visual line. For wrapped lines, the viewport
// may need to scroll more aggressively to keep the cursor visible.
func (m *editorModel) ensureCursorVisible() {
	// Get usable width for calculating wrapped lines
	usableWidth := m.width
	if m.showLineNumbers {
		usableWidth -= 4 // Line numbers take 4 characters
	}

	// Handle edge case where usableWidth might be invalid
	if usableWidth <= 0 {
		usableWidth = 1
	}

	// Calculate the visual position of the cursor (accounting for wrapped lines above it)
	cursorVisualRow := 0
	for i := 0; i < m.cursor.Row && i < m.buffer.lineCount(); i++ {
		line := m.buffer.Line(i)
		cleanLine := ansiRegex.ReplaceAllString(line, "")
		lineVisualLength := visualLength(cleanLine, 0)

		if lineVisualLength == 0 {
			cursorVisualRow++
		} else {
			wrappedLines := (lineVisualLength + usableWidth - 1) / usableWidth
			cursorVisualRow += wrappedLines
		}
	}

	// Calculate cursor's visual row within its own line (accounting for wrapping)
	if m.cursor.Row < m.buffer.lineCount() {
		line := m.buffer.Line(m.cursor.Row)
		cleanLine := ansiRegex.ReplaceAllString(line, "")
		// Calculate visual column position within the line
		visualCol := bufferToVisualPosition(cleanLine, m.cursor.Col)
		// Add the wrapped lines within the current line up to the cursor position
		cursorVisualRow += visualCol / usableWidth
	}

	// If cursor is above the viewport, scroll up
	if cursorVisualRow < m.viewport.YOffset {
		m.viewport.YOffset = cursorVisualRow
	} else if cursorVisualRow >= m.viewport.YOffset+m.height {
		// If cursor is below the viewport, scroll down
		// Ensure we don't scroll too far
		maxOffset := cursorVisualRow - m.height + 1
		if maxOffset < 0 {
			maxOffset = 0
		}
		m.viewport.YOffset = maxOffset
	}

	// Ensure cursor is within valid bounds
	m.adjustCursorPosition()
}

// adjustCursorPosition ensures the cursor stays within valid bounds
// Has different behavior based on the current mode (Insert vs Normal/Visual)
func (m *editorModel) adjustCursorPosition() {
	// Keep cursor within valid rows
	if m.cursor.Row < 0 {
		m.cursor.Row = 0
	}
	if m.cursor.Row >= m.buffer.lineCount() {
		m.cursor.Row = m.buffer.lineCount() - 1
	}

	// Adjust column position based on mode
	lineLen := m.buffer.lineLength(m.cursor.Row)
	if m.mode == ModeInsert {
		// In insert mode, cursor can be at end of line
		if m.cursor.Col > lineLen {
			m.cursor.Col = lineLen
		}
	} else {
		// In normal/visual mode, cursor can't be at end of line (except empty lines)
		if lineLen == 0 {
			m.cursor.Col = 0
		} else if m.cursor.Col >= lineLen {
			m.cursor.Col = lineLen - 1
		}
	}

	// Keep cursor within valid columns
	if m.cursor.Col < 0 {
		m.cursor.Col = 0
	}
}
