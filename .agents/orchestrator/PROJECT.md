# Project: LMS PKBM Tunas Ilmu Frontend Overhaul

## Architecture
- **Frontend**: React 19, Vite 8, TypeScript 6, Tailwind CSS v4, Radix UI primitives, Lucide React icons, Recharts.
- **Routing**: Custom state-driven view routing in `App.tsx` (`const [page, setPage] = useState('dashboard')`).
- **Design System**: Strict shadcn/ui design standards & primitives (`Card`, `Button`, `Dialog`, `Table`, `Select`, `Input`, `Label`, `Badge`, `Tabs`, `Alert`, `Sheet`/Sidebar, `DropdownMenu`, `Tooltip`, `AlertDialog`, `Toast`/Feedback).
- **Backend Integration**: Go 1.23 Fiber v2 REST API (Auth, CRUD, Attendance canvas signatures, Excel Import, PDF Export, Mass Promotion, Cron Settings).
- **Dual Track Strategy**:
  - **Implementation Track**: Modular view overhaul preserving REST API integration.
  - **E2E Testing Track**: Requirement-driven opaque-box testing suite covering Tiers 1-4, publishing `TEST_READY.md`.

## Code Layout
- `frontend/src/App.tsx`: Main layout, state routing, login, dashboard view, fetch request helper.
- `frontend/src/components/ui/`: Primitive UI components (shadcn pattern).
- `frontend/src/MasterData.tsx`: Generic CRUD interface for Tutor, Orang Tua, Pokjar, Tahun Ajaran, Mapel.
- `frontend/src/OperationalViews.tsx`: ClassesView, StudentsView, AssignmentsView, ArchiveView.
- `frontend/src/ClassEditor.tsx` & `ClassHistory.tsx`: Class wali assignment editor & history modal.
- `frontend/src/ClassSubjects.tsx`: Class-to-Subject curriculum matrix.
- `frontend/src/StudentEditor.tsx` & `StudentImport.tsx`: Student creation/editing form & Excel import modal.
- `frontend/src/Attendance.tsx` & `AttendanceRecap.tsx`: Attendance workspace, signature pad, PDF/CSV export, recap table.
- `frontend/src/Promotion.tsx`: Batch student promotion wizard.
- `frontend/src/Accounts.tsx`: Account management & role assignments.
- `frontend/src/ScheduleSettings.tsx`: Automated KBM schedule configuration.
- `frontend/src/AuditLogs.tsx`: System audit log viewer.
- `frontend/src/DashboardCharts.tsx`: Recharts visualizations.

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| 0 | E2E Testing Track | Requirement-driven test harness (Tiers 1-4) & `TEST_READY.md` | none | DONE |
| 1 | Core Design System & Layout | primitives (`sheet`, `dropdown-menu`, `tooltip`, `tabs`, `alert-dialog`), sidebar layout, theme tokens, `App.tsx` auth/login | none | DONE |
| 2 | Dashboard & Master Data Views | `DashboardCharts.tsx`, `MasterData.tsx` (Tutor, Orang Tua, Pokjar, Tahun Ajaran, Mapel) | M1 | PLANNED |
| 3 | Operational Classes & Subjects | `ClassesView`, `AssignmentsView`, `ClassSubjects.tsx`, `ClassEditor.tsx`, `ClassHistory.tsx` | M1 | PLANNED |
| 4 | Student Management & Import | `StudentsView`, `StudentEditor.tsx`, `StudentImport.tsx` | M1 | PLANNED |
| 5 | Attendance Workspace & Promotion | `Attendance.tsx`, `AttendanceRecap.tsx`, `Promotion.tsx` | M1 | PLANNED |
| 6 | Administrative Views | `Accounts.tsx`, `ScheduleSettings.tsx`, `AuditLogs.tsx`, `ArchiveView` | M1 | PLANNED |
| 7 | Final E2E Pass & Tier 5 Hardening | 100% E2E test suite pass (Tiers 1-4) + Adversarial hardening (Tier 5) | M0 - M6 | PLANNED |

## Interface Contracts
### Frontend ↔ Backend REST API
- `POST /api/auth/login`: `{ username, password }` -> `{ token, user }` + refresh cookie
- `GET /api/auth/me`: Bearer Auth -> `{ id, username, nama_lengkap, role, tutor_id }`
- `GET /api/dashboard`: Bearer Auth -> summary stats & charts data
- `GET/POST/PUT/DELETE /api/*`: CRUD resources (tutor, orang-tua, pokjar, tahun-ajaran, mapel, users, kelas, peserta-didik, penugasan, presensi, settings/jadwal, audit-logs, arsip)
- Content Types: `application/json` for standard API, `multipart/form-data` for `/api/peserta-didik/import`, `application/pdf` for PDF endpoints.
