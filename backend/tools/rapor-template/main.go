// Command rapor-template extracts layout and artwork only; student data and macros
// are never included in the application template.
package main

import (
	"archive/zip"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/xuri/excelize/v2"
	"os"
	"path/filepath"
)

type cell struct {
	Address string
	Text    string
	Style   *excelize.Style
	Runs    []excelize.RichTextRun `json:",omitempty"`
}
type picture struct {
	Cell                string
	Data                []byte
	Extension           string
	Format              *excelize.GraphicOptions
	X, Y, Width, Height float64
}
type marker struct {
	Col    int   `xml:"col"`
	Row    int   `xml:"row"`
	ColOff int64 `xml:"colOff"`
	RowOff int64 `xml:"rowOff"`
}
type anchor struct {
	From marker `xml:"from"`
	Pic  struct {
		Props struct {
			Transform struct {
				Off struct {
					X, Y int64 `xml:",attr"`
				} `xml:"off"`
				Ext struct {
					CX int64 `xml:"cx,attr"`
					CY int64 `xml:"cy,attr"`
				} `xml:"ext"`
			} `xml:"xfrm"`
		} `xml:"spPr"`
	} `xml:"pic"`
}
type drawing struct {
	Anchors []anchor `xml:"twoCellAnchor"`
}
type sheet struct {
	Name         string
	Columns      []float64
	Rows         []float64
	Cells        []cell
	Merges       []string
	Pictures     []picture
	ColumnPoints []float64
}

// Geometry measured from Excel 2021 at the workbook's original 100% print zoom.
//
//go:embed geometry.json
var geometry []byte

func main() {
	if len(os.Args) != 3 {
		panic("usage: rapor-template source.xlsm output.json")
	}
	f, err := excelize.OpenFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	archive, err := zip.OpenReader(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer archive.Close()
	drawings := map[string]string{"CETAK RAPOR": "xl/drawings/drawing3.xml", "CETAK COVER": "xl/drawings/drawing2.xml", "CETAK IDENTITAS": "xl/drawings/drawing1.xml"}
	specs := []struct {
		name       string
		cols, rows int
	}{{"CETAK RAPOR", 30, 79}, {"CETAK COVER", 9, 45}, {"CETAK IDENTITAS", 4, 34}}
	result := []sheet{}
	var exactColumns map[string]struct {
		Columns []float64
		Rows    []float64
	}
	if err = json.Unmarshal(bytesWithoutBOM(geometry), &exactColumns); err != nil {
		panic(err)
	}
	for _, spec := range specs {
		s := sheet{Name: spec.name, ColumnPoints: exactColumns[spec.name].Columns}
		for col := 1; col <= spec.cols; col++ {
			name, _ := excelize.ColumnNumberToName(col)
			v, _ := f.GetColWidth(spec.name, name)
			s.Columns = append(s.Columns, v)
		}
		for row := 1; row <= spec.rows; row++ {
			v, _ := f.GetRowHeight(spec.name, row)
			s.Rows = append(s.Rows, v)
			for col := 1; col <= spec.cols; col++ {
				address, _ := excelize.CoordinatesToCellName(col, row)
				value, _ := f.GetCellValue(spec.name, address)
				formula, _ := f.GetCellFormula(spec.name, address)
				if formula != "" {
					value = ""
				}
				styleID, _ := f.GetCellStyle(spec.name, address)
				style, _ := f.GetStyle(styleID)
				runs, _ := f.GetCellRichText(spec.name, address)
				if formula != "" || len(runs) < 2 {
					runs = nil
				}
				s.Cells = append(s.Cells, cell{Address: address, Text: value, Style: style, Runs: runs})
				pics, _ := f.GetPictures(spec.name, address)
				for _, p := range pics {
					s.Pictures = append(s.Pictures, picture{Cell: address, Data: p.File, Extension: p.Extension, Format: p.Format})
				}
			}
		}
		for _, entry := range archive.File {
			if entry.Name != drawings[spec.name] {
				continue
			}
			reader, e := entry.Open()
			if e != nil {
				panic(e)
			}
			var dr drawing
			e = xml.NewDecoder(reader).Decode(&dr)
			reader.Close()
			if e != nil {
				panic(e)
			}
			for i := range s.Pictures {
				pic := &s.Pictures[i]
				for _, a := range dr.Anchors {
					address, _ := excelize.CoordinatesToCellName(a.From.Col+1, a.From.Row+1)
					if address == pic.Cell && a.Pic.Props.Transform.Ext.CX > 0 {
						pic.X = float64(a.From.ColOff) / 36000
						pic.Y = float64(a.From.RowOff) / 36000
						pic.Width = float64(a.Pic.Props.Transform.Ext.CX) / 36000
						pic.Height = float64(a.Pic.Props.Transform.Ext.CY) / 36000
					}
				}
			}
		}
		merges, _ := f.GetMergeCells(spec.name)
		if len(exactColumns[spec.name].Rows) == len(s.Rows) {
			s.Rows = exactColumns[spec.name].Rows
		}
		for _, m := range merges {
			s.Merges = append(s.Merges, m.GetStartAxis()+":"+m.GetEndAxis())
		}
		result = append(result, s)
	}
	b, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	if err = os.MkdirAll(filepath.Dir(os.Args[2]), 0755); err != nil {
		panic(err)
	}
	if err = os.WriteFile(os.Args[2], b, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Extracted %d template sheets, %d bytes\n", len(result), len(b))
}
func bytesWithoutBOM(v []byte) []byte {
	if len(v) >= 3 && v[0] == 239 && v[1] == 187 && v[2] == 191 {
		return v[3:]
	}
	return v
}
