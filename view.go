// Package vimtea provides a Vim-like text editor component for terminal applications
package vimtea

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// Regular expression for matching ANSI escape sequences
// Used to correctly calculate visible text length with syntax highlighting
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// renderTab renders a tab character with visual representation using spaces
func renderTab(col int) string {
	spaces := tabWidth - (col % tabWidth)
	return strings.Repeat(" ", spaces)
}

// visualLength calculates the visual length of a string, counting tabs as tabWidth spaces
// and ignoring ANSI escape sequences
func visualLength(s string, startCol int) int {
	length := 0
	i := 0
	for i < len(s) {
		// Check if we're at an ANSI escape sequence
		if ansiMatches := ansiRegex.FindStringIndex(s[i:]); ansiMatches != nil && ansiMatches[0] == 0 {
			// Skip the entire ANSI sequence
			i += ansiMatches[1]
			continue
		}

		r := s[i]
		if r == '\t' {
			// Tab advances to the next tab stop
			spaces := tabWidth - ((startCol + length) % tabWidth)
			length += spaces
		} else {
			length++
		}
		i++
	}
	return length
}

// bufferToVisualPosition converts a buffer position to a visual position
// This accounts for tabs that visually occupy multiple columns and ANSI escape sequences
func bufferToVisualPosition(line string, bufferCol int) int {
	if bufferCol > len(line) {
		bufferCol = len(line)
	}

	visualCol := 0
	i := 0
	for i < bufferCol && i < len(line) {
		// Check if we're at an ANSI escape sequence
		if ansiMatches := ansiRegex.FindStringIndex(line[i:]); ansiMatches != nil && ansiMatches[0] == 0 {
			// Skip the entire ANSI sequence
			i += ansiMatches[1]
			continue
		}

		r := line[i]
		if r == '\t' {
			spaces := tabWidth - (visualCol % tabWidth)
			visualCol += spaces
		} else {
			visualCol++
		}
		i++
	}
	return visualCol
}

// renderLineWithTabs renders a line with proper tab expansion
// Preserves ANSI escape sequences
func renderLineWithTabs(line string) string {
	var sb strings.Builder
	visualCol := 0
	i := 0

	for i < len(line) {
		// Check if we're at an ANSI escape sequence
		if ansiMatches := ansiRegex.FindStringIndex(line[i:]); ansiMatches != nil && ansiMatches[0] == 0 {
			// Copy the entire ANSI sequence as-is
			sb.WriteString(line[i : i+ansiMatches[1]])
			i += ansiMatches[1]
			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		if r == '\t' {
			spaces := tabWidth - (visualCol % tabWidth)
			sb.WriteString(strings.Repeat(" ", spaces))
			visualCol += spaces
		} else {
			sb.WriteRune(r)
			visualCol++
		}
		i += size
	}

	return sb.String()
}

// View renders the editor and returns it as a string
// This is part of the bubbletea.Model interface
func (m *editorModel) View() string {
	// Build components from top to bottom
	components := []string{
		m.renderContent(), // Main editor content
	}
	if m.enableStatusBar {
		components = append(components, m.renderStatusLine()) // Status bar and command line
	}

	// Join all components vertically
	return lipgloss.JoinVertical(
		lipgloss.Top,
		components...,
	)
}

func (m *editorModel) renderContent() string {
	var sb strings.Builder

	var selStart, selEnd Cursor
	if m.mode == ModeVisual {
		selStart, selEnd = m.GetSelectionBoundary()
	}

	visibleContent := m.getVisibleContent()

	for i, line := range visibleContent {
		lineNum := i + m.viewport.YOffset + 1
		rowIdx := lineNum - 1

		if m.showLineNumbers {
			sb.WriteString(m.renderLineNumber(lineNum, rowIdx))
		}

		if rowIdx >= m.buffer.lineCount() {
			sb.WriteString("\n")
			continue
		}

		inVisualSelection := m.mode == ModeVisual && rowIdx >= selStart.Row && rowIdx <= selEnd.Row
		sb.WriteString(m.renderLine(line, rowIdx, inVisualSelection, selStart, selEnd))
		sb.WriteString("\n")
	}

	return sb.String()
}

func (m *editorModel) renderLine(line string, rowIdx int, inVisualSelection bool, selStart, selEnd Cursor) string {
	displayLine := renderLineWithTabs(line)

	if m.mode == ModeVisual && m.isVisualLine && inVisualSelection {
		// Strip ANSI characters for clean visual line selection
		cleanLine := ansiRegex.ReplaceAllString(line, "")
		cleanDisplayLine := renderLineWithTabs(cleanLine)
		return m.selectedStyle.Render(cleanDisplayLine)
	}

	if m.mode != ModeVisual && m.yankHighlight.Active && m.isLineInYankHighlight(rowIdx) {
		return m.renderLineWithYankHighlight(line, rowIdx)
	}

	var highlightedLine string
	if m.highlighter != nil && m.highlighter.enabled {
		highlightedLine = m.highlighter.HighlightLine(displayLine)
	} else {
		highlightedLine = displayLine
	}

	if rowIdx == m.cursor.Row {
		if len(line) == 0 {
			if m.cursor.Col == 0 {
				return m.renderCursor(" ")
			}
			return ""
		}

		if m.cursor.Col >= len(line) {
			return highlightedLine + m.renderCursor(" ")
		}

		if m.mode == ModeVisual && !m.isVisualLine && inVisualSelection {
			return m.renderLineWithCursorInVisualSelection(line, rowIdx, selStart, selEnd)
		}

		if m.highlighter != nil && m.highlighter.enabled && displayLine != highlightedLine {
			return m.renderSyntaxHighlightedCursorLine(highlightedLine, line)
		}

		return m.renderRegularCursorLine(line)
	}

	if m.mode == ModeVisual && !m.isVisualLine && inVisualSelection {
		return m.renderLineInVisualSelection(line, rowIdx, selStart, selEnd)
	}

	return highlightedLine
}

func (m *editorModel) renderCursor(char string) string {
	if !m.cursorBlink {
		return char
	}

	switch m.mode {
	case ModeInsert:
		return lipgloss.NewStyle().Underline(true).Render(char)
	case ModeCommand:
		return char
	default:
		return m.cursorStyle.Render(char)
	}
}

func (m *editorModel) renderLineNumber(lineNum int, rowIdx int) string {
	if !m.showLineNumbers {
		return ""
	}

	if rowIdx >= m.buffer.lineCount() {
		return m.lineNumberStyle.Render("    ")
	}

	if rowIdx == m.cursor.Row {
		return m.currentLineNumberStyle.Render(fmt.Sprintf("%4d", lineNum))
	}

	if m.relativeNumbers {
		distance := abs(rowIdx - m.cursor.Row)
		return m.lineNumberStyle.Render(fmt.Sprintf("%4d", distance))
	}

	return m.lineNumberStyle.Render(fmt.Sprintf("%4d", lineNum))
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (m *editorModel) renderRegularCursorLine(line string) string {
	var sb strings.Builder
	visualCol := 0
	bufferIdx := 0

	// Process characters up to the cursor position
	for bufferIdx < m.cursor.Col && bufferIdx < len(line) {
		// Check if we're at an ANSI escape sequence
		if ansiMatches := ansiRegex.FindStringIndex(line[bufferIdx:]); ansiMatches != nil && ansiMatches[0] == 0 {
			// Copy the entire ANSI sequence as-is
			sb.WriteString(line[bufferIdx : bufferIdx+ansiMatches[1]])
			bufferIdx += ansiMatches[1]
			continue
		}

		r, size := utf8.DecodeRuneInString(line[bufferIdx:])
		if r == '\t' {
			spaces := tabWidth - (visualCol % tabWidth)
			sb.WriteString(strings.Repeat(" ", spaces))
			visualCol += spaces
		} else {
			sb.WriteRune(r)
			visualCol++
		}
		bufferIdx += size
	}

	// Handle cursor character
	if m.cursor.Col < len(line) {
		// Check if cursor is on an ANSI escape sequence
		if ansiMatches := ansiRegex.FindStringIndex(line[m.cursor.Col:]); ansiMatches != nil && ansiMatches[0] == 0 {
			// Cursor is on ANSI sequence, skip it and find the next character
			ansiEnd := m.cursor.Col + ansiMatches[1]
			if ansiEnd < len(line) {
				cursorRune, _ := utf8.DecodeRuneInString(line[ansiEnd:])
				if cursorRune == '\t' {
					// For tab, just highlight the first space
					sb.WriteString(m.renderCursor(" "))
					// Write the remaining spaces
					spaces := tabWidth - 1 - (visualCol % tabWidth)
					if spaces > 0 {
						sb.WriteString(strings.Repeat(" ", spaces))
					}
					visualCol += tabWidth - (visualCol % tabWidth)
				} else {
					sb.WriteString(m.renderCursor(string(cursorRune)))
					visualCol++
				}
				bufferIdx = ansiEnd + utf8.RuneLen(cursorRune)
			} else {
				// Cursor at end of line after ANSI
				sb.WriteString(m.renderCursor(" "))
				visualCol++
				bufferIdx = len(line)
			}
		} else {
			cursorRune, size := utf8.DecodeRuneInString(line[m.cursor.Col:])
			if cursorRune == '\t' {
				// For tab, just highlight the first space
				sb.WriteString(m.renderCursor(" "))
				// Write the remaining spaces
				spaces := tabWidth - 1 - (visualCol % tabWidth)
				if spaces > 0 {
					sb.WriteString(strings.Repeat(" ", spaces))
				}
				visualCol += tabWidth - (visualCol % tabWidth)
			} else {
				sb.WriteString(m.renderCursor(string(cursorRune)))
				visualCol++
			}
			bufferIdx = m.cursor.Col + size
		}
	} else {
		// Cursor at end of line
		sb.WriteString(m.renderCursor(" "))
		visualCol++
		bufferIdx = len(line)
	}

	// Process remaining characters after cursor
	for bufferIdx < len(line) {
		// Check if we're at an ANSI escape sequence
		if ansiMatches := ansiRegex.FindStringIndex(line[bufferIdx:]); ansiMatches != nil && ansiMatches[0] == 0 {
			// Copy the entire ANSI sequence as-is
			sb.WriteString(line[bufferIdx : bufferIdx+ansiMatches[1]])
			bufferIdx += ansiMatches[1]
			continue
		}

		r, size := utf8.DecodeRuneInString(line[bufferIdx:])
		if r == '\t' {
			spaces := tabWidth - (visualCol % tabWidth)
			sb.WriteString(strings.Repeat(" ", spaces))
			visualCol += spaces
		} else {
			sb.WriteRune(r)
			visualCol++
		}
		bufferIdx += size
	}

	return sb.String()
}

func (m *editorModel) renderSyntaxHighlightedCursorLine(highlightedLine, plainLine string) string {
	// For syntax highlighting with tabs, we need to:
	// 1. Render the plain line with proper tab expansion
	// 2. Apply cursor highlighting at the correct position

	// If the cursor is at the end, just append it
	if m.cursor.Col >= len(plainLine) {
		return highlightedLine + m.renderCursor(" ")
	}

	// Calculate the visual position of the cursor
	visualCursorPos := bufferToVisualPosition(plainLine, m.cursor.Col)

	// Get the character at the cursor position
	var cursorChar string
	if m.cursor.Col < len(plainLine) {
		if plainLine[m.cursor.Col] == '\t' {
			cursorChar = " " // Show first space of tab
		} else {
			cursorChar = string(plainLine[m.cursor.Col])
		}
	} else {
		cursorChar = " "
	}

	// If we're dealing with a tab at cursor position, we need special handling
	if m.cursor.Col < len(plainLine) && plainLine[m.cursor.Col] == '\t' {
		return m.renderRegularCursorLine(plainLine)
	}

	// For non-tab characters, we can try to locate the cursor position in the highlighted line
	// Find all ANSI escape sequences in the highlighted line
	ansiMatches := ansiRegex.FindAllStringIndex(highlightedLine, -1)

	// Match the visual position in the highlighted line
	visibleIdx := 0
	cursorHighlightPos := -1

	for i := 0; i < len(highlightedLine); {
		isAnsi := false
		for _, match := range ansiMatches {
			if match[0] == i {
				i = match[1]
				isAnsi = true
				break
			}
		}

		if isAnsi {
			continue
		}

		if visibleIdx == visualCursorPos {
			cursorHighlightPos = i
			break
		}

		visibleIdx++
		i++
	}

	// If we couldn't find the cursor position in the highlighted output,
	// fall back to regular cursor line rendering
	if cursorHighlightPos == -1 {
		return m.renderRegularCursorLine(plainLine)
	}

	// Extract ANSI codes that should be active before the cursor
	var ansiBeforeCursor string
	for _, match := range ansiMatches {
		if match[0] < cursorHighlightPos {
			ansiBeforeCursor += highlightedLine[match[0]:match[1]]
		}
	}

	// Build the final output with the cursor properly highlighted
	var sb strings.Builder
	sb.WriteString(highlightedLine[:cursorHighlightPos])
	sb.WriteString("\x1b[0m") // Reset all ANSI formatting

	sb.WriteString(m.renderCursor(cursorChar))

	// Restore ANSI formatting for text after the cursor
	sb.WriteString(ansiBeforeCursor)

	if cursorHighlightPos+1 < len(highlightedLine) {
		afterCursorStart := cursorHighlightPos + 1

		// Skip any ANSI sequences immediately after the cursor
		for _, match := range ansiMatches {
			if afterCursorStart >= match[0] && afterCursorStart < match[1] {
				afterCursorStart = match[1]
				break
			}
		}

		sb.WriteString(highlightedLine[afterCursorStart:])
	}

	return sb.String()
}

func (m *editorModel) renderLineWithCursorInVisualSelection(line string, rowIdx int, selStart, selEnd Cursor) string {
	// In visual mode, strip ANSI characters to avoid conflicts and show clean selection
	// This matches what gets yanked (no ANSI characters)
	cleanLine := ansiRegex.ReplaceAllString(line, "")

	// Use plain rendering with clean text
	return m.renderLineWithCursorInVisualSelectionPlain(cleanLine, rowIdx, selStart, selEnd)
}

func (m *editorModel) renderLineWithCursorInVisualSelectionHighlighted(highlightedLine string, plainLine string, rowIdx int, selStart Cursor, selEnd Cursor) string {
	// Get selection boundaries in buffer coordinates
	selBegin := 0
	if rowIdx == selStart.Row {
		selBegin = selStart.Col
	}

	selEndCol := len(plainLine)
	if rowIdx == selEnd.Row {
		selEndCol = selEnd.Col + 1
	}

	// Convert buffer positions to visual positions for highlighted text
	visSelBegin := bufferToVisualPosition(plainLine, selBegin)
	visSelEnd := bufferToVisualPosition(plainLine, selEndCol)
	visCursorPos := bufferToVisualPosition(plainLine, m.cursor.Col)

	// Extract ANSI escape sequences from highlighted text
	ansiMatches := ansiRegex.FindAllStringIndex(highlightedLine, -1)

	// Build the result with proper selection highlighting
	var sb strings.Builder
	visPos := 0
	inSelection := false

	for i := 0; i < len(highlightedLine); {
		// Check if we're at an ANSI sequence
		isAnsi := false
		for _, match := range ansiMatches {
			if match[0] == i {
				// Copy ANSI sequence as-is
				sb.WriteString(highlightedLine[match[0]:match[1]])
				i = match[1]
				isAnsi = true
				break
			}
		}

		if isAnsi {
			continue
		}

		// Update selection state
		if visPos == visSelBegin {
			inSelection = true
		}
		if visPos == visSelEnd {
			inSelection = false
		}

		char := highlightedLine[i]

		// Handle cursor position
		if visPos == visCursorPos {
			if inSelection {
				// Cursor in selection - use selected style
				sb.WriteString("\x1b[0m") // Reset any existing styles
				sb.WriteString(m.selectedStyle.Render(string(char)))
			} else {
				// Cursor not in selection - use cursor style
				sb.WriteString("\x1b[0m") // Reset any existing styles
				sb.WriteString(m.renderCursor(string(char)))
			}
		} else if inSelection {
			// Character in selection but not cursor
			sb.WriteString("\x1b[0m") // Reset any existing styles
			sb.WriteString(m.selectedStyle.Render(string(char)))
		} else {
			// Regular character with syntax highlighting
			sb.WriteString(string(char))
		}

		visPos++
		i++
	}

	return sb.String()
}

// renderLineWithCursorInVisualSelectionPlain handles rendering a line with a cursor in visual selection
// when no syntax highlighting is applied.
func (m *editorModel) renderLineWithCursorInVisualSelectionPlain(line string, rowIdx int, selStart, selEnd Cursor) string {
	var sb strings.Builder

	// Get selection boundaries in buffer coordinates
	selBegin := 0
	if rowIdx == selStart.Row {
		selBegin = selStart.Col
	}

	selEndCol := len(line)
	if rowIdx == selEnd.Row {
		selEndCol = selEnd.Col + 1
	}

	// Process the line with proper tab rendering
	curVisualPos := 0
	for i, r := range line {
		// Handle character before selection start
		if i < selBegin {
			if r == '\t' {
				spaces := tabWidth - (curVisualPos % tabWidth)
				sb.WriteString(strings.Repeat(" ", spaces))
				curVisualPos += spaces
			} else {
				sb.WriteRune(r)
				curVisualPos++
			}
			continue
		}

		// Handle cursor character
		if i == m.cursor.Col {
			// Get appropriate character display
			var cursorChar string
			if r == '\t' {
				cursorChar = " " // Show first space of tab
			} else {
				cursorChar = string(r)
			}

			if m.cursorBlink {
				sb.WriteString(m.cursorStyle.Render(cursorChar))
			} else {
				sb.WriteString(m.selectedStyle.Render(cursorChar))
			}

			// Handle remaining spaces for tab
			if r == '\t' {
				spaces := tabWidth - 1 - (curVisualPos % tabWidth)
				if spaces > 0 {
					sb.WriteString(m.selectedStyle.Render(strings.Repeat(" ", spaces)))
				}
				curVisualPos += tabWidth - (curVisualPos % tabWidth)
			} else {
				curVisualPos++
			}
			continue
		}

		// Handle selection (non-cursor)
		if i < selEndCol {
			if r == '\t' {
				spaces := tabWidth - (curVisualPos % tabWidth)
				sb.WriteString(m.selectedStyle.Render(strings.Repeat(" ", spaces)))
				curVisualPos += spaces
			} else {
				sb.WriteString(m.selectedStyle.Render(string(r)))
				curVisualPos++
			}
			continue
		}

		// Handle character after selection end
		if r == '\t' {
			spaces := tabWidth - (curVisualPos % tabWidth)
			sb.WriteString(strings.Repeat(" ", spaces))
			curVisualPos += spaces
		} else {
			sb.WriteRune(r)
			curVisualPos++
		}
	}

	return sb.String()
}

func (m *editorModel) renderLineInVisualSelection(line string, rowIdx int, selStart, selEnd Cursor) string {
	// In visual mode, strip ANSI characters to avoid conflicts and show clean selection
	// This matches what gets yanked (no ANSI characters)
	cleanLine := ansiRegex.ReplaceAllString(line, "")

	// Use plain rendering with clean text
	return m.renderLineInVisualSelectionPlain(cleanLine, rowIdx, selStart, selEnd)
}

// renderLineInVisualSelectionPlain handles rendering a line in visual selection
// when no syntax highlighting is applied.
func (m *editorModel) renderLineInVisualSelectionPlain(line string, rowIdx int, selStart, selEnd Cursor) string {
	var sb strings.Builder

	// Get selection boundaries in buffer coordinates
	selBegin := 0
	if rowIdx == selStart.Row {
		selBegin = selStart.Col
	}

	selEndCol := len(line)
	if rowIdx == selEnd.Row {
		selEndCol = selEnd.Col + 1
	}

	// Process the line with proper tab rendering
	curVisualPos := 0
	for i, r := range line {
		// Handle character before selection start
		if i < selBegin {
			if r == '\t' {
				spaces := tabWidth - (curVisualPos % tabWidth)
				sb.WriteString(strings.Repeat(" ", spaces))
				curVisualPos += spaces
			} else {
				sb.WriteRune(r)
				curVisualPos++
			}
			continue
		}

		// Handle selection
		if i < selEndCol {
			if r == '\t' {
				spaces := tabWidth - (curVisualPos % tabWidth)
				sb.WriteString(m.selectedStyle.Render(strings.Repeat(" ", spaces)))
				curVisualPos += spaces
			} else {
				sb.WriteString(m.selectedStyle.Render(string(r)))
				curVisualPos++
			}
			continue
		}

		// Handle character after selection end
		if r == '\t' {
			spaces := tabWidth - (curVisualPos % tabWidth)
			sb.WriteString(strings.Repeat(" ", spaces))
			curVisualPos += spaces
		} else {
			sb.WriteRune(r)
			curVisualPos++
		}
	}

	return sb.String()
}

func (m editorModel) getVisibleContent() []string {
	// Calculate the usable width for content (accounting for line numbers)
	usableWidth := m.width
	if m.showLineNumbers {
		usableWidth -= 4 // Line numbers take 4 characters
	}

	// Calculate the usable height for content
	usableHeight := m.height

	// Adjust Y-axis for scrolling and visibility
	startLine := m.viewport.YOffset

	// Calculate how many buffer lines to show based on available height
	estimatedLines := usableHeight

	// If we have a valid usable width, calculate exactly how many buffer lines fit
	if usableWidth > 0 {
		estimatedLines = m.calculateBufferLinesForHeight(usableHeight, startLine, usableWidth)
	}

	endLine := startLine + estimatedLines

	// Ensure that we do not go out of bounds
	if startLine < 0 {
		startLine = 0
	}

	contentLines := []string{}

	for i := startLine; i < min(endLine, m.buffer.lineCount()); i++ {
		contentLines = append(contentLines, m.buffer.Line(i))
	}

	// Fill in any remaining lines if necessary (only up to usableHeight)
	emptyLinesNeeded := estimatedLines - len(contentLines)
	if emptyLinesNeeded > 0 {
		for i := 0; i < emptyLinesNeeded; i++ {
			contentLines = append(contentLines, "")
		}
	}

	return contentLines
}

// calculateBufferLinesForHeight calculates how many buffer lines can fit in the given height
// accounting for line wrapping. usableWidth is the width available for content (excluding line numbers)
func (m editorModel) calculateBufferLinesForHeight(height, startLine, usableWidth int) int {
	if height <= 0 {
		return 0
	}

	totalLines := m.buffer.lineCount()
	if totalLines == 0 {
		return height
	}

	bufferLines := 0
	visualLinesUsed := 0

	for i := startLine; i < totalLines && visualLinesUsed < height; i++ {
		line := m.buffer.Line(i)
		// Remove ANSI escape codes for length calculation
		cleanLine := ansiRegex.ReplaceAllString(line, "")
		lineVisualLength := visualLength(cleanLine, 0)

		// Calculate how many visual lines this buffer line takes
		if lineVisualLength == 0 {
			// Empty line takes 1 visual line
			visualLinesUsed++
		} else {
			// Calculate wrapped lines: ceiling division of visual length by usableWidth
			wrappedLines := (lineVisualLength + usableWidth - 1) / usableWidth
			visualLinesUsed += wrappedLines
		}

		// If we've exceeded the height, stop without counting this line
		if visualLinesUsed > height {
			break
		}

		bufferLines++
	}

	return bufferLines
}

func (m *editorModel) renderStatusLine() string {
	status := m.getStatusText()
	cursorPos := fmt.Sprintf(" %d:%d ", m.cursor.Row+1, m.cursor.Col+1)

	padding := max(m.width-lipgloss.Width(status)-lipgloss.Width(cursorPos), 0)

	return m.statusStyle.Render(status + strings.Repeat(" ", padding) + cursorPos)
}

func (m *editorModel) getStatusText() string {
	if m.mode == ModeCommand {
		return ":" + m.commandBuffer
	}

	status := fmt.Sprintf(" %s", m.mode)
	if len(m.keySequence) > 0 {
		status += fmt.Sprintf(" | %s", strings.Join(m.keySequence, ""))
	}

	if m.statusMessage != "" {
		status += fmt.Sprintf(" | %s", m.statusMessage)
	}

	return status
}

func (m *editorModel) isLineInYankHighlight(rowIdx int) bool {
	return m.yankHighlight.Active &&
		rowIdx >= m.yankHighlight.Start.Row && rowIdx <= m.yankHighlight.End.Row
}

func (m *editorModel) getYankHighlightBounds(rowIdx int) (int, int) {
	if !m.yankHighlight.Active || !m.isLineInYankHighlight(rowIdx) {
		return -1, -1
	}

	start := 0
	end := m.buffer.lineLength(rowIdx)

	if !m.yankHighlight.IsLinewise {
		if rowIdx == m.yankHighlight.Start.Row {
			start = m.yankHighlight.Start.Col
		}

		if rowIdx == m.yankHighlight.End.Row {
			end = m.yankHighlight.End.Col + 1
		}
	}

	return start, end
}

func (m *editorModel) renderLineWithYankHighlight(line string, rowIdx int) string {
	var sb strings.Builder
	// Use a more readable highlight color (dark blue background)
	highlightStyle := lipgloss.NewStyle().Background(lipgloss.Color("4")).Foreground(lipgloss.Color("15"))

	start, end := m.getYankHighlightBounds(rowIdx)
	if start < 0 || end < 0 {
		// Strip ANSI characters for clean display
		cleanLine := ansiRegex.ReplaceAllString(line, "")
		return renderLineWithTabs(cleanLine)
	}

	// Strip ANSI characters for consistent highlighting
	cleanLine := ansiRegex.ReplaceAllString(line, "")
	start = max(0, min(start, len(cleanLine)))
	end = max(0, min(end, len(cleanLine)))

	// Process the clean line with proper tab rendering
	curVisualPos := 0
	i := 0
	for i < len(cleanLine) {
		r, size := utf8.DecodeRuneInString(cleanLine[i:])

		// Handle character before highlight start
		if i < start {
			if r == '\t' {
				spaces := tabWidth - (curVisualPos % tabWidth)
				sb.WriteString(strings.Repeat(" ", spaces))
				curVisualPos += spaces
			} else {
				sb.WriteRune(r)
				curVisualPos++
			}
			i += size
			continue
		}

		// Handle cursor character within highlight
		if i == m.cursor.Col && rowIdx == m.cursor.Row && i >= start && i < end {
			// Get appropriate character display
			var cursorChar string
			if r == '\t' {
				cursorChar = " " // Show first space of tab
			} else {
				cursorChar = string(r)
			}

			if m.cursorBlink {
				sb.WriteString(m.cursorStyle.Render(cursorChar))
			} else {
				sb.WriteString(highlightStyle.Render(cursorChar))
			}

			// Handle remaining spaces for tab
			if r == '\t' {
				spaces := tabWidth - 1 - (curVisualPos % tabWidth)
				if spaces > 0 {
					sb.WriteString(highlightStyle.Render(strings.Repeat(" ", spaces)))
				}
				curVisualPos += tabWidth - (curVisualPos % tabWidth)
			} else {
				curVisualPos++
			}
			i += size
			continue
		}

		// Handle highlighted character (non-cursor)
		if i < end {
			if r == '\t' {
				spaces := tabWidth - (curVisualPos % tabWidth)
				sb.WriteString(highlightStyle.Render(strings.Repeat(" ", spaces)))
				curVisualPos += spaces
			} else {
				sb.WriteString(highlightStyle.Render(string(r)))
				curVisualPos++
			}
			i += size
			continue
		}

		// Handle character after highlight end
		if r == '\t' {
			spaces := tabWidth - (curVisualPos % tabWidth)
			sb.WriteString(strings.Repeat(" ", spaces))
			curVisualPos += spaces
		} else {
			sb.WriteRune(r)
			curVisualPos++
		}
		i += size
	}

	return sb.String()
}
