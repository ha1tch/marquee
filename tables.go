package marquee

import (
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// TableRenderHandler handles table, thead, tbody, tr, th, td elements
type TableRenderHandler struct{}

func (h *TableRenderHandler) CanRender(node HTMLNode) bool {
	return node.Tag == "table" || node.Tag == "thead" || node.Tag == "tbody" || 
		   node.Tag == "tr" || node.Tag == "th" || node.Tag == "td"
}

func (h *TableRenderHandler) Render(node HTMLNode, ctx RenderContext) RenderResult {
	switch node.Tag {
	case "table":
		return h.renderTable(node, ctx)
	case "thead":
		// thead is handled by table - shouldn't render independently
		return RenderResult{NextY: ctx.Y}
	case "tbody":
		// tbody is handled by table - shouldn't render independently  
		return RenderResult{NextY: ctx.Y}
	case "tr":
		// tr is handled by table - shouldn't render independently
		return RenderResult{NextY: ctx.Y}
	case "th", "td":
		// cells are handled by table - shouldn't render independently
		return RenderResult{NextY: ctx.Y}
	default:
		return RenderResult{NextY: ctx.Y}
	}
}

// Table represents a complete table structure after parsing
type Table struct {
	Rows         []TableRow
	ColumnWidths []float32
	RowHeights   []float32
	TotalWidth   float32
	TotalHeight  float32
	ColumnCount  int
}

// TableRow represents a single table row
type TableRow struct {
	Cells     []TableCell
	IsHeader  bool
	Height    float32
}

// TableCell represents a single table cell
type TableCell struct {
	Content     []HTMLNode  // Cell content (can be any HTML)
	ColSpan     int         // Future: colspan support
	RowSpan     int         // Future: rowspan support
	IsHeader    bool        // th vs td
	Width       float32     // Calculated width
	Height      float32     // Calculated height
	MinWidth    float32     // Minimum required width
	PrefWidth   float32     // Preferred width
}

// CellContentResult represents rendered cell content
type CellContentResult struct {
	Height    float32
	LinkAreas []LinkArea
}

func (h *TableRenderHandler) renderTable(node HTMLNode, ctx RenderContext) RenderResult {
	// Phase 1: Parse table structure
	table := h.parseTableStructure(node)
	if len(table.Rows) == 0 {
		return RenderResult{NextY: ctx.Y}
	}

	// Phase 2: Measure content and calculate dimensions
	h.measureTableContent(&table, ctx)
	h.calculateColumnWidths(&table, ctx)
	h.calculateRowHeights(&table, ctx)

	// Phase 3: Render the table
	return h.renderTableContent(&table, ctx)
}

// Phase 1: Parse table structure into our data model
func (h *TableRenderHandler) parseTableStructure(node HTMLNode) Table {
	table := Table{
		Rows:        make([]TableRow, 0),
		ColumnCount: 0,
	}

	// Process thead, tbody, or direct tr children
	for _, child := range node.Children {
		switch child.Tag {
		case "thead":
			rows := h.parseTableSection(child, true)
			table.Rows = append(table.Rows, rows...)
		case "tbody":
			rows := h.parseTableSection(child, false)
			table.Rows = append(table.Rows, rows...)
		case "tr":
			row := h.parseTableRow(child, false)
			table.Rows = append(table.Rows, row)
		}
	}

	// Calculate maximum column count
	for _, row := range table.Rows {
		if len(row.Cells) > table.ColumnCount {
			table.ColumnCount = len(row.Cells)
		}
	}

	// Initialize column tracking arrays
	table.ColumnWidths = make([]float32, table.ColumnCount)
	table.RowHeights = make([]float32, len(table.Rows))

	return table
}

func (h *TableRenderHandler) parseTableSection(section HTMLNode, isHeader bool) []TableRow {
	var rows []TableRow
	for _, child := range section.Children {
		if child.Tag == "tr" {
			row := h.parseTableRow(child, isHeader)
			rows = append(rows, row)
		}
	}
	return rows
}

func (h *TableRenderHandler) parseTableRow(tr HTMLNode, isHeader bool) TableRow {
	row := TableRow{
		Cells:    make([]TableCell, 0),
		IsHeader: isHeader,
	}

	for _, child := range tr.Children {
		if child.Tag == "th" || child.Tag == "td" {
			cell := TableCell{
				Content:   []HTMLNode{child}, // Wrap in slice for future nested content support
				ColSpan:   1,                 // Future: parse colspan attribute
				RowSpan:   1,                 // Future: parse rowspan attribute  
				IsHeader:  child.Tag == "th" || isHeader,
			}
			row.Cells = append(row.Cells, cell)
		}
	}

	return row
}

// Phase 2a: Measure content in each cell to determine size requirements
func (h *TableRenderHandler) measureTableContent(table *Table, ctx RenderContext) {
	for rowIdx := range table.Rows {
		row := &table.Rows[rowIdx]
		for cellIdx := range row.Cells {
			cell := &row.Cells[cellIdx]
			h.measureCellContent(cell, ctx)
		}
	}
}

func (h *TableRenderHandler) measureCellContent(cell *TableCell, ctx RenderContext) {
	// For now, we'll measure simple text content
	// Future: this should handle any HTML content within cells
	
	if len(cell.Content) == 0 {
		cell.MinWidth = 50    // Minimum cell width
		cell.PrefWidth = 100  // Preferred cell width
		return
	}

	// Extract text content for measurement
	text := h.extractCellText(cell.Content[0])
	if text == "" {
		cell.MinWidth = 50
		cell.PrefWidth = 100
		return
	}

	font := ctx.Widget.Fonts.Regular
	if cell.IsHeader {
		font = ctx.Widget.Fonts.Bold
	}

	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	// Measure text dimensions
	textSize := ctx.Widget.measureText(font, text, fontSize)
	
	// Add padding
	padding := float32(12)
	cell.MinWidth = textSize.X + 2*padding
	cell.PrefWidth = cell.MinWidth
	
	// For long text, allow wrapping by setting a reasonable preferred width
	if len(text) > 20 {
		words := strings.Fields(text)
		if len(words) > 3 {
			// Estimate width for ~3 words per line
			avgWordWidth := textSize.X / float32(len(words))
			cell.PrefWidth = avgWordWidth * 3 + 2*padding
			if cell.PrefWidth < cell.MinWidth {
				cell.PrefWidth = cell.MinWidth
			}
		}
	}
}

func (h *TableRenderHandler) extractCellText(node HTMLNode) string {
	var text strings.Builder
	
	// Handle text nodes
	if node.Type == NodeTypeText {
		return node.Content
	}
	
	// Handle element nodes - extract all text content
	for _, child := range node.Children {
		if child.Type == NodeTypeText {
			text.WriteString(child.Content)
		} else {
			text.WriteString(h.extractCellText(child))
		}
	}
	
	return text.String()
}

// Phase 2b: Calculate column widths based on available space and content
func (h *TableRenderHandler) calculateColumnWidths(table *Table, ctx RenderContext) {
	if table.ColumnCount == 0 {
		return
	}

	// Calculate available width (account for margins and borders)
	borderWidth := float32(1)
	totalBorderWidth := float32(table.ColumnCount+1) * borderWidth
	rightMargin := ctx.Widget.BodyMargin + ctx.Widget.BodyPadding
	availableWidth := ctx.Width - rightMargin - totalBorderWidth

	// Collect minimum and preferred widths for each column
	minWidths := make([]float32, table.ColumnCount)
	prefWidths := make([]float32, table.ColumnCount)

	for _, row := range table.Rows {
		for cellIdx, cell := range row.Cells {
			if cellIdx < table.ColumnCount {
				if cell.MinWidth > minWidths[cellIdx] {
					minWidths[cellIdx] = cell.MinWidth
				}
				if cell.PrefWidth > prefWidths[cellIdx] {
					prefWidths[cellIdx] = cell.PrefWidth
				}
			}
		}
	}

	// Calculate total minimum and preferred widths
	totalMinWidth := float32(0)
	totalPrefWidth := float32(0)
	for i := 0; i < table.ColumnCount; i++ {
		totalMinWidth += minWidths[i]
		totalPrefWidth += prefWidths[i]
	}

	// Distribute width using a fair algorithm
	if totalPrefWidth <= availableWidth {
		// We have enough space for preferred widths
		copy(table.ColumnWidths, prefWidths)
		
		// Distribute any extra space proportionally
		extraSpace := availableWidth - totalPrefWidth
		if extraSpace > 0 {
			for i := 0; i < table.ColumnCount; i++ {
				proportion := prefWidths[i] / totalPrefWidth
				table.ColumnWidths[i] += extraSpace * proportion
			}
		}
	} else if totalMinWidth <= availableWidth {
		// We can fit minimum widths, distribute remaining space proportionally
		extraSpace := availableWidth - totalMinWidth
		for i := 0; i < table.ColumnCount; i++ {
			table.ColumnWidths[i] = minWidths[i]
			if totalPrefWidth > totalMinWidth {
				proportion := (prefWidths[i] - minWidths[i]) / (totalPrefWidth - totalMinWidth)
				table.ColumnWidths[i] += extraSpace * proportion
			}
		}
	} else {
		// Not enough space even for minimums - distribute equally
		columnWidth := availableWidth / float32(table.ColumnCount)
		for i := 0; i < table.ColumnCount; i++ {
			table.ColumnWidths[i] = columnWidth
		}
	}

	table.TotalWidth = availableWidth + totalBorderWidth
}

// Phase 2c: Calculate row heights based on content and column widths
func (h *TableRenderHandler) calculateRowHeights(table *Table, ctx RenderContext) {
	for rowIdx := range table.Rows {
		row := &table.Rows[rowIdx]
		maxHeight := float32(0)

		for cellIdx := range row.Cells {
			if cellIdx >= table.ColumnCount {
				break // Skip cells beyond column count
			}

			cell := &row.Cells[cellIdx]
			cell.Width = table.ColumnWidths[cellIdx]
			
			// Calculate height needed for this cell's content
			cellHeight := h.calculateCellHeight(cell, ctx)
			cell.Height = cellHeight
			
			if cellHeight > maxHeight {
				maxHeight = cellHeight
			}
		}

		row.Height = maxHeight
		table.RowHeights[rowIdx] = maxHeight
		table.TotalHeight += maxHeight
	}

	// Add border heights
	table.TotalHeight += float32(len(table.Rows)+1) * 1 // 1px borders
}

func (h *TableRenderHandler) calculateCellHeight(cell *TableCell, ctx RenderContext) float32 {
	if len(cell.Content) == 0 {
		return 30 // Minimum cell height
	}

	text := h.extractCellText(cell.Content[0])
	if text == "" {
		return 30
	}

	font := ctx.Widget.Fonts.Regular
	if cell.IsHeader {
		font = ctx.Widget.Fonts.Bold
	}

	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	// Calculate height needed for text wrapping
	padding := float32(12)
	contentWidth := cell.Width - 2*padding
	if contentWidth <= 0 {
		contentWidth = 100 // Fallback
	}

	lineHeight := fontSize * 1.2
	
	// Simple word wrapping calculation
	words := strings.Fields(text)
	if len(words) == 0 {
		return lineHeight + 2*padding
	}

	currentLineWidth := float32(0)
	lineCount := 1

	for _, word := range words {
		wordWidth := ctx.Widget.measureTextWidth(font, word+" ", fontSize)
		if currentLineWidth + wordWidth > contentWidth && currentLineWidth > 0 {
			lineCount++
			currentLineWidth = wordWidth
		} else {
			currentLineWidth += wordWidth
		}
	}

	return float32(lineCount)*lineHeight + 2*padding
}

// Phase 3: Render the complete table
func (h *TableRenderHandler) renderTableContent(table *Table, ctx RenderContext) RenderResult {
	result := RenderResult{NextY: ctx.Y}
	
	currentY := ctx.Y + 10 // Top margin
	
	// Draw table border and background
	tableRect := rl.NewRectangle(ctx.X, currentY, table.TotalWidth, table.TotalHeight)
	rl.DrawRectangleRec(tableRect, rl.White)
	rl.DrawRectangleLinesEx(tableRect, 1, rl.Color{R: 200, G: 200, B: 200, A: 255})

	// Render each row
	for rowIdx, row := range table.Rows {
		h.renderTableRow(table, rowIdx, row, ctx.X, currentY, ctx)
		currentY += table.RowHeights[rowIdx] + 1 // +1 for border
		
		// Collect any link areas (future enhancement)
		// result.LinkAreas = append(result.LinkAreas, rowLinkAreas...)
	}

	result.NextY = currentY + 10 // Bottom margin
	result.Height = result.NextY - ctx.Y
	return result
}

func (h *TableRenderHandler) renderTableRow(table *Table, rowIdx int, row TableRow, startX, startY float32, ctx RenderContext) {
	currentX := startX + 1 // Start inside left border
	
	for cellIdx, cell := range row.Cells {
		if cellIdx >= table.ColumnCount {
			break
		}

		cellWidth := table.ColumnWidths[cellIdx]
		cellHeight := table.RowHeights[rowIdx]
		
		// Draw cell background (different for headers)
		cellRect := rl.NewRectangle(currentX, startY+1, cellWidth, cellHeight)
		if cell.IsHeader {
			rl.DrawRectangleRec(cellRect, rl.Color{R: 248, G: 249, B: 250, A: 255})
		}
		
		// Draw cell border
		rl.DrawRectangleLinesEx(cellRect, 1, rl.Color{R: 220, G: 220, B: 220, A: 255})
		
		// Render cell content
		h.renderCellContent(cell, currentX, startY+1, ctx)
		
		currentX += cellWidth + 1 // +1 for border
	}
}

func (h *TableRenderHandler) renderCellContent(cell TableCell, x, y float32, ctx RenderContext) {
	if len(cell.Content) == 0 {
		return
	}

	text := h.extractCellText(cell.Content[0])
	if text == "" {
		return
	}

	font := ctx.Widget.Fonts.Regular
	color := rl.Black
	if cell.IsHeader {
		font = ctx.Widget.Fonts.Bold
		color = rl.Color{R: 52, G: 58, B: 64, A: 255} // Slightly darker for headers
	}

	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	// Render text with padding and wrapping
	padding := float32(12)
	contentX := x + padding
	contentY := y + padding
	contentWidth := cell.Width - 2*padding
	
	// Simple text rendering with basic wrapping
	h.renderWrappedText(text, contentX, contentY, contentWidth, font, color, ctx)
}

func (h *TableRenderHandler) renderWrappedText(text string, x, y, width float32, font rl.Font, color rl.Color, ctx RenderContext) {
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}
	
	lineHeight := fontSize * 1.2
	currentY := y
	
	words := strings.Fields(text)
	if len(words) == 0 {
		return
	}

	currentLine := ""
	
	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word
		
		lineWidth := ctx.Widget.measureTextWidth(font, testLine, fontSize)
		
		if lineWidth > width && currentLine != "" {
			// Render current line and start new one
			ctx.Widget.renderTextWithUnicode(currentLine, x, currentY, font, color)
			currentY += lineHeight
			currentLine = word
		} else {
			currentLine = testLine
		}
	}
	
	// Render final line
	if currentLine != "" {
		ctx.Widget.renderTextWithUnicode(currentLine, x, currentY, font, color)
	}
}