package main

import "github.com/gofiber/fiber/v2"

type ReportPromotionRules struct {
	Completion bool   `json:"completion"`
	NextClass  string `json:"nextClass"`
	ValidGrade bool   `json:"validGrade"`
}

func reportPromotionRules(grade int) ReportPromotionRules {
	rules := ReportPromotionRules{ValidGrade: grade >= 1 && grade <= 12, Completion: grade == 6 || grade == 9 || grade == 12}
	if rules.ValidGrade && !rules.Completion {
		rules.NextClass = reportClassName(grade + 1)
	}
	return rules
}

func reportPromotionAllowed(grade int, decision string) bool {
	rules := reportPromotionRules(grade)
	if !rules.ValidGrade {
		return false
	}
	if rules.Completion {
		return decision == "lulus" || decision == "belum_lulus"
	}
	return decision == "naik" || decision == "tinggal"
}

// Reads never rewrite saved decisions. Recompute the target only when a tutor
// explicitly changes the decision/target or requests the automatic target.
func reportPreparePromotion(grade int, semester string, previous ReportDetails, next *ReportDetails, update bool) error {
	if semester != "Genap" {
		next.Promotion, next.NextClass = previous.Promotion, previous.NextClass
		return nil
	}
	if !update && next.Promotion == previous.Promotion && next.NextClass == previous.NextClass {
		return nil
	}
	if next.Promotion == "" {
		next.NextClass = ""
		return nil
	}
	if !reportPromotionAllowed(grade, next.Promotion) {
		if reportPromotionRules(grade).Completion {
			return fiber.NewError(400, "Kelas akhir program memakai keputusan Lulus atau Belum lulus")
		}
		return fiber.NewError(400, "Pilih keputusan Naik kelas atau Tinggal kelas")
	}
	next.NextClass = ""
	if next.Promotion == "naik" {
		next.NextClass = reportPromotionRules(grade).NextClass
	}
	return nil
}
