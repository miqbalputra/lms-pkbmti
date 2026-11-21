package main

import (
	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
	"math"
	"strings"
)

const reportBodyPadding = 2.0 // mm, shared by measurement and painting

func reportStyledTemplate(t reportTemplate) reportTemplate {
	if t.Name != "CETAK RAPOR" || t.styled {
		return t
	}
	t.styled = true
	// The source workbook relies on text spilling across these attendance
	// cells. Merge the label explicitly so wrapping uses its complete width.
	t.Merges = append(append([]string(nil), t.Merges...), "A49:F49")
	t.Cells = append([]reportTemplateCell(nil), t.Cells...)
	for i := range t.Cells {
		cell := &t.Cells[i]
		col, row, _ := excelize.CellNameToCoordinates(cell.Address)
		if cell.role == "" {
			body := row >= 19 && row <= 24 || row >= 26 && row <= 35 || row >= 38 && row <= 41 || row == 44
			if body {
				cell.role = "body"
				if col >= 14 || row == 44 && col >= 11 {
					cell.role = "description"
				}
			}
			if cell.Address == "L47" || cell.Address == "A52" {
				cell.role = "description"
			}
			if cell.Address == "A47" || cell.Address == "A48" || cell.Address == "A49" {
				cell.role = "label"
			}
		}
		if cell.role == "" || cell.Style == nil {
			continue
		}
		style := *cell.Style
		font := excelize.Font{Family: "Arial", Size: 11}
		if style.Font != nil {
			font = *style.Font
			font.Family = "Arial"
			font.Size = 11
		}
		alignment := excelize.Alignment{WrapText: true, Vertical: "center"}
		if style.Alignment != nil {
			alignment = *style.Alignment
			alignment.WrapText = true
		}
		if cell.role == "description" {
			alignment.Vertical = "top"
			alignment.Horizontal = "justify"
		}
		style.Font = &font
		style.Alignment = &alignment
		cell.Style = &style
	}
	return t
}

func reportCellPadding(cell reportTemplateCell) float64 {
	if cell.role != "" {
		return reportBodyPadding
	}
	if cell.Style != nil {
		for _, b := range cell.Style.Border {
			if b.Style != 0 {
				return 1.2
			}
		}
	}
	return .55
}

// Padding must participate in height calculations too: short attendance rows
// and the promotion paragraph otherwise become clipped by their old borders.
func reportGrowBorderedCells(p *gofpdf.Fpdf, t *reportTemplate, values map[string]string) {
	type area struct{ c1, r1, c2, r2 int }
	merges := map[string]area{}
	skip := map[string]bool{}
	for _, m := range t.Merges {
		ends := strings.Split(m, ":")
		c1, r1, _ := excelize.CellNameToCoordinates(ends[0])
		c2, r2, _ := excelize.CellNameToCoordinates(ends[1])
		merges[ends[0]] = area{c1, r1, c2, r2}
		for r := r1; r <= r2; r++ {
			for c := c1; c <= c2; c++ {
				a, _ := excelize.CoordinatesToCellName(c, r)
				if a != ends[0] {
					skip[a] = true
				}
			}
		}
	}
	for _, cell := range t.Cells {
		if cell.Style == nil || skip[cell.Address] {
			continue
		}
		col, row, _ := excelize.CellNameToCoordinates(cell.Address)
		if row < 17 || row > 70 && row <= 79 {
			continue
		}
		bordered := false
		for _, b := range cell.Style.Border {
			bordered = bordered || b.Style != 0
		}
		if !bordered {
			continue
		}
		text := cell.Text
		if v, ok := values[cell.Address]; ok {
			text = v
		}
		if text == "" {
			continue
		}
		a := area{col, row, col, row}
		if m, ok := merges[cell.Address]; ok {
			a = m
		}
		width, height := 0.0, 0.0
		for i := a.c1 - 1; i < a.c2; i++ {
			if len(t.ColumnPoints) == len(t.Columns) {
				width += t.ColumnPoints[i] * 25.4 / 72
			} else {
				width += math.Floor(t.Columns[i]*7+.5) * 25.4 / 96
			}
		}
		for r := a.r1; r <= a.r2; r++ {
			height += t.Rows[r-1] * 25.4 / 72
		}
		size, family, weight := 11.0, "Arial", ""
		if font := cell.Style.Font; font != nil {
			if font.Size > 0 {
				size = font.Size
			}
			if font.Bold {
				weight += "B"
			}
			if font.Italic {
				weight += "I"
			}
			if font.Family == "Arial Narrow" {
				family = "Narrow"
			}
			if font.Family == "Times New Roman" {
				family = "Times"
			}
		}
		p.SetFont(family, weight, size)
		padding := reportCellPadding(cell)
		lines := 1
		if cell.Style.Alignment != nil && cell.Style.Alignment.WrapText {
			lines = len(p.SplitText(text, width-2*padding))
		}
		need := float64(lines)*size*25.4/72*1.29 + 2*padding
		if need > height {
			t.Rows[a.r2-1] += (need - height) * 72 / 25.4
		}
	}
}
