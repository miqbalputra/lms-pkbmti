## 2026-08-02T13:49:53Z
You are the Worker assigned to fix a pagination boundary bug in d:\Project LMS PKBM Tunas Ilmu\frontend\src\MasterData.tsx.

Your working directory is: d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m2_fix

Problem Description:
The Challenger found that when a dataset has (for example) 11 items with 10 items per page (totalPages = 2), if the user is on page 2 (currentPage = 2) and deletes the 11th item:
- filteredRows drops to 10 items.
- totalPages recalculates to 1.
- However, currentPage remains stuck at 2.
- paginatedRows returns empty ([]), causing the table to render the empty state while the footer reads "Menampilkan 11 - 10 dari 10 data".

Required Action:
1. In frontend/src/MasterData.tsx, add reactive clamp logic (e.g. via useEffect watching totalPages and currentPage):
   ```tsx
   useEffect(() => {
     if (currentPage > totalPages) {
       setCurrentPage(totalPages)
     }
   }, [totalPages, currentPage])
   ```
2. Verify that deleting the last item on page N correctly auto-adjusts currentPage to totalPages (or 1 if totalPages is 0).
3. Run `npm run build` in d:\Project LMS PKBM Tunas Ilmu\frontend to ensure zero TypeScript or Vite errors.
4. Run `go test -count=1 ./...` in d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server to ensure all backend tests pass.
5. Record your work and build/test verification results in d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m2_fix\handoff.md.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.
