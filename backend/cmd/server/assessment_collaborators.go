package main

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	assessmentModuleUjian    = "ujian_online"
	assessmentModuleSimulasi = "simulasi"
	assessmentRoleEditor     = "editor"
	assessmentRoleGrader     = "grader"
	assessmentRoleViewer     = "viewer"
)

func validAssessmentModule(module string) bool {
	return module == assessmentModuleUjian || module == assessmentModuleSimulasi
}

func validAssessmentCollaboratorRole(role string) bool {
	return role == assessmentRoleEditor || role == assessmentRoleGrader || role == assessmentRoleViewer
}

func (s *Server) assessmentOwner(module, assessmentID string) (string, error) {
	switch module {
	case assessmentModuleUjian:
		var row Ujian
		if err := s.db.Select("dibuat_oleh_user_id").First(&row, "id = ?", assessmentID).Error; err != nil {
			return "", fiber.NewError(404, "ujian tidak ditemukan")
		}
		return row.DibuatOlehUserID, nil
	case assessmentModuleSimulasi:
		var row SimulasiPaket
		if err := s.db.Select("dibuat_oleh_user_id").First(&row, "id = ?", assessmentID).Error; err != nil {
			return "", fiber.NewError(404, "paket simulasi tidak ditemukan")
		}
		return row.DibuatOlehUserID, nil
	default:
		return "", fiber.NewError(404, "modul asesmen tidak dikenal")
	}
}

func (s *Server) assessmentCollaboratorRole(module, assessmentID, userID string) string {
	var row AsesmenKolaborator
	if s.db.Select("peran").Where("modul = ? AND asesmen_id = ? AND user_id = ?", module, assessmentID, userID).First(&row).Error != nil {
		return ""
	}
	return row.Peran
}

func (s *Server) assessmentCollaboratorIDs(module, userID string) []string {
	var ids []string
	_ = s.db.Model(&AsesmenKolaborator{}).Where("modul = ? AND user_id = ?", module, userID).Pluck("asesmen_id", &ids).Error
	return ids
}

func (s *Server) assessmentCollaboratorCanEdit(module, assessmentID, userID string) bool {
	return s.assessmentCollaboratorRole(module, assessmentID, userID) == assessmentRoleEditor
}

func (s *Server) assessmentCollaboratorCanGrade(module, assessmentID, userID string) bool {
	role := s.assessmentCollaboratorRole(module, assessmentID, userID)
	return role == assessmentRoleEditor || role == assessmentRoleGrader
}

func (s *Server) assessmentCollaborators(c *fiber.Ctx) error {
	module, assessmentID := c.Params("module"), c.Params("assessmentId")
	if !validAssessmentModule(module) {
		return fiber.NewError(404, "modul asesmen tidak dikenal")
	}
	ownerID, err := s.assessmentOwner(module, assessmentID)
	if err != nil {
		return err
	}
	userID, _ := c.Locals("userID").(string)
	role, _ := c.Locals("role").(string)
	canManage := role == "admin" || ownerID == userID
	currentRole := s.assessmentCollaboratorRole(module, assessmentID, userID)
	if !canManage {
		switch module {
		case assessmentModuleUjian:
			var row Ujian
			if err := s.db.First(&row, "id = ?", assessmentID).Error; err != nil {
				return fiber.NewError(404, "ujian tidak ditemukan")
			}
			if err := s.scopeUjian(c, &row); err != nil {
				return err
			}
		case assessmentModuleSimulasi:
			row, err := s.getSimulasiPaket(assessmentID)
			if err != nil {
				return fiber.NewError(404, "paket simulasi tidak ditemukan")
			}
			if err := s.simulasiPaketScope(c, row, false); err != nil {
				return err
			}
		}
	}
	var rows []AsesmenKolaborator
	if err := s.db.Where("modul = ? AND asesmen_id = ?", module, assessmentID).Order("created_at asc").Find(&rows).Error; err != nil {
		return fiber.NewError(500, "gagal memuat kolaborator")
	}
	result := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		var user User
		if err := s.db.Select("id", "username", "email", "role", "is_active").First(&user, "id = ?", row.UserID).Error; err != nil {
			continue
		}
		result = append(result, fiber.Map{"id": row.ID, "userId": user.ID, "username": user.Username, "email": user.Email, "peran": row.Peran, "aktif": user.IsActive})
	}
	if role == "admin" {
		currentRole = "admin"
	} else if ownerID == userID {
		currentRole = "owner"
	}
	return c.JSON(fiber.Map{"items": result, "canManage": canManage, "currentRole": currentRole})
}

func (s *Server) addAssessmentCollaborator(c *fiber.Ctx) error {
	module, assessmentID := c.Params("module"), c.Params("assessmentId")
	if !validAssessmentModule(module) {
		return fiber.NewError(404, "modul asesmen tidak dikenal")
	}
	ownerID, err := s.assessmentOwner(module, assessmentID)
	if err != nil {
		return err
	}
	userID, _ := c.Locals("userID").(string)
	role, _ := c.Locals("role").(string)
	if role != "admin" && ownerID != userID {
		return fiber.NewError(403, "hanya pemilik asesmen yang dapat menambahkan kolaborator")
	}
	var in struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Peran    string `json:"peran"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(400, "data kolaborator tidak valid")
	}
	in.UserID = strings.TrimSpace(in.UserID)
	in.Username = strings.TrimSpace(in.Username)
	in.Email = strings.TrimSpace(in.Email)
	in.Peran = strings.ToLower(strings.TrimSpace(in.Peran))
	if !validAssessmentCollaboratorRole(in.Peran) || (in.UserID == "" && in.Username == "" && in.Email == "") {
		return fiber.NewError(400, "pilih akun tutor dan peran editor, grader, atau viewer")
	}
	var target User
	query := s.db.Where("is_active = ? AND role = ?", true, "guru")
	switch {
	case in.UserID != "":
		query = query.Where("id = ?", in.UserID)
	case in.Email != "":
		query = query.Where("lower(email) = lower(?)", in.Email)
	default:
		query = query.Where("lower(username) = lower(?)", in.Username)
	}
	if err := query.First(&target).Error; err != nil {
		return fiber.NewError(404, "akun tutor aktif tidak ditemukan")
	}
	if target.ID == ownerID {
		return fiber.NewError(400, "pemilik asesmen tidak perlu ditambahkan sebagai kolaborator")
	}
	row := AsesmenKolaborator{Modul: module, AsesmenID: assessmentID, UserID: target.ID, Peran: in.Peran, DibuatOlehUserID: userID}
	if err := s.db.Where("modul = ? AND asesmen_id = ? AND user_id = ?", module, assessmentID, target.ID).Assign(map[string]any{"peran": in.Peran, "dibuat_oleh_user_id": userID}).FirstOrCreate(&row).Error; err != nil {
		return fiber.NewError(500, "gagal menyimpan kolaborator")
	}
	s.audit(&userID, "share", "asesmen_kolaborator", module+":"+assessmentID+":"+target.ID+":"+in.Peran)
	return c.Status(201).JSON(fiber.Map{"id": row.ID, "userId": target.ID, "username": target.Username, "email": target.Email, "peran": row.Peran, "aktif": target.IsActive})
}

func (s *Server) removeAssessmentCollaborator(c *fiber.Ctx) error {
	module, assessmentID := c.Params("module"), c.Params("assessmentId")
	if !validAssessmentModule(module) {
		return fiber.NewError(404, "modul asesmen tidak dikenal")
	}
	ownerID, err := s.assessmentOwner(module, assessmentID)
	if err != nil {
		return err
	}
	userID, _ := c.Locals("userID").(string)
	role, _ := c.Locals("role").(string)
	if role != "admin" && ownerID != userID {
		return fiber.NewError(403, "hanya pemilik asesmen yang dapat mencabut kolaborator")
	}
	result := s.db.Where("id = ? AND modul = ? AND asesmen_id = ?", c.Params("collaboratorId"), module, assessmentID).Delete(&AsesmenKolaborator{})
	if result.Error != nil {
		return fiber.NewError(500, "gagal mencabut akses kolaborator")
	}
	if result.RowsAffected == 0 {
		return fiber.NewError(404, "kolaborator tidak ditemukan")
	}
	s.audit(&userID, "revoke", "asesmen_kolaborator", module+":"+assessmentID+":"+c.Params("collaboratorId"))
	return c.SendStatus(204)
}
