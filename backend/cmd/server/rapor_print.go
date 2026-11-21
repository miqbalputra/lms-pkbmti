package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
)

//go:embed rapor_assets/layout.json
var reportLayout []byte

//go:embed rapor_assets/fonts/*.ttf
var reportFonts embed.FS

type reportTemplateCell struct {
	Address string
	Text    string
	Style   *excelize.Style
	Runs    []excelize.RichTextRun
	role    string
}
type reportTemplatePicture struct {
	Cell                string
	Data                []byte
	Extension           string
	Format              *excelize.GraphicOptions
	X, Y, Width, Height float64
}
type reportTemplate struct {
	Name         string
	Columns      []float64
	Rows         []float64
	Cells        []reportTemplateCell
	Merges       []string
	Pictures     []reportTemplatePicture
	ColumnPoints []float64
	values       map[string]string
	continuation bool
	extraRows    []int
	styled       bool
}

func reportPDF() *gofpdf.Fpdf {
	p := gofpdf.New("P", "mm", "A4", "")
	p.SetMargins(10, 16, 10)
	p.SetCellMargin(0)
	p.SetAutoPageBreak(false, 10)
	p.AddUTF8FontFromBytes("Report", "", goregular.TTF)
	p.AddUTF8FontFromBytes("Report", "B", gobold.TTF)
	p.AddUTF8FontFromBytes("Report", "I", goitalic.TTF)
	p.AddUTF8FontFromBytes("Report", "BI", gobolditalic.TTF)
	for _, font := range []struct{ family, file string }{{"Narrow", "ArialNarrow"}, {"Times", "Times"}, {"Arial", "Arial"}} {
		for _, weight := range []struct{ style, suffix string }{{"", ""}, {"B", "-Bold"}, {"I", "-Italic"}, {"BI", "-BoldItalic"}} {
			data, err := reportFonts.ReadFile("rapor_assets/fonts/" + font.file + weight.suffix + ".ttf")
			if err == nil {
				p.AddUTF8FontFromBytes(font.family, weight.style, data)
			}
		}
	}
	p.SetFont("Report", "", 11)
	return p
}

var reportMeasureOnce sync.Once
var reportMeasureMutex sync.Mutex
var reportMeasure *gofpdf.Fpdf

func reportTextFits(text string) bool {
	return reportAreaFits(text, 94, 4)
}
func reportAreaFits(text string, width float64, maxLines int) bool {
	if text == "" {
		return true
	}
	if len([]rune(text)) > 600 {
		return false
	}
	reportMeasureOnce.Do(func() {
		reportMeasure = gofpdf.New("P", "mm", "A4", "")
		reportMeasure.SetCellMargin(0)
		font, _ := reportFonts.ReadFile("rapor_assets/fonts/ArialNarrow.ttf")
		reportMeasure.AddUTF8FontFromBytes("Narrow", "", font)
		reportMeasure.SetFont("Narrow", "", 11)
	})
	reportMeasureMutex.Lock()
	defer reportMeasureMutex.Unlock()
	return len(reportMeasure.SplitText(text, width)) <= maxLines
}
func reportClassName(n int) string {
	names := []string{"", "Satu", "Dua", "Tiga", "Empat", "Lima", "Enam", "Tujuh", "Delapan", "Sembilan", "Sepuluh", "Sebelas", "Dua Belas"}
	if n > 0 && n < len(names) {
		return reportRoman(n) + " (" + names[n] + ")"
	}
	return fmt.Sprint(n)
}
func reportDate(v string) string {
	t, e := time.Parse("2006-01-02", v)
	if e != nil {
		return v
	}
	months := []string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}
	return fmt.Sprintf("%d %s %d", t.Day(), months[int(t.Month())], t.Year())
}
func reportValues(d ReportDocument, name string) map[string]string {
	v := map[string]string{}
	if name == "CETAK RAPOR" {
		semester := "1 (Satu)"
		if d.Scope.Semester == "Genap" {
			semester = "2 (Dua)"
		}
		v = map[string]string{"A9": d.Config.Program, "F11": d.Student.Nama, "F12": d.Student.NIS, "F13": d.Student.NISN, "AA11": reportClassName(d.Class.Jenjang), "AA12": d.Config.Phase, "AA13": semester, "AA14": d.Class.TahunAjaran.NamaTahunAjaran, "H47": strconv.Itoa(d.Attendance.Sakit), "H48": strconv.Itoa(d.Attendance.Izin), "H49": strconv.Itoa(d.Attendance.Alpa), "L47": d.Details.Note, "U61": d.Config.City + ", " + reportDate(d.Config.Date), "T68": d.Config.Teacher, "U69": "NIP. " + d.Config.TeacherNIP, "J78": d.Config.Principal, "J79": "NIP. " + d.Config.PrincipalNIP, "A52": "", "C44": "-", "K44": "-"}
		if d.Scope.Semester == "Genap" {
			decision := "TINGGAL DI KELAS " + reportClassName(d.Class.Jenjang)
			if d.Details.Promotion == "naik" {
				decision = "NAIK KELAS ke " + d.Details.NextClass
			}
			if d.Details.Promotion != "" {
				v["A52"] = "Berdasarkan Laporan Hasil Belajar (Rapor) Kelas " + reportClassName(d.Class.Jenjang) + " Fase " + d.Config.Phase + " Semester 1 (Satu) dan Semester 2 (Dua) Kurikulum Merdeka, maka peserta didik " + d.Student.Nama + " dinyatakan " + decision + "."
			}
		}
		if len(d.Details.Extras) > 0 {
			a, b := []string{}, []string{}
			for _, e := range d.Details.Extras {
				a = append(a, e.Name)
				b = append(b, e.Description)
			}
			v["C44"] = strings.Join(a, "\n")
			v["K44"] = strings.Join(b, "\n")
		}
		rows := []int{19, 21, 23, 26, 28, 30, 32, 34, 38, 40}
		for i, r := range rows {
			for _, col := range []string{"A", "C", "K", "N"} {
				v[fmt.Sprintf("%s%d", col, r)] = ""
			}
			v[fmt.Sprintf("N%d", r+1)] = ""
			if i < len(d.Subjects) && d.Subjects[i].Name != "" {
				m := d.Subjects[i]
				number := m.Number
				if number == 0 {
					number = i + 1
				}
				v[fmt.Sprintf("A%d", r)] = strconv.Itoa(number)
				v[fmt.Sprintf("C%d", r)] = m.Name
				if m.Score != nil {
					v[fmt.Sprintf("K%d", r)] = fmt.Sprintf("%.0f", *m.Score)
				}
				v[fmt.Sprintf("N%d", r)] = m.Strength
				v[fmt.Sprintf("N%d", r+1)] = m.Growth
			}
		}
	} else if name == "CETAK COVER" {
		v = map[string]string{"A6": d.Config.Program, "B26": strings.ToUpper(d.Student.Nama), "B30": d.Student.NIS, "F30": d.Student.NISN}
	} else {
		v = map[string]string{"D4": strings.ToUpper(d.Student.Nama), "D5": d.Student.NIS, "D6": d.Student.NISN, "D7": "", "D8": d.Student.JenisKelamin, "D9": "", "D10": "", "D11": "", "D12": "", "D13": "", "D14": "", "D15": "", "D16": "", "D17": "", "D18": "", "D19": "", "D20": "", "D21": "", "D22": "", "D23": "", "D24": ""}
		if d.Student.TanggalLahir != nil {
			v["D8"] = reportDate(d.Student.TanggalLahir.Format("2006-01-02"))
		}
		v["D7"] = ""
		v["D9"] = d.Student.JenisKelamin
		v["D10"] = ""
		v["D14"] = reportClassName(d.Class.Jenjang)
		v["D15"] = ""
		v["D16"] = ""
		v["D27"] = d.Config.City + ", " + reportDate(d.Config.Date)
		v["D33"] = d.Config.Principal
		for key, cell := range map[string]string{"birthPlace": "D7", "birthDate": "D8", "religion": "D10", "phone": "D11", "address": "D12", "admissionClass": "D14", "admissionStatus": "D15", "admissionYear": "D16", "father": "D18", "mother": "D19", "fatherJob": "D21", "motherJob": "D22", "guardian": "D23", "guardianJob": "D24"} {
			if value := d.Details.Identity[key]; value != "" {
				if key == "birthDate" {
					value = reportDate(value)
				}
				v[cell] = value
			}
		}
	}
	return v
}
func reportRGB(color string) (int, int, int) {
	color = strings.TrimPrefix(color, "#")
	if len(color) == 8 {
		color = color[2:]
	}
	if len(color) != 6 {
		return 0, 0, 0
	}
	n, e := strconv.ParseUint(color, 16, 32)
	if e != nil {
		return 0, 0, 0
	}
	return int(n >> 16), int(n >> 8 & 255), int(n & 255)
}
func reportRenderTemplate(p *gofpdf.Fpdf, t reportTemplate, d ReportDocument, from, to, page int) {
	t = reportStyledTemplate(t)
	p.AddPage()
	xs := []float64{10}
	ys := []float64{16}
	total := 0.0
	for i, w := range t.Columns {
		if len(t.ColumnPoints) == len(t.Columns) {
			total += t.ColumnPoints[i] * 25.4 / 72
		} else {
			total += math.Floor(w*7+.5) * 25.4 / 96
		}
	}
	// Excel template is printed at 100%, not stretched to fit the paper.
	scale := 1.0
	xs[0] = 10 + (190-total)/2
	for i, w := range t.Columns {
		width := math.Floor(w*7+.5) * 25.4 / 96
		if len(t.ColumnPoints) == len(t.Columns) {
			width = t.ColumnPoints[i] * 25.4 / 72
		}
		xs = append(xs, xs[len(xs)-1]+width*scale)
	}
	for i := from; i <= to; i++ {
		ys = append(ys, ys[len(ys)-1]+t.Rows[i-1]*25.4/72*scale)
	}
	type rect struct{ c1, r1, c2, r2 int }
	merges := map[string]rect{}
	membership := map[string]rect{}
	skip := map[string]bool{}
	for _, m := range t.Merges {
		parts := strings.Split(m, ":")
		c1, r1, _ := excelize.CellNameToCoordinates(parts[0])
		c2, r2, _ := excelize.CellNameToCoordinates(parts[1])
		if r1 < from || r2 > to || c2 > len(t.Columns) {
			continue
		}
		merges[parts[0]] = rect{c1, r1, c2, r2}
		for r := r1; r <= r2; r++ {
			for c := c1; c <= c2; c++ {
				address, _ := excelize.CoordinatesToCellName(c, r)
				membership[address] = rect{c1, r1, c2, r2}
				if address != parts[0] {
					skip[address] = true
				}
			}
		}
	}
	values := reportValues(d, t.Name)
	if t.values != nil {
		values = t.values
	}
	content := map[string]string{}
	for _, cell := range t.Cells {
		content[cell.Address] = cell.Text
	}
	for address, value := range values {
		content[address] = value
	}
	// Draw artwork first, including the cover frame. The principal stamp in the
	// source is not automatically used as a signature on a newly issued report.
	for i, pic := range t.Pictures {
		if t.Name == "CETAK RAPOR" && pic.Cell == "J69" || t.Name == "CETAK IDENTITAS" && pic.Cell == "C26" {
			continue
		}
		col, row, e := excelize.CellNameToCoordinates(pic.Cell)
		if e != nil || row < from || row > to || col > len(t.Columns) {
			continue
		}
		width, height := pic.Width, pic.Height
		if width == 0 {
			cfg, _, e := image.DecodeConfig(bytes.NewReader(pic.Data))
			if e != nil {
				continue
			}
			width = float64(cfg.Width) * 25.4 / 96
			height = float64(cfg.Height) * 25.4 / 96
		}
		name := fmt.Sprintf("%s-%d", t.Name, i)
		kind := strings.ToUpper(strings.TrimPrefix(pic.Extension, "."))
		p.RegisterImageOptionsReader(name, gofpdf.ImageOptions{ImageType: kind}, bytes.NewReader(pic.Data))
		p.ImageOptions(name, xs[col-1]+pic.X*scale, ys[row-from]+pic.Y*scale, width*scale, height*scale, false, gofpdf.ImageOptions{ImageType: kind}, 0, "")
	}
	if t.Name == "CETAK RAPOR" && from == 1 && !t.continuation {
		y := ys[6] + 1.8*scale
		p.SetLineWidth(.6)
		p.Line(xs[0], y, xs[len(xs)-1], y)
		p.SetLineWidth(.25)
		p.Line(xs[0], y+1.2, xs[len(xs)-1], y+1.2)
	}
	// Borders and fills belong to individual cells, including the perimeter of
	// merged cells. Render them before the merged text layer.
	for _, cell := range t.Cells {
		col, row, e := excelize.CellNameToCoordinates(cell.Address)
		if e != nil || row < from || row > to || col > len(t.Columns) || cell.Style == nil {
			continue
		}
		x, y := xs[col-1], ys[row-from]
		w, h := xs[col]-x, ys[row-from+1]-y
		style := cell.Style
		if style.Fill.Pattern == 1 && len(style.Fill.Color) > 0 {
			r, g, b := reportRGB(style.Fill.Color[0])
			p.SetFillColor(r, g, b)
			p.Rect(x, y, w, h, "F")
		}
		p.SetDrawColor(0, 0, 0)
		p.SetLineWidth(.18)
		for _, border := range style.Border {
			if border.Style == 0 {
				continue
			}
			if m, ok := membership[cell.Address]; ok {
				if border.Type == "left" && col != m.c1 || border.Type == "right" && col != m.c2 || border.Type == "top" && row != m.r1 || border.Type == "bottom" && row != m.r2 {
					continue
				}
			}
			switch border.Type {
			case "left":
				p.Line(x, y, x, y+h)
			case "right":
				p.Line(x+w, y, x+w, y+h)
			case "top":
				p.Line(x, y, x+w, y)
			case "bottom":
				p.Line(x, y+h, x+w, y+h)
			}
		}
	}
	for _, cell := range t.Cells {
		col, row, e := excelize.CellNameToCoordinates(cell.Address)
		if e != nil || row < from || row > to || col > len(t.Columns) || skip[cell.Address] {
			continue
		}
		c2, r2 := col, row
		if m, ok := merges[cell.Address]; ok {
			c2 = m.c2
			r2 = m.r2
		}
		x, y := xs[col-1], ys[row-from]
		w, h := xs[c2]-x, ys[r2-from+1]-y
		style := cell.Style
		if style == nil {
			continue
		}
		padding := reportCellPadding(cell)
		text := cell.Text
		if value, ok := values[cell.Address]; ok {
			text = value
		}
		if text == "" {
			continue
		}
		fontSize := 11.0
		fontStyle := ""
		if style.Font != nil {
			if style.Font.Size > 0 {
				fontSize = style.Font.Size
			}
			if style.Font.Bold {
				fontStyle += "B"
			}
			if style.Font.Italic {
				fontStyle += "I"
			}
			r, g, b := reportRGB(style.Font.Color)
			p.SetTextColor(r, g, b)
		} else {
			p.SetTextColor(0, 0, 0)
		}
		family := "Arial"
		if style.Font != nil {
			if style.Font.Family == "Times New Roman" {
				family = "Times"
			}
			if style.Font.Family == "Arial Narrow" {
				family = "Narrow"
			}
		}
		p.SetFont(family, fontStyle, fontSize*scale)
		lineHeight := fontSize * 25.4 / 72 * 1.29 * scale
		align := "L"
		vertical := "bottom"
		wrap := false
		if style.Alignment != nil {
			switch style.Alignment.Horizontal {
			case "center":
				align = "C"
			case "right":
				align = "R"
			}
			vertical = style.Alignment.Vertical
			wrap = style.Alignment.WrapText
		}
		lines := []string{text}
		if !wrap && align == "L" {
			if _, merged := merges[cell.Address]; !merged {
				for next := col + 1; next <= len(t.Columns); next++ {
					address, _ := excelize.CoordinatesToCellName(next, row)
					if content[address] != "" {
						break
					}
					w = xs[next] - x
				}
			}
		}
		if wrap {
			lines = p.SplitText(text, w-2*padding)
		}
		textHeight := float64(len(lines)) * lineHeight
		ty := y + padding
		if vertical == "center" {
			ty = y + (h-textHeight)/2
		}
		if vertical == "bottom" || vertical == "" {
			ty = y + h - textHeight - padding
		}
		if ty < y+padding {
			ty = y + padding
		}
		p.ClipRect(x, y, w, h, false)
		p.SetXY(x+padding, ty)
		if _, overridden := values[cell.Address]; !overridden && len(cell.Runs) > 1 && !wrap {
			width := 0.0
			for _, run := range cell.Runs {
				runStyle := fontStyle
				size := fontSize
				if run.Font != nil {
					runStyle = ""
					if run.Font.Bold {
						runStyle += "B"
					}
					if run.Font.Italic {
						runStyle += "I"
					}
					if run.Font.Underline != "" {
						runStyle += "U"
					}
					if run.Font.Size > 0 {
						size = run.Font.Size
					}
				}
				p.SetFont(family, runStyle, size)
				width += p.GetStringWidth(run.Text)
			}
			left := x + padding
			if align == "C" {
				left = x + (w-width)/2
			}
			if align == "R" {
				left = x + w - width - padding
			}
			p.SetXY(left, ty)
			for _, run := range cell.Runs {
				runStyle := fontStyle
				size := fontSize
				if run.Font != nil {
					runStyle = ""
					if run.Font.Bold {
						runStyle += "B"
					}
					if run.Font.Italic {
						runStyle += "I"
					}
					if run.Font.Underline != "" {
						runStyle += "U"
					}
					if run.Font.Size > 0 {
						size = run.Font.Size
					}
				}
				p.SetFont(family, runStyle, size)
				p.CellFormat(p.GetStringWidth(run.Text), lineHeight, run.Text, "", 0, "L", false, 0, "")
			}
			p.ClipEnd()
			continue
		}
		if wrap {
			// Pass the original paragraph: inserting hard newlines after each
			// measured line would prevent MultiCell from justifying it.
			if cell.role == "description" {
				align = "J"
			}
			p.MultiCell(w-2*padding, lineHeight, text, "", align, false)
		} else {
			p.CellFormat(w-2*padding, lineHeight, text, "", 0, align, false, 0, "")
		}
		p.ClipEnd()
	}
	if t.Name == "CETAK IDENTITAS" && from == 1 && len(ys) > 32 {
		p.SetDrawColor(0, 0, 0)
		p.SetLineWidth(.2)
		x, y := xs[1]+388469.0/36000, ys[27]+104588.0/36000
		p.Rect(x, y, 687295.0/36000, 887797.0/36000, "")
		p.SetFont("Arial", "", 9)
		p.SetTextColor(0, 0, 0)
		p.SetXY(x, y+7)
		p.MultiCell(687295.0/36000, 5, "Pas Foto\n\n3 x 4", "", "C", false)
	}
	if t.Name == "CETAK RAPOR" {
		if from == 71 && d.Config.PrincipalSignature != "" {
			if data, ok := signatureImage(d.Config.PrincipalSignature); ok {
				key := "principal-" + d.Student.ID + "-" + d.Scope.TahunAjaranID + "-" + d.Scope.Semester
				p.RegisterImageOptionsReader(key, gofpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(data))
				p.ImageOptions(key, 91, 27, 25, 22, false, gofpdf.ImageOptions{ImageType: "PNG"}, 0, "")
			}
		}
		semester := "Satu"
		if d.Scope.Semester == "Genap" {
			semester = "Dua"
		}
		p.SetFont("Times", "", 8)
		p.SetTextColor(0, 0, 0)
		p.SetXY(10, 285)
		p.CellFormat(165, 4, fmt.Sprintf("%s (%s / %s) | Rapor Semester %s | Kelas %s | Tahun Ajaran %s | PKBM Tunas Ilmu", d.Student.Nama, d.Student.NIS, d.Student.NISN, semester, reportRoman(d.Class.Jenjang), d.Class.TahunAjaran.NamaTahunAjaran), "", 0, "L", false, 0, "")
		p.SetXY(175, 285)
		p.CellFormat(25, 4, fmt.Sprintf("Halaman | %d", page), "", 0, "R", false, 0, "")
	}
}
func reportSendPDF(c *fiber.Ctx, docs []ReportDocument, kind string) error {
	p := reportPDF()
	var templates []reportTemplate
	if err := json.Unmarshal(reportLayout, &templates); err != nil {
		return err
	}
	for _, d := range docs {
		if kind == "cover" || kind == "lengkap" {
			reportRenderTemplate(p, templates[1], d, 1, 45, 0)
		}
		if kind == "identitas" || kind == "lengkap" {
			reportRenderTemplate(p, templates[2], d, 1, 34, 0)
		}
		if kind == "rapor" || kind == "lengkap" {
			// The original template has 3 general subjects on page 1, 5 on page 2,
			// and 2 empowerment subjects on page 3. Keep those physical page breaks.
			general, empower := []ReportSubject{}, []ReportSubject{}
			for i, m := range d.Subjects {
				if m.Number == 0 {
					m.Number = i + 1
				}
				if m.Group == "pemberdayaan" {
					empower = append(empower, m)
				} else {
					general = append(general, m)
				}
			}
			original := d
			d.Subjects = make([]ReportSubject, 10)
			copy(d.Subjects, general[:min(8, len(general))])
			copy(d.Subjects[8:], empower[max(0, len(empower)-2):])
			page := 1
			page += reportRenderAdaptive(p, templates[0], d, 1, 24, page)
			page += reportRenderAdaptive(p, templates[0], d, 25, 35, page)
			// Additional subjects reuse the original five-subject table before the
			// summary and signature pages, keeping the original type and cell sizes.
			for _, extra := range [][]ReportSubject{general[min(8, len(general)):], empower[:max(0, len(empower)-2)]} {
				for start := 0; start < len(extra); start += 5 {
					continued := original
					continued.Subjects = make([]ReportSubject, 10)
					copy(continued.Subjects[3:], extra[start:min(start+5, len(extra))])
					page += reportRenderAdaptive(p, templates[0], continued, 25, 35, page)
				}
			}
			page += reportRenderAdaptive(p, templates[0], d, 36, 70, page)
			reportRenderTemplate(p, templates[0], d, 71, 79, page)
		}
	}
	if p.Error() != nil {
		return p.Error()
	}
	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", "inline; filename=rapor-"+kind+".pdf")
	return p.Output(c.Response().BodyWriter())
}
func (s *Server) reportPrint(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	if err = s.scopeKelas(c, k.ID); err != nil {
		return err
	}
	kind := c.Query("kind", "rapor")
	if kind != "rapor" && kind != "cover" && kind != "identitas" && kind != "lengkap" {
		return fiber.ErrBadRequest
	}
	if version := c.QueryInt("version"); version > 0 {
		if c.Query("studentId") == "" {
			return fiber.NewError(400, "Pilih siswa untuk melihat versi rapor")
		}
		var rec ReportRecord
		if err = s.db.Where("period_id = ? AND student_id = ?", p.ID, c.Query("studentId")).First(&rec).Error; err != nil {
			return fiber.ErrNotFound
		}
		var revision ReportRevision
		if err = s.db.Where("record_id = ? AND version = ?", rec.ID, version).First(&revision).Error; err != nil {
			return fiber.ErrNotFound
		}
		var d ReportDocument
		if err = json.Unmarshal([]byte(revision.Snapshot), &d); err != nil {
			return err
		}
		return reportSendPDF(c, []ReportDocument{d}, kind)
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	docs := []ReportDocument{}
	for _, pd := range roster {
		if c.Query("studentId") != "" && pd.ID != c.Query("studentId") {
			continue
		}
		d, _, e := s.buildReport(s.db, k, p, pd.ID)
		if e != nil {
			return e
		}
		docs = append(docs, d)
	}
	if len(docs) == 0 {
		return fiber.NewError(404, "Tidak ada siswa untuk dicetak")
	}
	return reportSendPDF(c, docs, kind)
}
func (s *Server) reportLedger(c *fiber.Ctx) error {
	k, p, err := s.reportScope(c, false)
	if err != nil {
		return err
	}
	if err = s.scopeKelas(c, k.ID); err != nil {
		return err
	}
	roster, err := s.reportRoster(s.db, k)
	if err != nil {
		return err
	}
	type ledgerRow struct {
		Student  PesertaDidik    `json:"student"`
		Subjects []ReportSubject `json:"subjects"`
		Total    *float64        `json:"total"`
		Rank     *int            `json:"rank"`
	}
	rows := []ledgerRow{}
	for _, pd := range roster {
		d, _, e := s.buildReport(s.db, k, p, pd.ID)
		if e != nil {
			return e
		}
		row := ledgerRow{Student: pd, Subjects: d.Subjects}
		total := 0.0
		complete := len(d.Subjects) > 0
		for _, m := range d.Subjects {
			if m.Score == nil || !m.Complete {
				complete = false
			} else {
				total += *m.Score
			}
		}
		if complete {
			row.Total = &total
		}
		rows = append(rows, row)
	}
	allComplete := len(rows) > 0
	for _, r := range rows {
		if r.Total == nil {
			allComplete = false
		}
	}
	if allComplete {
		for i := range rows {
			rank := 1
			for _, r := range rows {
				if *r.Total > *rows[i].Total {
					rank++
				}
			}
			rows[i].Rank = &rank
		}
	}
	if c.Query("format") == "xlsx" {
		f := excelize.NewFile()
		defer f.Close()
		sheet := "Sheet1"
		headers := []interface{}{"No", "Nama", "NIS", "NISN"}
		cfg, e := s.reportConfig(s.db, k, p)
		if e != nil {
			return e
		}
		for _, m := range cfg.Subjects {
			headers = append(headers, m.Name)
		}
		headers = append(headers, "Jumlah", "Peringkat")
		if e = f.SetSheetRow(sheet, "A1", &headers); e != nil {
			return e
		}
		for i, r := range rows {
			values := []interface{}{i + 1, r.Student.Nama, r.Student.NIS, r.Student.NISN}
			for _, m := range r.Subjects {
				if m.Score == nil {
					values = append(values, "")
				} else {
					values = append(values, *m.Score)
				}
			}
			total, rank := interface{}(""), interface{}("")
			if r.Total != nil {
				total = *r.Total
			}
			if r.Rank != nil {
				rank = *r.Rank
			}
			values = append(values, total, rank)
			cell, _ := excelize.CoordinatesToCellName(1, i+2)
			if e = f.SetSheetRow(sheet, cell, &values); e != nil {
				return e
			}
		}
		f.SetColWidth(sheet, "B", "B", 28)
		f.SetColWidth(sheet, "C", "D", 16)
		f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2"})
		c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Attachment("leger.xlsx")
		return f.Write(c.Response().BodyWriter())
	}
	if c.Query("format") == "pdf" {
		pdf := reportPDF()
		pdf.AddPageFormat("L", gofpdf.SizeType{Wd: 297, Ht: 210})
		pdf.SetFont("Report", "B", 12)
		pdf.CellFormat(277, 8, "LEGER NILAI - "+k.TahunAjaran.NamaTahunAjaran+" / "+p.Semester, "", 1, "C", false, 0, "")
		pdf.SetFont("Report", "", 8)
		for _, r := range rows {
			if pdf.GetY() > 185 {
				pdf.AddPageFormat("L", gofpdf.SizeType{Wd: 297, Ht: 210})
			}
			text := r.Student.Nama + " | "
			for _, m := range r.Subjects {
				score := "-"
				if m.Score != nil {
					score = fmt.Sprintf("%.0f", *m.Score)
				}
				text += m.Name + ": " + score + "; "
			}
			if r.Total != nil {
				text += fmt.Sprintf("Jumlah %.0f", *r.Total)
			}
			if r.Rank != nil {
				text += fmt.Sprintf(" | Peringkat %d", *r.Rank)
			}
			pdf.MultiCell(277, 4, text, "1", "L", false)
		}
		c.Set("Content-Type", "application/pdf")
		return pdf.Output(c.Response().BodyWriter())
	}
	return c.JSON(rows)
}
