## 2026-08-02T13:27:06Z
You are assigned to implement Milestone 2: Dashboard & Master Data Views for LMS PKBM Tunas Ilmu.
Your working directory is: d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m2_1

Tasks:
1. Overhaul `frontend/src/DashboardCharts.tsx` & Dashboard view in `frontend/src/App.tsx`:
   - Use `Card`, `CardHeader`, `CardTitle`, `CardDescription`, `CardContent` for KPI stat cards (students, rombels, tutors, active attendance).
   - Enhance Recharts visualizations with modern tooltips, responsive containers, and OKLCH color palettes matching the design system.
   - Add empty states and loading skeletons.
2. Overhaul `frontend/src/MasterData.tsx` (handles Tutor, Orang Tua, Pokjar, Tahun Ajaran, Mapel CRUD):
   - Replace old table and forms with shadcn/ui primitives (`Table`, `TableHeader`, `TableRow`, `TableCell`, `Button`, `Input`, `Dialog`, `Select`, `Badge`, `Label`, `AlertDialog` for delete confirmation modal, `toast`/`sonner` for action feedback).
   - Implement inline search filtering, pagination/item count badges, clear empty state indicators.
   - For Tahun Ajaran: support active year status toggle badge and confirmation dialog.
3. Verification:
   - Run `npm run build` in `frontend` to verify 0 TypeScript/Vite errors.
   - Run `go test ./...` in `backend/cmd/server` to verify 0 backend test failures.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work.

Write your handoff report to `d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m2_1\handoff.md` and send a message to parent.
