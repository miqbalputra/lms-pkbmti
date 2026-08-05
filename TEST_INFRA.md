# E2E Test Infrastructure & Specification — LMS PKBM Tunas Ilmu

## 1. Test Philosophy

The E2E test infrastructure for **LMS PKBM Tunas Ilmu** follows an **opaque-box, requirement-driven testing approach** based strictly on the business domain requirements specified in `PRD.md` and `ORIGINAL_REQUEST.md`.

### Core Principles
1. **Opaque-Box Verification**: Tests validate external API contracts, HTTP status codes, payload structures, RBAC permissions, and database state persistence without relying on non-public implementation details.
2. **Zero Production Mutation**: E2E test execution operates against mock/isolated databases (in-memory SQLite / test instances) without modifying production source code or production database states.
3. **4-Tier Test Coverage**: Structured hierarchically from single-feature contracts to complex real-world multi-actor end-to-end workflows.
4. **Deterministic & Isolated**: Each test suite creates isolated, seeded environments, ensuring parallel execution safety and zero inter-test side effects.
5. **Strict Build Integrity**: All test suites must execute cleanly alongside `cmd /c npm run build` (frontend TypeScript/Vite compiler) and `go test ./...` (Go backend test runner).

---

## 2. Feature Inventory & Matrix

| Module ID | Feature Name | Core Functionality | Required Tier 1 Tests |
|---|---|---|---|
| **F01** | Auth | Cloudflare Turnstile token validation, JWT access/refresh issuance, cookie management, token rotation, logout revocation, failed login rate-limiting/lockout | >= 5 |
| **F02** | CRUD Tutors | Create, list, detail view, update, delete tutor records; link to login user accounts | >= 5 |
| **F03** | CRUD Parents (Orang Tua) | Manage parent profiles (Ibu/Bapak), list, update, delete; multi-child student linkage | >= 5 |
| **F04** | CRUD Pokjars | Manage learning group centers (Pusat & Binaan/Cabang), unique name enforcement, CRUD | >= 5 |
| **F05** | CRUD Years (Tahun Ajaran) | Academic period management, start/end dates, single-active-year enforcement (toggle active auto-deactivates others) | >= 5 |
| **F06** | CRUD Subjects (Mapel) | Subject master data, name/code attributes, active/inactive toggle, CRUD operations | >= 5 |
| **F07** | CRUD Classes (Kelas) | Rombel management (Jenjang 1-6 + Rombel A/B/C), pokjar & academic year binding, wali kelas assignment, wali history tracking, duplicate classes across years | >= 5 |
| **F08** | CRUD Students (Peserta Didik) | Student records with NIS/NISN, class/pokjar assignment, status tracking, atomic Excel bulk import with validation & rollback | >= 5 |
| **F09** | CRUD Assignments (Penugasan) | Class-subject mapping pivot (`setKelasMapel`), subject teacher assignment per class, bulk assignment across all classes with subject | >= 5 |
| **F10** | Attendance Canvas (Presensi) | Weekly meeting presensi, base64 PNG signature validation, status handling (berlangsung/libur/dipindah), per-student checklist, CSV & PDF exports | >= 5 |
| **F11** | Promotion (Kenaikan Kelas) | End-of-year mass promotion wizard, status override (naik/tinggal/lulus/pindah/keluar), append-only `RiwayatKelasPesertaDidik` history, target class year validation | >= 5 |
| **F12** | Accounts (Akun/Users) | User account administration, role assignment (admin, kepala_sekolah, guru), guru-tutor link requirement, password security, lockout after 5 failures | >= 5 |
| **F13** | Settings (Pengaturan Jadwal) | Global single-row schedule configuration, default day (Sabtu), generation time (HH:MM WIB), timezone validation | >= 5 |
| **F14** | Audit Logs | Sensitive activity logging (login, create, update, delete, promote, import), resource/action query filtering, 250 record capping | >= 5 |

---

## 3. Test Architecture & Directory Structure

```
d:\Project LMS PKBM Tunas Ilmu\
├── backend/
│   └── cmd/
│       └── server/
│           ├── auth_test.go              # Unit auth tests
│           ├── routes_test.go            # Route helper tests
│           ├── e2e_tier1_feature_test.go   # Tier 1: Feature Coverage (70+ tests)
│           ├── e2e_tier2_boundary_test.go  # Tier 2: Boundary & Corner Cases (25+ tests)
│           ├── e2e_tier3_combination_test.go # Tier 3: Pairwise & Cross-Feature (20+ tests)
│           └── e2e_tier4_scenario_test.go  # Tier 4: Real-World E2E Workflows (10+ scenarios)
├── e2e/
│   ├── README.md                         # E2E Test Suite documentation & manual trigger instructions
│   └── run_e2e.bat                       # Automated test runner script for Windows
├── TEST_INFRA.md                         # This test infrastructure document
└── TEST_READY.md                         # Readiness summary & coverage checklist
```

---

## 4. 4-Tier Testing Methodology

### Tier 1: Feature Coverage (Minimum 5 test cases per feature)
- Validates the baseline functional contracts for all 14 system features.
- Total expected Tier 1 test cases: **>= 70 tests**.

### Tier 2: Boundary & Corner Cases
- Tests negative paths, edge cases, invalid inputs, role violations, and security guards:
  - Empty strings, invalid email/date formats, passwords < 8 characters.
  - Out-of-bound Jenjang values (< 1 or > 6).
  - Malformed or non-PNG base64 signature strings (> 1MB or JPEG headers).
  - Invalid schedule generation times (e.g. `25:00`).
  - Cross-class attendance access attempts by non-assigned Wali Kelas (IDOR).
  - Mutation requests (POST/PUT/DELETE) by `kepala_sekolah` role (403 Forbidden).
  - Unauthenticated endpoint access attempts without JWT headers.
  - Re-use of revoked or expired refresh tokens.
  - Malformed Excel bulk import files (invalid headers, > 5MB size, > 1000 rows).
  - Promotion to target class in non-matching academic year.

### Tier 3: Cross-Feature Combinations
- Validates pairwise interactions and state cascading across system boundaries:
  - **Tutor Assignment → Attendance Generation**: Changing Wali Kelas closes prior `RiwayatWaliKelas` and assigns newly generated attendance meetings to active tutor.
  - **Student Creation → Class History**: Creating a student automatically appends an initial `RiwayatKelasPesertaDidik` record.
  - **Class Duplication → Subject Binding**: Duplicate classes to a new year retains Jenjang, Rombel, and Pokjar bindings while resetting Wali Kelas pointers.
  - **Subject Configuration → Bulk Teacher Assignment**: `assignAllClasses` targets only classes having the specified subject linked in `KelasMapel`.
  - **Student Promotion → History Archive**: `promote` updates `PesertaDidik.KelasID` and status while creating append-only entries in `RiwayatKelasPesertaDidik`.
  - **User Login → Audit Logging**: Authenticating generates a log record in `AuditLog` linked to `UserID`.
  - **Account Lockout → Login Restriction**: 5 consecutive failed passwords lock account until `LockedUntil` timestamp expires.

### Tier 4: Real-World Application Workflows
- End-to-end multi-step application scenarios mimicking real operational workflows:
  1. **Academic Year Setup Workflow**: Create year, create pokjar, add tutors & classes, bind subjects, assign teachers.
  2. **Student Bulk Enrollment Workflow**: Download template, import valid Excel dataset, add transfer student, verify roster.
  3. **Weekly Attendance Lifecycle**: Auto-generate meeting, record attendance checklist, capture PNG signature, reschedule meeting, export CSV & PDF.
  4. **Year-End Promotion & Graduation Wizard**: Setup new academic year, duplicate classes, execute mass promotion with overrides (naik, tinggal, lulus), verify multi-year student history.
  5. **Multi-Role RBAC Compliance Flow**: Execute identical workflows under Admin, Kepala Sekolah, and Guru roles; verify permission enforcement.
  6. **Multi-Year Archive Retrieval**: Access `/api/arsip` with Ganjil/Genap semester filters for past years, verify immutable data retrieval.
  7. **Security Audit Trail Verification**: Perform multi-resource mutations and verify complete audit trail in `AuditLog`.
  8. **Scheduled Cron Attendance Generation**: Configure schedule settings, trigger cron generator, verify attendance generation for all active classes.
  9. **Token Lifecycle & Refresh Rotation**: Complete login, token expiration, refresh rotation, logout, and token revocation check.
  10. **Atomic Bulk Import Failure Recovery**: Import dataset containing 1 invalid row among 20 valid rows; verify 100% rollback with zero orphaned DB entries.

---

## 5. Coverage & Verification Thresholds

| Metric | Target Threshold | Verification Command |
|---|---|---|
| **Tier 1 Feature Coverage** | 100% (14/14 features, >=5 tests each) | `go test -v ./backend/cmd/server -run TestE2E_Tier1` |
| **Tier 2 Boundary Cases** | 100% (25+ boundary scenarios) | `go test -v ./backend/cmd/server -run TestE2E_Tier2` |
| **Tier 3 Combinations** | 100% (20+ pairwise scenarios) | `go test -v ./backend/cmd/server -run TestE2E_Tier3` |
| **Tier 4 Workflows** | 100% (10 full E2E scenarios) | `go test -v ./backend/cmd/server -run TestE2E_Tier4` |
| **Backend Unit/E2E Pass Rate** | 100% (0 failing tests) | `cmd /c "go test ./..."` in `backend/cmd/server` |
| **Frontend Build Pass Rate** | 100% (0 TypeScript / Vite errors) | `cmd /c "npm run build"` in `frontend` |
