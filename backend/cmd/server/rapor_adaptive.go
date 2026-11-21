package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

// Only description rows grow. All dimensions are kept in the template's points.
func reportDescriptionLines(p *gofpdf.Fpdf, t reportTemplate, text string) ([]string, float64) {
	width := 0.0
	for i := 13; i < 30; i++ {
		if len(t.ColumnPoints) == len(t.Columns) {
			width += t.ColumnPoints[i] * 25.4 / 72
		} else {
			width += math.Floor(t.Columns[i]*7+.5) * 25.4 / 96
		}
	}
	size := 11.0
	for _, cell := range t.Cells {
		if cell.Address == "N19" && cell.Style != nil && cell.Style.Font != nil {
			size = cell.Style.Font.Size
		}
	}
	p.SetFont("Arial", "", size)
	return p.SplitText(text, width-2*reportBodyPadding), size * 25.4 / 72 * 1.29
}

func reportGrowDescriptions(p *gofpdf.Fpdf, t reportTemplate, values map[string]string) reportTemplate {
	t.Rows = append([]float64(nil), t.Rows...)
	for _, row := range []int{19, 20, 21, 22, 23, 24, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 38, 39, 40, 41} {
		lines, height := reportDescriptionLines(p, t, values[fmt.Sprintf("N%d", row)])
		required := (float64(len(lines))*height + 2*reportBodyPadding) * 72 / 25.4
		t.Rows[row-1] = math.Max(t.Rows[row-1], required)
	}
	reportGrowExtraRows(p, &t, values)
	if t.styled {
		reportGrowBorderedCells(p, &t, values)
	}
	return t
}

type reportRowBlock struct {
	rows   []int
	values map[string]string
}

// Build a page using source rows, preserving their styles, merged borders and
// resolved values even when a subject continues on another page.
func reportPageFragment(t reportTemplate, rows []int, values map[string]string, heights []float64, continued bool) reportTemplate {
	f := t
	f.Rows, f.Cells, f.Merges, f.Pictures = nil, nil, nil, nil
	f.values = map[string]string{}
	f.continuation = continued
	indices := map[int]int{}
	for i, row := range rows {
		indices[row] = i + 1
		f.Rows = append(f.Rows, heights[row-1])
	}
	f.signatureRow = indices[t.signatureRow]
	for _, cell := range t.Cells {
		col, row, _ := excelize.CellNameToCoordinates(cell.Address)
		if target, ok := indices[row]; ok {
			original := cell.Address
			cell.Address, _ = excelize.CoordinatesToCellName(col, target)
			f.Cells = append(f.Cells, cell)
			if value, exists := values[original]; exists {
				f.values[cell.Address] = value
			}
		}
	}
	for _, merge := range t.Merges {
		ends := strings.Split(merge, ":")
		c1, r1, _ := excelize.CellNameToCoordinates(ends[0])
		c2, r2, _ := excelize.CellNameToCoordinates(ends[1])
		if indices[r1] != 0 && indices[r2] != 0 {
			a, _ := excelize.CoordinatesToCellName(c1, indices[r1])
			b, _ := excelize.CoordinatesToCellName(c2, indices[r2])
			f.Merges = append(f.Merges, a+":"+b)
		}
	}
	for _, pic := range t.Pictures {
		// Do not reintroduce the workbook's decorative signature image when
		// rows are renumbered on a continuation page.
		if t.Name == "CETAK RAPOR" && pic.Cell == "J69" {
			continue
		}
		col, row, _ := excelize.CellNameToCoordinates(pic.Cell)
		if indices[row] != 0 {
			pic.Cell, _ = excelize.CoordinatesToCellName(col, indices[row])
			f.Pictures = append(f.Pictures, pic)
		}
	}
	return f
}

func reportRenderAdaptive(p *gofpdf.Fpdf, t reportTemplate, d ReportDocument, from, to, page int) int {
	t = reportStyledTemplate(t)
	values := reportValues(d, t.Name)
	if from == 36 {
		t = reportSignatureTemplate(p, t, d)
		t = reportExtraTemplate(p, t, d, values)
	}
	grown := reportGrowDescriptions(p, t, values)
	changed := false
	if from == 36 {
		changed = true // summary includes the semester-specific signature block
	}
	for row := from; row <= to; row++ {
		changed = changed || grown.Rows[row-1] > t.Rows[row-1]+.001
	}
	if !changed {
		reportRenderTemplate(p, t, d, from, to, page)
		return 1
	}
	// The original page breaks remain the defaults. Extra pages are inserted
	// only inside a section whose descriptions actually overflow.
	headerEnd := map[int]int{1: 18, 25: 25, 36: 37}[from]
	header := []int{}
	for r := from; r <= headerEnd; r++ {
		header = append(header, r)
	}
	blocks := []reportRowBlock{}
	for row := headerEnd + 1; row <= to; {
		if from == 36 && row == 51 && d.Scope.Semester != "Genap" {
			row = 56
			continue
		}
		if from == 36 && row == 43 {
			if !reportExtrasEnabled(d.Config) {
				row = 46
				continue
			}
			if len(t.extraRows) > 0 {
				for i, r := range t.extraRows {
					rows := []int{r}
					if i == 0 {
						rows = []int{43, r}
					}
					blocks = append(blocks, reportRowBlock{rows, values})
				}
				row = 45
				continue
			}
		}
		end := row
		if from == 36 {
			switch row {
			case 43:
				end = 44
			case 46:
				end = 49
			case 51:
				end = 70
			case 56:
				end = 70
			case 61:
				end = 70
			}
		}
		for {
			previous := end
			for _, merge := range t.Merges {
				ends := strings.Split(merge, ":")
				_, a, _ := excelize.CellNameToCoordinates(ends[0])
				_, b, _ := excelize.CellNameToCoordinates(ends[1])
				if a >= row && a <= end && b <= to && b > end {
					end = b
				}
			}
			if end == previous {
				break
			}
		}
		rows := []int{}
		for r := row; r <= end; r++ {
			rows = append(rows, r)
		}
		// Very tall descriptions use bounded chunks that also fit beneath
		// the first page's letterhead (its available body is about 149 mm).
		// Both halves retain the same subject number, name and score.
		if end == row+1 && row <= 40 && (values[fmt.Sprintf("N%d", row)] != "" || values[fmt.Sprintf("N%d", end)] != "") {
			a, _ := reportDescriptionLines(p, t, values[fmt.Sprintf("N%d", row)])
			b, _ := reportDescriptionLines(p, t, values[fmt.Sprintf("N%d", end)])
			height := grown.Rows[row-1] + grown.Rows[end-1]
			if height*25.4/72 > 140 {
				for start := 0; start < max(len(a), len(b)); start += 12 {
					v := map[string]string{}
					for k, value := range values {
						v[k] = value
					}
					v[fmt.Sprintf("N%d", row)] = strings.Join(a[min(start, len(a)):min(start+12, len(a))], " ")
					v[fmt.Sprintf("N%d", end)] = strings.Join(b[min(start, len(b)):min(start+12, len(b))], " ")
					if start > 0 {
						v[fmt.Sprintf("C%d", row)] += " (lanjutan)"
					}
					blocks = append(blocks, reportRowBlock{rows, v})
				}
				row = end + 1
				continue
			}
		}
		blocks = append(blocks, reportRowBlock{rows, values})
		row = end + 1
	}
	count := 0
	pageRows := append([]int(nil), header...)
	pageValues := map[string]string{}
	for k, v := range values {
		pageValues[k] = v
	}
	heights := append([]float64(nil), grown.Rows...)
	height := 0.0
	for _, r := range pageRows {
		height += heights[r-1] * 25.4 / 72
	}
	nextHeader := 25
	flush := func() {
		f := reportPageFragment(t, pageRows, pageValues, heights, from != 1 || count > 0)
		reportRenderTemplate(p, f, d, 1, len(f.Rows), page+count)
		count++
		// Continuations repeat the table column headings, without the letterhead.
		pageRows = nil
		height = 0
		if nextHeader > 0 {
			pageRows = []int{nextHeader}
			height = t.Rows[nextHeader-1] * 25.4 / 72
		}
		heights = append([]float64(nil), t.Rows...)
		pageValues = map[string]string{}
		for k, v := range values {
			pageValues[k] = v
		}
	}
	for _, block := range blocks {
		nextHeader = 0
		if block.rows[0] <= 41 {
			nextHeader = 25
		} else if block.rows[0] > 79 {
			nextHeader = 43
		}
		bt := reportGrowDescriptions(p, t, block.values)
		bh := 0.0
		for _, r := range block.rows {
			bh += bt.Rows[r-1] * 25.4 / 72
		}
		repeated := false
		for _, existing := range pageRows {
			for _, r := range block.rows {
				repeated = repeated || existing == r
			}
		}
		if (height+bh > 263 || repeated) && len(pageRows) > 0 {
			flush()
		}
		for k, v := range block.values {
			_, r, _ := excelize.CellNameToCoordinates(k)
			for _, source := range block.rows {
				if r == source {
					pageValues[k] = v
				}
			}
		}
		for _, r := range block.rows {
			pageRows = append(pageRows, r)
			heights[r-1] = bt.Rows[r-1]
		}
		height += bh
	}
	if len(pageRows) > 0 {
		flush()
	}
	return count
}
