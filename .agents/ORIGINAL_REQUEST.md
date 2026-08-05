# Original User Request

## 2026-08-02T12:50:32Z

Redesign and overhaul the frontend UI of LMS PKBM Tunas Ilmu to strictly and completely use shadcn/ui design standards and primitives, making the application user-friendly, highly intuitive, and visually polished.

Working directory: d:\Project LMS PKBM Tunas Ilmu
Integrity mode: demo

## Requirements

### R1. Comprehensive shadcn/ui Modern UI Refactor
- Overhaul all views (Dashboard, Tutor, Orang Tua, Pokjar, Tahun Ajaran, Mapel, Kelas, Kelas-Mapel, Peserta Didik, Penugasan, Presensi, Kenaikan Kelas, Akun, Pengaturan Jadwal, Audit Log, Arsip) using shadcn/ui components (`Card`, `Button`, `Dialog`, `Table`, `Select`, `Input`, `Label`, `Badge`, `Tabs`, `Alert`, `Sheet`/Sidebar, `DropdownMenu`, `Tooltip`).
- Ensure consistent design tokens (OKLCH color system, slate/zinc clean dark/light contrast), modern typography, micro-interactions, responsive sidebars, and clean spacing.

### R2. User Friendliness & Ergonomics
- Add clear navigation cues, empty states, loading indicators, confirmation dialogs for destructive actions (delete/reset), and clean feedback toasts/alerts.
- Improve form layouts with clear labels, inline validations, and structured tabbed views for complex operations (e.g., Kenaikan Kelas wizard & Master Data).

### R3. Preservation of Full Functionality & Integration
- Retain 100% of existing backend REST API integrations (Auth, CRUD, Attendance Signature Canvas, Excel Import, PDF Export, Mass Promotion, Cron Settings).
- Ensure frontend build (`npm run build`) passes cleanly with zero TypeScript or Vite errors.

## Acceptance Criteria

### UI & Styling Standards
- [ ] Every single view and interactive component uses shadcn/ui primitives.
- [ ] Mobile/tablet responsive layout with collapsable sidebar navigation.
- [ ] Cohesive color palette and typography matching shadcn/ui modern aesthetics.

### Usability & Reliability
- [ ] Destructive actions (delete user/student/class) prompt modal confirmations.
- [ ] Successful actions display clean feedback notifications; invalid inputs show helpful inline errors.
- [ ] No regression in any existing feature (Auth, Master Data, Presensi, Kenaikan Kelas, Export).

### Build Verification
- [ ] `npm run build` in `frontend` executes cleanly without errors.
- [ ] `go test ./...` in `backend/cmd/server` continues to pass cleanly.
