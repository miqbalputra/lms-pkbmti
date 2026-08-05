package main
import ("encoding/xml";"fmt";"os";"strings")
type wb struct{ XMLName xml.Name `xml:"Workbook"`; Ws []struct{ Table struct{ Rows []struct{ Cells []struct{ Data string `xml:"Data"` } `xml:"Cell"` } `xml:"Row"` } `xml:"Table"` } `xml:"Worksheet"` }
func main() {
	raw,_:=os.ReadFile(os.Args[1]); var w wb; xml.Unmarshal(raw,&w)
	badNIK, badNISN, ok := 0,0,0
	for _,sh:=range w.Ws {
		for i,r:=range sh.Table.Rows {
			if i==0||len(r.Cells)<13 {continue}
			nm:=strings.TrimSpace(r.Cells[6].Data)
			nisn:=strings.TrimSpace(r.Cells[8].Data)
			nik:=strings.TrimSpace(r.Cells[12].Data)
			nikOK := len(nik)==16 && strings.IndexFunc(nik, func(r rune)bool{ return r<'0'||r>'9'})==-1
			nisnOK := len(nisn)==10 && strings.IndexFunc(nisn, func(r rune)bool{ return r<'0'||r>'9'})==-1
			if !nikOK { badNIK++ }
			if !nisnOK { badNISN++ }
			if nikOK { ok++ }
			fmt.Printf("nikOK=%v nisnOK=%v  nik=%q nisn=%q  %s\n", nikOK, nisnOK, nik, nisn, nm)
		}
	}
	fmt.Printf("\nSUMMARY: rows=%d  validNIK=%d  badNIK=%d  badNISN=%d\n", ok+badNIK, ok, badNIK, badNISN)
}
