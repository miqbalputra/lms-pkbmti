package main

import (
	"sort"
	"strings"
	"unicode"
)

const reportStrengthOpening = "Ananda menunjukkan penguasaan yang baik dalam hal "
const reportGrowthOpening = "Perlu peningkatan dalam hal "

type reportNarrativeCP struct {
	Label string  `json:"capaian"`
	Value float64 `json:"nilai"`
	order int
}

// Format the generated clause only. Stored CP labels and tutor text are never
// rewritten. Internal punctuation and proper-name capitalization stay intact.
func reportCPClause(label string) string {
	return reportLowercaseCPStart(strings.TrimSpace(strings.TrimRight(strings.TrimSpace(label), ".")))
}

func reportLowercaseCPStart(label string) string {
	text := []rune(label)
	if len(text) > 0 {
		text[0] = unicode.ToLower(text[0])
	}
	return string(text)
}

// Count scored, nonempty CP entries rather than punctuation in their labels.
// Equal scores follow the original CP order and never select the same entry
// for both sections. One scored CP therefore has no growth section.
func reportSelectNarrativeCPs(cps []ReportCP, scores []ReportScore) ([]reportNarrativeCP, []reportNarrativeCP) {
	values := map[string]*float64{}
	for _, score := range scores {
		values[score.CPID] = score.Value
	}
	ranked := []reportNarrativeCP{}
	for order, cp := range cps {
		if value := values[cp.ID]; value != nil && reportCPClause(cp.Label) != "" {
			ranked = append(ranked, reportNarrativeCP{Label: reportCPClause(cp.Label), Value: *value, order: order})
		}
	}
	count := 1
	if len(ranked) > 4 {
		count = 2
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].Value > ranked[j].Value })
	if len(ranked) < count {
		count = len(ranked)
	}
	strong := append([]reportNarrativeCP{}, ranked[:count]...)
	rest := append([]reportNarrativeCP{}, ranked[count:]...)
	sort.SliceStable(rest, func(i, j int) bool {
		if rest[i].Value == rest[j].Value {
			return rest[i].order < rest[j].order
		}
		return rest[i].Value < rest[j].Value
	})
	if len(rest) < count {
		count = len(rest)
	}
	return strong, rest[:count]
}

func reportNarrativeSentence(opening string, selected []reportNarrativeCP) string {
	if len(selected) == 0 {
		return ""
	}
	labels := make([]string, len(selected))
	for i, cp := range selected {
		labels[i] = cp.Label
	}
	return opening + strings.Join(labels, " serta ") + "."
}

func reportAutomaticNarrative(cps []ReportCP, scores []ReportScore) (string, string) {
	strong, growth := reportSelectNarrativeCPs(cps, scores)
	return reportNarrativeSentence(reportStrengthOpening, strong), reportNarrativeSentence(reportGrowthOpening, growth)
}
