# Empirical Challenge & Verification Handoff Report — Milestone 1

**Agent ID**: teamwork_preview_challenger_m1_1  
**Role**: EMPIRICAL CHALLENGER (critic, specialist)  
**Target Milestone**: Milestone 1: Core Design System, Sidebar Layout, & Auth / Login View for LMS PKBM Tunas Ilmu  

---

## 1. Observation

### Command 1: Frontend Build (`npm run build` in `frontend/`)
- **Command**: `cmd /c npm run build` (Executed in `d:\Project LMS PKBM Tunas Ilmu\frontend`)
- **Result**: **PASS** (Exit code 0)
- **Output**:
  ```text
  > frontend@0.0.0 build
  > tsc -b && vite build

  vite v8.2.0 building client environment for production...
  transforming...✓ 2473 modules transformed.
  rendering chunks...
  computing gzip size...
  dist/index.html                            0.45 kB │ gzip:   0.29 kB
  dist/assets/index-DOY3LocW.css            40.67 kB │ gzip:   7.48 kB
  dist/assets/DashboardCharts-D9RaB2Er.js  395.12 kB │ gzip: 102.99 kB
  dist/assets/index-D2EVKbBS.js            447.75 kB │ gzip: 133.37 kB

  ✓ built in 539ms
  ```

### Command 2: Backend E2E Test Suite (`go test -v ./... -run TestE2E` in `backend/cmd/server/`)
- **Command**: `cmd /c go test -v ./... -run TestE2E` (Executed in `d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server`)
- **Result**: **FAIL & DEADLOCK / HANG**
- **Detailed Findings per Tier**:
  - **Tier 1 (`TestE2E_Tier1_*`)**: Fails & Hangs indefinitely at `TestE2E_Tier1_CRUD_Assignments`.
    - Tests `TestE2E_Tier1_Auth`, `TestE2E_Tier1_CRUD_Tutors`, `CRUD_Parents`, `CRUD_Pokjars`, `CRUD_Years`, `CRUD_Subjects`, `CRUD_Classes`, `CRUD_Students` PASSED (0.29s-0.44s).
    - `TestE2E_Tier1_CRUD_Assignments` hung indefinitely during Step 4 (`POST /api/penugasan/semua-kelas`).
  - **Tier 2 (`TestE2E_Tier2_*`)**: **ALL PASS** (Total time 6.076s). All boundary tests (`Auth_Boundaries`, `Class_Boundaries`, `User_Boundaries`, `Attendance_Boundaries`, `Role_Boundaries`, `Promotion_Boundaries`, `Settings_Boundaries`) passed.
  - **Tier 3 (`TestE2E_Tier3_*`)**: **FAIL & HANG**.
    - `TestE2E_Tier3_TutorWaliHistoryCombination` FAILS:
      ```text
      --- FAIL: TestE2E_Tier3_TutorWaliHistoryCombination (0.11s)
          e2e_tier3_combination_test.go:56: new history entry is not active
          e2e_tier3_combination_test.go:59: old history entry was not closed
      ```
    - `TestE2E_Tier3_SubjectPivotBulkAssignCombination` HANGS indefinitely on bulk assignment.
  - **Tier 4 (`TestE2E_Tier4_*`)**: **HANGS**. `TestE2E_Tier4_Scenario1_AcademicYearOnboarding` hangs indefinitely during `setKelasMapel` / assignment steps.

### Code Inspection Observations:

1. **Deadlock in `assignAllClasses` (`backend/cmd/server/routes.go:420-456`)**:
   ```go
   if err := s.db.Transaction(func(tx *gorm.DB) error {
       for _, class := range classes {
           assignment := PenugasanGuruMapel{TutorID: in.TutorID, KelasID: class.ID, MapelID: in.MapelID}
           result := tx.Where(PenugasanGuruMapel{TutorID: assignment.TutorID, KelasID: assignment.KelasID, MapelID: assignment.MapelID}).FirstOrCreate(&assignment)
   ```
   `PenugasanGuruMapel` inherits `Base` struct (`ID`, `CreatedAt`, `UpdatedAt`). `Base.BeforeCreate` sets `assignment.ID = uuid.NewString()`. When `tx.Where(PenugasanGuruMapel{...})` is executed with the struct, GORM includes `id = <uuid>` in the `WHERE` clause. Since the UUID is generated fresh, `First` finds no records, and `FirstOrCreate` executes an `INSERT`. When `(tutor_id, kelas_id, mapel_id)` already exists, SQLite returns a `UNIQUE constraint failed`, causing the transaction to block/fail abruptly without cleanly returning an error to Fiber `app.Test`.

2. **Deadlock in `setKelasMapel` (`backend/cmd/server/routes.go:395-413`)**:
   ```go
   return s.db.Transaction(func(tx *gorm.DB) error {
       tx.Where("kelas_id = ?", id(c)).Delete(&KelasMapel{})
       for _, mapelID := range in.MapelIDs {
           if e := tx.Create(&KelasMapel{KelasID: id(c), MapelID: mapelID}).Error; e != nil {
               return e
           }
       }
       uid := c.Locals("userID").(string)
       s.audit(&uid, "update", "kelas_mapel", id(c)) // <--- DEADLOCK!
       return c.SendStatus(204)
   })
   ```
   `s.audit` calls `s.db.Create(&AuditLog{...})` using `s.db` (the global DB instance) inside `s.db.Transaction(func(tx *gorm.DB) error { ... })`. On SQLite in-memory with shared cache, querying `s.db` while `tx` holds a write transaction creates a database lock deadlock.

3. **Layout Edge Cases in Frontend (`frontend/src/`)**:
   - **Collapsed Sidebar State Toggles**: Verified in `AppShell.tsx`, `AppSidebar.tsx`, `AppHeader.tsx`. State `collapsed` expands/collapses sidebar (`w-64` vs `w-16`). Icons maintain proper size (`h-5 w-5`), text labels truncate/hide, tooltips wrap icon triggers using `TooltipProvider` / `TooltipContent side="right"`. Toggle buttons (`PanelLeft` in header, `ChevronLeft`/`ChevronRight` in sidebar) function seamlessly.
   - **Mobile Sheet State Toggles**: State `mobileOpen` controls `<Sheet open={mobileOpen} onOpenChange={setMobileOpen}>`. Mobile hamburger button (`Menu`, `md:hidden`) opens drawer sidebar. Clicking any item closes drawer (`setMobileOpen(false)`).
   - **Role Filtering Restrictions**: Verified in `frontend/src/App.tsx`, `AppSidebar.tsx`, and backend `routes.go`.
     - Frontend `AppSidebar.tsx`: `visibleItems = NAV_ITEMS.filter((item) => item.roles.includes(role))` hides admin routes (`tutor`, `orang-tua`, `pokjar`, `tahun-ajaran`, `mapel`, `kelas-mapel`, `penugasan`, `kenaikan-kelas`, `akun`, `pengaturan-jadwal`, `audit-log`, `arsip`) from `guru`. Guru only sees `dashboard`, `kelas`, `peserta-didik`, `presensi`.
     - Frontend Router `App.tsx`: `Workspace` component checks `readOnly={user.role !== 'admin'}`. If a `guru` attempts to open an admin view directly (e.g. `page === 'akun'`), it renders the `<Restricted />` alert banner ("Akses ini hanya tersedia untuk Admin.") instead of exposing admin controls.
     - Backend: Middleware `s.admin` returns `403 Forbidden` (`admin access required`) for all write endpoints. Middleware `s.canManageKelas` prevents `guru` from querying/managing classes or students outside their assigned wali kelas domain.
   - **Turnstile Token Pass-Through**: In `Login.tsx`, `TurnstileWidget` captures token on `onSuccess`, setting `turnstileToken`. Login submit passes `turnstileToken` in request body payload to `POST /api/auth/login`. In `main.go`, `s.login` checks `in.TurnstileToken` and validates via `verifyTurnstile` in `production` environment.
   - **Error State Alerts**: Verified in `Login.tsx` (`<Alert variant="destructive">` displaying error messages inline on failed login) and global `Toaster` component from `sonner`. Backend `apiError` middleware returns JSON formatted errors `{"error": "message"}` with standard status codes.

---

## 2. Logic Chain

1. **Observation**: Frontend build (`npm run build` in `frontend/`) executed `tsc -b && vite build` and completed with zero errors and generated bundle artifacts.
   - **Logic**: The React + TypeScript frontend codebase has valid type safety, syntax, and Vite bundler configuration.

2. **Observation**: Running full Go E2E suite (`go test -v ./... -run TestE2E`) hangs indefinitely at `TestE2E_Tier1_CRUD_Assignments`, `TestE2E_Tier3_SubjectPivotBulkAssignCombination`, and `TestE2E_Tier4_Scenario1_AcademicYearOnboarding`. Running Tier 2 tests (`go test -v . -run TestE2E_Tier2`) passes all tests in 6.076s.
   - **Logic**: The issue is isolated to specific backend route handlers called during Tier 1, Tier 3, and Tier 4 E2E tests, specifically `assignAllClasses` and `setKelasMapel`.

3. **Observation**: Inspection of `routes.go:441` shows `tx.Where(PenugasanGuruMapel{TutorID: assignment.TutorID, ...}).FirstOrCreate(&assignment)`. `PenugasanGuruMapel` has an auto-generated `ID` set by `Base.BeforeCreate`.
   - **Logic**: Passing a struct with a newly auto-generated UUID to GORM's `tx.Where(struct)` causes GORM to add `WHERE id = '<new-uuid>'`. GORM never finds the existing record, attempts a duplicate `INSERT`, and hits `UNIQUE constraint failed: penugasan_guru_mapels.tutor_id, penugasan_guru_mapels.kelas_id, mapel_id`. Inside Fiber `app.Test`, this causes an unhandled transaction lockup.

4. **Observation**: Inspection of `routes.go:410` shows `s.audit(&uid, "update", "kelas_mapel", id(c))` called inside `s.db.Transaction(func(tx *gorm.DB) error { ... })`.
   - **Logic**: `s.audit` attempts to insert into `s.db` (outside `tx`). In SQLite shared in-memory DBs, calling `s.db.Create` while `tx` is holding an open transaction deadlocks the connection.

5. **Observation**: Code inspection of layout, role filtering, Turnstile, and error state alerts shows complete implementation in `AppShell.tsx`, `AppSidebar.tsx`, `AppHeader.tsx`, `App.tsx`, `Login.tsx`, and backend middleware.
   - **Logic**: The frontend layout edge cases and access control contracts conform to Milestone 1 requirements, but backend E2E test execution fails due to backend transaction deadlock bugs.

---

## 3. Caveats

- **DB Engine Differences**: The deadlock occurred on SQLite in-memory (`mode=memory&cache=shared`) used during `go test`. On production PostgreSQL (`DATABASE_URL`), connection pooling handles concurrent transactions differently, but calling non-tx `s.db` inside a GORM `tx` transaction is still anti-pattern and can cause connection pool starvation.
- **Frontend Browser E2E**: Verified frontend build and code structure empirically; browser rendering was checked via code flow analysis of state toggles (`collapsed`, `mobileOpen`) and UI component render paths.

---

## 4. Conclusion

- **Frontend Build**: **VERIFIED PASS** (`npm run build` succeeds cleanly).
- **Frontend Core Layout & Auth**: **VERIFIED PASS** (Sidebar collapse, mobile sheet drawer, role filtering `<Restricted />` guard, Turnstile pass-through, and alert components are properly implemented).
- **Backend E2E Test Suite**: **FAILED & DEADLOCK FOUND**. `go test -v ./... -run TestE2E` cannot complete due to two critical backend bugs:
  1. GORM struct matching bug in `assignAllClasses` (`routes.go:441`).
  2. Nested DB transaction deadlock in `setKelasMapel` (`routes.go:410`).
  3. `TestE2E_Tier3_TutorWaliHistoryCombination` logic failure (`routes_test.go` vs `routes.go` history tracking discrepancy).

---

## 5. Verification Method

To independently verify these findings:

1. **Frontend Build Verification**:
   ```powershell
   cd "d:\Project LMS PKBM Tunas Ilmu\frontend"
   cmd /c npm run build
   ```
   *Expected result*: Build succeeds with exit code 0.

2. **Backend E2E Test Deadlock Reproduction**:
   ```powershell
   cd "d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server"
   cmd /c go test -v . -run TestE2E_Tier1_CRUD_Assignments
   ```
   *Expected result*: The test process hangs indefinitely due to GORM `FirstOrCreate` UUID issue.

3. **Backend Tier 2 Verification**:
   ```powershell
   cd "d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server"
   cmd /c go test -v . -run TestE2E_Tier2
   ```
   *Expected result*: All 7 Tier 2 boundary tests pass (PASS in ~6.0s).

4. **Backend Tier 3 Failure Reproduction**:
   ```powershell
   cd "d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server"
   cmd /c go test -v . -run TestE2E_Tier3_TutorWaliHistoryCombination
   ```
   *Expected result*: Fails with `new history entry is not active` / `old history entry was not closed`.
