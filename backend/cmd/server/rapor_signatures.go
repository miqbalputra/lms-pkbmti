package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

func reportSignatureCount(d ReportDocument) int {
	if d.Scope.Semester == "Genap" {
		return 3
	}
	return 2
}

func reportSignatureHeight(p *gofpdf.Fpdf, d ReportDocument) float64 {
	height := 40 + reportSignatureTextHeight(p, d.Config.Teacher, true) + reportSignatureTextHeight(p, "NIP. "+d.Config.TeacherNIP, false)
	if reportSignatureCount(d) == 3 {
		height += 8 + 36 + reportSignatureTextHeight(p, d.Config.Principal, true) + reportSignatureTextHeight(p, "NIP. "+d.Config.PrincipalNIP, false)
	}
	return height + 3
}

func reportSignatureTextHeight(p *gofpdf.Fpdf, text string, bold bool) float64 {
	lineHeight := 4.5
	p.SetFont("Arial", "", 10)
	if bold {
		p.SetFont("Arial", "B", 11)
		lineHeight = 5
	}
	return float64(max(1, len(p.SplitText(text, 87)))) * lineHeight
}

// Reserve one indivisible block in the summary's pagination, replacing the
// separate parent/teacher and principal sections from the Excel template.
func reportSignatureTemplate(p *gofpdf.Fpdf, t reportTemplate, d ReportDocument) reportTemplate {
	t.Rows = append([]float64(nil), t.Rows...)
	t.Cells = append([]reportTemplateCell(nil), t.Cells...)
	cells := t.Cells[:0]
	for _, cell := range t.Cells {
		_, row, _ := excelize.CellNameToCoordinates(cell.Address)
		if row < 61 || row > 79 {
			cells = append(cells, cell)
		}
	}
	t.Cells = cells
	merges := []string{}
	for _, merge := range t.Merges {
		ends := strings.Split(merge, ":")
		_, a, _ := excelize.CellNameToCoordinates(ends[0])
		_, b, _ := excelize.CellNameToCoordinates(ends[1])
		if b < 61 || a > 79 {
			merges = append(merges, merge)
		}
	}
	t.Merges = merges
	pictures := []reportTemplatePicture{}
	for _, picture := range t.Pictures {
		_, row, _ := excelize.CellNameToCoordinates(picture.Cell)
		if row < 61 || row > 79 {
			pictures = append(pictures, picture)
		}
	}
	t.Pictures = pictures
	t.signatureRow = 61
	t.Rows[60] = reportSignatureHeight(p, d) * 72 / 25.4
	for i := 61; i < 79; i++ {
		t.Rows[i] = 0
	}
	return t
}

func reportDrawSignatures(p *gofpdf.Fpdf, d ReportDocument, y, tableLeft, tableRight float64) {
	width := 95.0
	p.SetTextColor(0, 0, 0)
	p.SetFont("Arial", "", 11)
	p.SetXY(tableLeft, y)
	p.CellFormat(tableRight-tableLeft, 5, d.Config.City+", "+reportDate(d.Config.Date), "", 0, "R", false, 0, "")
	titles := []string{"Orang Tua/Wali Murid", "Wali Kelas"}
	names := []string{"", d.Config.Teacher}
	nips := []string{"", d.Config.TeacherNIP}
	nameHeight := reportSignatureTextHeight(p, d.Config.Teacher, true)
	for i, title := range titles {
		x := 10 + float64(i)*width + 4
		p.SetFont("Arial", "B", 11)
		p.SetXY(x, y+9)
		p.MultiCell(width-8, 5, title, "", "C", false)
		if i == 0 {
			p.SetDrawColor(75, 75, 75)
			p.SetLineWidth(.16)
			p.Line(32.5, y+44, 82.5, y+44)
			continue
		}
		p.SetXY(x, y+40)
		p.MultiCell(width-8, 5, names[i], "", "C", false)
		p.SetFont("Arial", "", 10)
		p.SetXY(x, y+40+nameHeight)
		p.MultiCell(width-8, 4.5, "NIP. "+nips[i], "", "C", false)
	}
	if reportSignatureCount(d) != 3 {
		return
	}
	principalTop := y + 40 + nameHeight + reportSignatureTextHeight(p, "NIP. "+d.Config.TeacherNIP, false) + 8
	p.SetFont("Arial", "B", 11)
	p.SetXY(61.5, principalTop)
	p.MultiCell(87, 5, "Mengetahui,\nKepala PKBM Tunas Ilmu", "", "C", false)
	p.SetXY(61.5, principalTop+36)
	p.MultiCell(87, 5, d.Config.Principal, "", "C", false)
	principalNameHeight := reportSignatureTextHeight(p, d.Config.Principal, true)
	p.SetFont("Arial", "", 10)
	p.SetXY(61.5, principalTop+36+principalNameHeight)
	p.MultiCell(87, 4.5, "NIP. "+d.Config.PrincipalNIP, "", "C", false)
	if d.Config.PrincipalSignature != "" {
		if data, ok := signatureImage(d.Config.PrincipalSignature); ok {
			key := fmt.Sprintf("principal-%s-%s-%s", d.Student.ID, d.Scope.TahunAjaranID, d.Scope.Semester)
			p.RegisterImageOptionsReader(key, gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(data))
			p.ImageOptions(key, 92.5, principalTop+12, 25, 20, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		}
	}
}
