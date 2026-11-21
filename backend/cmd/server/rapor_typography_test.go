package main

import (
	"encoding/json"
	"testing"
)

func TestReportTypographyAndContinuation(t *testing.T) {
	var templates []reportTemplate
	if err := json.Unmarshal(reportLayout, &templates); err != nil {
		t.Fatal(err)
	}
	base := templates[0]
	styled := reportStyledTemplate(base)
	found := false
	for _, cell := range styled.Cells {
		if cell.Address == "N17" && (len(cell.Style.Fill.Color) == 0 || cell.Style.Fill.Color[0] != "F1F3F5") {
			t.Fatal("table header should be light gray")
		}
		if cell.Address == "N19" {
			found = true
			if cell.Style.Font.Family != "Arial" || cell.Style.Font.Size != 11 || cell.Style.Alignment.Horizontal != "justify" || reportCellPadding(cell) != 2 {
				t.Fatal("description typography is inconsistent")
			}
		}
	}
	if !found {
		t.Fatal("missing description")
	}
	for _, cell := range base.Cells {
		if cell.Address == "N19" && cell.Style.Font.Family != "Arial Narrow" {
			t.Fatal("shared source template changed")
		}
	}
	values := reportValues(ReportDocument{}, base.Name)
	grown := reportGrowDescriptions(reportPDF(), styled, values)
	if grown.Rows[48]*25.4/72 > 25 {
		t.Fatal("attendance label wrapped into individual letters")
	}
	rows := []int{61, 62, 63, 64, 65, 66, 67, 68, 69, 70}
	f := reportPageFragment(styled, rows, values, grown.Rows, true)
	if len(f.Pictures) != 0 {
		t.Fatal("workbook signature picture leaked into continuation")
	}
}
