package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestReportNarrativeCurriculumRules(t *testing.T) {
	data, err := os.ReadFile("testdata/report_narrative_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name             string
		Labels           []string
		Values           []*float64
		Strength, Growth string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			cps, scores := []ReportCP{}, []ReportScore{}
			for i, label := range tc.Labels {
				id := fmt.Sprint(i)
				cps = append(cps, ReportCP{Base: Base{ID: id}, Label: label, Urutan: i + 1})
				scores = append(scores, ReportScore{CPID: id, Value: tc.Values[i]})
			}
			before := reportJSON([]interface{}{cps, scores})
			_, a, b, _ := reportCalculate(cps, scores)
			if a != tc.Strength || b != tc.Growth {
				t.Fatalf("got %q / %q; want %q / %q", a, b, tc.Strength, tc.Growth)
			}
			if strings.Contains(a+b, "dan dan") {
				t.Fatal("joining duplicated a conjunction")
			}
			if after := reportJSON([]interface{}{cps, scores}); after != before {
				t.Fatal("generating a description mutated CP labels or scores")
			}
		})
	}
}

func TestReportNarrativeAIUsesSelectedEvidence(t *testing.T) {
	cps, scores := []ReportCP{}, []ReportScore{}
	for i, value := range []float64{100, 70, 90, 60, 80} {
		id := fmt.Sprint(i)
		cps = append(cps, ReportCP{Base: Base{ID: id}, Label: "Membaca CP" + id})
		score := value
		scores = append(scores, ReportScore{CPID: id, Value: &score})
	}
	strong, growth := reportSelectNarrativeCPs(cps, scores)
	prompt := reportNarrativeAIPrompt("Bahasa Indonesia", 1, strong, growth, "catatan tutor")
	for _, text := range []string{reportStrengthOpening, reportGrowthOpening, "serta", "masing-masing 2", "jika daftar capaian terendah kosong", "membaca CP0", "membaca CP2", "membaca CP3", "membaca CP1"} {
		if !strings.Contains(prompt, text) {
			t.Fatalf("AI prompt missing %q", text)
		}
	}
	if strings.Contains(prompt, "membaca CP4") {
		t.Fatal("unselected middle CP was sent as AI evidence")
	}
	valid := reportAIResult{Strength: reportStrengthOpening + "membaca teks.", Growth: reportGrowthOpening + "menulis kalimat."}
	if !reportAIMatchesNarrativeFormat(valid, true) {
		t.Fatal("valid AI format rejected")
	}
	for _, tc := range []struct {
		result    reportAIResult
		hasGrowth bool
	}{
		{reportAIResult{Strength: "Ananda mampu membaca.", Growth: valid.Growth}, true},
		{reportAIResult{Strength: reportStrengthOpening + "Membaca teks.", Growth: valid.Growth}, true},
		{reportAIResult{Strength: valid.Strength, Growth: "Perlu bimbingan lebih lanjut."}, true},
		{reportAIResult{Strength: reportStrengthOpening + ".", Growth: valid.Growth}, true},
		{reportAIResult{Strength: valid.Strength}, true},
		{valid, false},
	} {
		if reportAIMatchesNarrativeFormat(tc.result, tc.hasGrowth) {
			t.Fatalf("noncompliant AI output accepted: %+v", tc)
		}
	}
	if !reportAIMatchesNarrativeFormat(reportAIResult{Strength: valid.Strength}, false) {
		t.Fatal("single-CP suggestion must allow empty growth")
	}
}
