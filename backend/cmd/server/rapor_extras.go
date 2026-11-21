package main

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

// nil preserves existing semesters and imported reports that predate this setting.
func reportExtrasEnabled(cfg ReportConfig) bool {
	return cfg.ExtrasEnabled == nil || *cfg.ExtrasEnabled
}

func reportExtrasEqual(a, b []ReportExtra) bool { return slices.Equal(a, b) }

func reportValidateExtraActivities(names []string) error {
	if len(names) > 20 {
		return fiber.NewError(400, "Maksimal 20 kegiatan ekstrakurikuler per kelas")
	}
	seen := map[string]bool{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || len([]rune(name)) > 100 || seen[key] {
			return fiber.NewError(400, "Nama kegiatan ekstrakurikuler harus diisi, unik, dan maksimal 100 karakter")
		}
		seen[key] = true
	}
	return nil
}

func reportValidateExtras(extras []ReportExtra) error {
	if len(extras) > 10 {
		return fiber.NewError(400, "Maksimal 10 ekstrakurikuler per siswa")
	}
	seen := map[string]bool{}
	for _, extra := range extras {
		key := strings.ToLower(strings.TrimSpace(extra.Name))
		if key == "" || seen[key] {
			return fiber.NewError(400, "Nama ekstrakurikuler wajib diisi dan tidak boleh sama")
		}
		if len([]rune(extra.Name)) > 100 || len([]rune(extra.Description)) > 1000 {
			return fiber.NewError(400, "Nama maksimal 100 karakter dan keterangan ekstrakurikuler maksimal 1.000 karakter")
		}
		seen[key] = true
	}
	return nil
}

// Each activity gets its own copy of the original template row. Synthetic row
// IDs allow several activities on the same page without merging their contents.
func reportExtraTemplate(p *gofpdf.Fpdf, t reportTemplate, d ReportDocument, values map[string]string) reportTemplate {
	if !reportExtrasEnabled(d.Config) || len(d.Details.Extras) == 0 {
		return t
	}
	t.Rows = append([]float64(nil), t.Rows...)
	t.Cells = append([]reportTemplateCell(nil), t.Cells...)
	t.Merges = append([]string(nil), t.Merges...)
	sourceCells := t.Cells
	for i, extra := range d.Details.Extras {
		width := 0.0
		for col := 10; col < 30; col++ {
			if len(t.ColumnPoints) == len(t.Columns) {
				width += t.ColumnPoints[col] * 25.4 / 72
			} else {
				width += math.Floor(t.Columns[col]*7+.5) * 25.4 / 96
			}
		}
		p.SetFont("Arial", "", 11)
		lines := p.SplitText(extra.Description, width-2*reportBodyPadding)
		pieces := []string{extra.Description}
		if len(lines) > 30 {
			pieces = nil
			for start := 0; start < len(lines); start += 30 {
				pieces = append(pieces, strings.Join(lines[start:min(start+30, len(lines))], " "))
			}
		}
		for part, description := range pieces {
			row := len(t.Rows) + 1
			t.extraRows = append(t.extraRows, row)
			t.Rows = append(t.Rows, t.Rows[43])
			for _, cell := range sourceCells {
				col, r, _ := excelize.CellNameToCoordinates(cell.Address)
				if r != 44 {
					continue
				}
				cell.Address, _ = excelize.CoordinatesToCellName(col, row)
				t.Cells = append(t.Cells, cell)
			}
			for _, merge := range []string{"A44:B44", "C44:J44", "K44:AD44"} {
				ends := strings.Split(merge, ":")
				c1, _, _ := excelize.CellNameToCoordinates(ends[0])
				c2, _, _ := excelize.CellNameToCoordinates(ends[1])
				a, _ := excelize.CoordinatesToCellName(c1, row)
				b, _ := excelize.CoordinatesToCellName(c2, row)
				t.Merges = append(t.Merges, a+":"+b)
			}
			values[fmt.Sprintf("A%d", row)] = fmt.Sprint(i + 1)
			values[fmt.Sprintf("C%d", row)] = extra.Name
			if part > 0 {
				values[fmt.Sprintf("C%d", row)] += " (lanjutan)"
			}
			values[fmt.Sprintf("K%d", row)] = description
		}
	}
	return t
}

func reportGrowExtraRows(p *gofpdf.Fpdf, t *reportTemplate, values map[string]string) {
	for _, row := range t.extraRows {
		height := t.Rows[row-1] * 25.4 / 72
		for _, area := range []struct {
			column     string
			start, end int
		}{{"C", 2, 10}, {"K", 10, 30}} {
			width := 0.0
			for i := area.start; i < area.end; i++ {
				if len(t.ColumnPoints) == len(t.Columns) {
					width += t.ColumnPoints[i] * 25.4 / 72
				} else {
					width += math.Floor(t.Columns[i]*7+.5) * 25.4 / 96
				}
			}
			p.SetFont("Arial", "", 11)
			lines := p.SplitText(values[fmt.Sprintf("%s%d", area.column, row)], width-2*reportBodyPadding)
			height = math.Max(height, float64(len(lines))*11*25.4/72*1.29+2*reportBodyPadding)
		}
		t.Rows[row-1] = height * 72 / 25.4
	}
}
