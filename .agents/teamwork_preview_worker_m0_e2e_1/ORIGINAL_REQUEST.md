## 2026-08-02T12:54:56Z
You are assigned to own the E2E Testing Track for LMS PKBM Tunas Ilmu (d:\Project LMS PKBM Tunas Ilmu).
Your working directory is: d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m0_e2e_1

Scope & Responsibilities:
1. Design an opaque-box, requirement-driven E2E test runner and comprehensive test suite based on user requirements in `d:\Project LMS PKBM Tunas Ilmu\.agents\ORIGINAL_REQUEST.md` and `PRD.md` (NOT implementation design).
2. Follow the 4-tier methodology:
   - Tier 1: Feature Coverage (>=5 test cases per feature for Auth, CRUD Tutors/Parents/Pokjars/Years/Subjects/Classes/Students/Assignments, Attendance Canvas, Promotion, Accounts, Settings, Audit Logs).
   - Tier 2: Boundary & Corner Cases (empty inputs, zero/negative, max length, invalid formats, role restriction violations).
   - Tier 3: Cross-Feature Combinations (pairwise interactions: e.g. tutor assignment -> presensi generation; student promotion -> class history).
   - Tier 4: Real-World Application Scenarios (end-to-end user workflows).
3. Create `TEST_INFRA.md` at project root (`d:\Project LMS PKBM Tunas Ilmu\TEST_INFRA.md`) detailing the test philosophy, feature inventory, test architecture, and coverage thresholds.
4. Implement runnable E2E test scripts or integration tests (e.g. in `e2e/` or test suite) that can execute against the app/API without modifying production source logic. Verify `npm run build` in `frontend` and `go test ./...` in `backend/cmd/server` pass.
5. Create `TEST_READY.md` at project root (`d:\Project LMS PKBM Tunas Ilmu\TEST_READY.md`) summarizing the runner command and Tier 1-4 test coverage checklist once the test suite is ready.
6. Write your handoff report to `d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m0_e2e_1\handoff.md` and send a message to parent.
