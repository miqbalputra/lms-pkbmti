# Comprehensive Execution Plan: LMS PKBM Tunas Ilmu Frontend Overhaul

## 1. Project Goal & Requirements

Redesign and overhaul the frontend UI of LMS PKBM Tunas Ilmu to strictly use **shadcn/ui design standards and primitives**, ensuring a modern, intuitive, responsive, and visually polished interface across all 16 views + Auth, while preserving 100% backend REST API functionality and maintaining 0 build/test errors.

### Key Requirements
1. **R1. Comprehensive shadcn/ui Modern UI Refactor**
   - Refactor all views using `Card`, `Button`, `Dialog`, `Table`, `Select`, `Input`, `Label`, `Badge`, `Tabs`, `Alert`, `Sheet`/Sidebar, `DropdownMenu`, `Tooltip`, `AlertDialog`.
   - Consistent design tokens (OKLCH color system, slate/zinc contrast), typography, micro-interactions, responsive sidebar.
2. **R2. User Friendliness & Ergonomics**
   - Navigation cues, empty states, loading indicators, confirmation dialogs for destructive operations, inline validations, feedback toasts.
3. **R3. Preservation of Full Functionality & Integration**
   - 100% REST API compatibility (Auth, CRUD, Canvas Signature, Excel Import, PDF Export, Mass Promotion, Cron Settings).
   - Zero errors in `npm run build` and `go test ./...`.

---

## 2. Orchestration Strategy & Dual Track Architecture

We employ the **Project Pattern** with **Dual Tracks**:
- **Track 1: Implementation Track** - Modular overhaul across 6 core implementation milestones.
- **Track 2: E2E Testing Track** - Requirement-driven opaque-box test suite creation (Tiers 1-4) publishing `TEST_READY.md`.

Each milestone will be executed using the **Explorer -> Worker -> Reviewer -> Challenger -> Forensic Auditor** subagent cycle.

---

## 3. Detailed Milestone Roadmap

### Milestone 0: E2E Testing Track (Parallel)
- **Scope**: Build requirement-driven opaque-box test runner & test suites for LMS features (Tiers 1-4).
- **Deliverable**: `TEST_READY.md` & test scripts in `frontend/e2e` or root.

### Milestone 1: Core Design System, Sidebar Layout, & Auth / Login
- **Scope**:
  - Add missing shadcn/ui primitives (`sheet.tsx`, `dropdown-menu.tsx`, `tooltip.tsx`, `tabs.tsx`, `alert-dialog.tsx`, `toast`/`sonner`).
  - Overhaul root layout (`App.tsx`), sidebar navigation (collapsable mobile sheet + desktop sidebar), user profile menu, header.
  - Overhaul Auth / Login page with modern card, input fields, Turnstile widget container, and inline error handling.

### Milestone 2: Dashboard & Master Data Views
- **Scope**:
  - `DashboardCharts.tsx` & `App.tsx` Dashboard view (KPI stats cards, lazy charts, quick actions).
  - `MasterData.tsx` generic CRUD interface (Tutor, Orang Tua, Pokjar, Tahun Ajaran, Mapel) with search, pagination, modal forms, and confirmation dialogs.

### Milestone 3: Operational Classes & Subject Assignment Views
- **Scope**:
  - `ClassesView` in `OperationalViews.tsx` (Rombel cards/table, wali status badges, action triggers).
  - `ClassEditor.tsx` (Wali assignment dialog with validation) & `ClassHistory.tsx` (Historical wali assignments modal table).
  - `ClassSubjects.tsx` (Subject allocation checkbox matrix with save feedback).
  - `AssignmentsView` (Penugasan Guru per mapel/kelas, bulk assignment dialog).

### Milestone 4: Student Management & Bulk Import
- **Scope**:
  - `StudentsView` in `OperationalViews.tsx` (Student directory, search/filter by rombel, status badges).
  - `StudentEditor.tsx` (Student add/edit form with tabs for profile/parent details).
  - `StudentImport.tsx` (XLSX drag-and-drop import modal, template download button, atomic error list display).

### Milestone 5: Attendance Workspace & Promotion Wizard
- **Scope**:
  - `Attendance.tsx` (Attendance list, meeting state tabs, HTML5 canvas digital signature pad refactor, PDF/CSV export buttons).
  - `AttendanceRecap.tsx` (Semester recap table & PDF report download trigger).
  - `Promotion.tsx` (Batch promotion wizard with step indicator, rombel auto-suggestion, mass action selectors).

### Milestone 6: Administrative Views
- **Scope**:
  - `Accounts.tsx` (User account table, role badges, create/edit user modal, link to tutor dropdown).
  - `ScheduleSettings.tsx` (Automated cron schedule settings form, day/time pickers, save toast).
  - `AuditLogs.tsx` (System audit log table with action/resource filter dropdowns, timestamp formatting).
  - `ArchiveView` in `OperationalViews.tsx` (Historical academic year & attendance archive explorer).

### Milestone 7: Final E2E Suite Execution & Tier 5 Adversarial Hardening
- **Phase 1**: Execute E2E Test Suite (Tiers 1-4) against completed implementation until 100% pass rate achieved.
- **Phase 2**: Tier 5 White-Box Adversarial Hardening (Challenger-driven gap analysis & edge case fixes).
- **Verification**: Forensic Audit & victory claim to Sentinel.

---

## 4. Verification & Quality Gates

Each milestone MUST pass the following gate criteria:
1. `npm run build` inside `frontend` compiles with 0 errors.
2. `go test ./...` inside `backend/cmd/server` passes with 100% success rate.
3. Reviewers confirm shadcn UI compliance, visual aesthetics, and micro-interactions.
4. Challengers verify functional behavior & REST API contracts.
5. Forensic Auditor confirms CLEAN verdict with zero hardcoding or facade implementations.
