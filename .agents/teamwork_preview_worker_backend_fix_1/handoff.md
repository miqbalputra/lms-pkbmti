# Handoff Report: Backend Bug Fixes for `routes.go`

## 1. Observation

- **Bug 1 (`assignAllClasses` in `backend/cmd/server/routes.go`)**:
  - Code at `routes.go:441` previously executed:
    `result := tx.Where(PenugasanGuruMapel{TutorID: assignment.TutorID, KelasID: assignment.KelasID, MapelID: assignment.MapelID}).FirstOrCreate(&assignment)`
  - Because `assignment` is initialized and passed by reference, GORM evaluated struct primary key fields and failed to match existing `(tutor_id, kelas_id, mapel_id)` records properly, causing duplicate insert attempts that violated the UNIQUE constraint (`gorm:"uniqueIndex:penugasan"`).

- **Bug 2 (`setKelasMapel` in `backend/cmd/server/routes.go`)**:
  - Code at `routes.go:410` previously executed:
    `s.audit(&uid, "update", "kelas_mapel", id(c))`
    inside an open GORM transaction callback `s.db.Transaction(func(tx *gorm.DB) error { ... })`.
  - `s.audit(...)` invoked `s.db.Create(&AuditLog{...})` using `s.db` outside `tx`. On SQLite shared in-memory connection pools with single max open connection (`sqlDB.SetMaxOpenConns(1)` as configured in E2E tests), this attempted to acquire a new DB connection while `tx` held the single open connection, resulting in a database transaction deadlock.
  - A similar issue existed in `s.promote` at `routes.go:663`.

## 2. Logic Chain

- **Fix 1 (`assignAllClasses`)**:
  - By replacing the struct-based `Where` with explicit column criteria:
    `tx.Where("tutor_id = ? AND kelas_id = ? AND mapel_id = ?", in.TutorID, class.ID, in.MapelID).FirstOrCreate(&assignment)`
    GORM explicitly queries existing records using `WHERE tutor_id = ? AND kelas_id = ? AND mapel_id = ?`. If a record exists, GORM populates `assignment` without attempting a duplicate insert (`RowsAffected == 0`). If no record exists, GORM inserts a single new record (`RowsAffected == 1`).

- **Fix 2 (`setKelasMapel` & Audit Calls in Transactions)**:
  - Added method `s.auditTx(tx *gorm.DB, uid *string, a, r, d string)` in `backend/cmd/server/main.go` which executes `tx.Create(&AuditLog{...})`.
  - Updated `setKelasMapel` (and `promote`) in `routes.go` to call `s.auditTx(tx, ...)` inside open transaction blocks.
  - Because `auditTx` reuses the active transaction `tx`, no additional database connection is requested from `s.db`, completely eliminating the deadlock on SQLite shared in-memory connections.

## 3. Caveats

- SQLite in-memory mode requires `auditTx` when auditing inside open transactions to prevent connection pool exhaustion/deadlock when `MaxOpenConns` is constrained. Any new transaction handlers added in the future should also use `s.auditTx(tx, ...)` if auditing inside a transaction.

## 4. Conclusion

Both identified backend bugs have been fully fixed in `backend/cmd/server/routes.go` and `backend/cmd/server/main.go`. All unit tests, boundary tests, combination tests, and E2E scenario tests pass cleanly with zero deadlocks or UNIQUE constraint failures, and frontend build passes with 0 errors.

## 5. Verification Method

To independently verify:

1. **Run Backend Test Suites**:
   ```powershell
   cd "d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server"
   go test -v ./...
   ```
   *Expected Output*: `PASS` with `ok pkbm-lms/backend/cmd/server` (all tests passed including `TestE2E_Tier1_CRUD_Assignments`, `TestE2E_Tier1_Promotion`, and all Tier 1-4 E2E test suites).

2. **Run Frontend Build**:
   ```powershell
   cd "d:\Project LMS PKBM Tunas Ilmu\frontend"
   cmd /c npm run build
   ```
   *Expected Output*: `vite building client environment for production... built in ...` with exit code 0.
