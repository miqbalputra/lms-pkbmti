# Comprehensive Codebase Analysis: LMS PKBM Tunas Ilmu

**Date:** 2026-08-02  
**Target Project:** LMS PKBM Tunas Ilmu (`d:\Project LMS PKBM Tunas Ilmu`)  
**Investigator:** Teamwork Preview Explorer  
**Working Directory:** `d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_explorer_init_1`

---

## 1. Executive Summary

This report presents a thorough analysis of the LMS PKBM Tunas Ilmu application codebase. The system is designed as a streamlined Learning Management System (LMS) specifically tailored for PKBM (Pusat Kegiatan Belajar Masyarakat) institutions, handling student administration, rombel (study group) allocations, tutor assignments, attendance tracking with digital signatures, promotion workflows, and audit logging.

- **Frontend Tech Stack:** React 19, Vite 8, TypeScript 6, Tailwind CSS v4, Radix UI primitives, Lucide React icons, Recharts.
- **Routing Strategy:** Custom state-driven view routing in `App.tsx` (no `react-router`).
- **State Management:** Native React state (`useState`, `useEffect`) with token/permission prop drilling.
- **Backend Tech Stack:** Go 1.23, Fiber v2, GORM ORM, SQLite (dev) / PostgreSQL (prod), JWT auth with HttpOnly refresh cookies, `robfig/cron` scheduler, `gofpdf` PDF generator, `excelize` Excel processor.
- **Build Status:**  
  - Frontend (`npm run build`): **PASSED** (0 errors, 689ms build time).  
  - Backend (`go test ./...`): **PASSED** (7/7 test suites passed in 7.136s).

---

## 2. Frontend Architecture & Infrastructure

### 2.1 Technology Stack & Dependencies (`frontend/package.json`)

- **Core Libraries:**
  - `react` (`^19.2.8`), `react-dom` (`^19.2.8`)
  - `vite` (`^8.2.0`), `@vitejs/plugin-react` (`^6.0.4`)
  - `typescript` (`~6.0.2`)
- **Styling & Components:**
  - `tailwindcss` (`^4.3.3`), `@tailwindcss/vite` (`^4.3.3`)
  - `@radix-ui/react-dialog` (`^1.1.23`), `@radix-ui/react-slot` (`^1.3.3`)
  - `lucide-react` (`^1.28.0`)
  - `class-variance-authority` (`^0.7.1`), `clsx` (`^2.1.1`), `tailwind-merge` (`^3.6.0`)
- **Data Visualization:**
  - `recharts` (`^2.15.1`)
- **Linting & Tooling:**
  - `oxlint` (`^1.75.0`)

*Note: `react-router` and `react-router-dom` are absent from `package.json`.*

### 2.2 Directory Structure (`frontend/src`)

```
frontend/src/
├── App.css
├── App.tsx                    # Main layout, state routing, login, dashboard, request helper
├── Accounts.tsx               # Admin view for managing user accounts
├── Attendance.tsx             # Presensi workspace with canvas signature pad
├── AttendanceRecap.tsx        # Presensi semester recap & PDF export view
├── AuditLogs.tsx              # System audit log viewer
├── ClassEditor.tsx            # Modal form for changing class wali kelas
├── ClassHistory.tsx           # Modal table showing historical wali kelas assignments
├── ClassSubjects.tsx          # Class-to-Subject curriculum matrix configuration
├── DashboardCharts.tsx        # Recharts visualizations (Pie & Bar charts)
├── MasterData.tsx             # Generic CRUD interface for master entities
├── OperationalViews.tsx       # ClassesView, StudentsView, AssignmentsView, ArchiveView
├── Promotion.tsx              # Batch student promotion wizard
├── ScheduleSettings.tsx       # KBM attendance generator schedule configuration
├── StudentEditor.tsx          # Student creation and editing form
├── StudentImport.tsx          # Excel XLSX bulk student importer modal
├── index.css
├── main.tsx
├── lib/
│   └── utils.ts               # Tailwind class merger (`cn` utility)
└── components/
    └── ui/                    # Primitive components (shadcn pattern)
        ├── alert.tsx
        ├── badge.tsx
        ├── button.tsx
        ├── card.tsx
        ├── checkbox.tsx
        ├── dialog.tsx
        ├── input.tsx
        ├── label.tsx
        ├── page.tsx
        ├── select.tsx
        ├── separator.tsx
        └── table.tsx
```

### 2.3 Build and Configuration Files

1. **`vite.config.ts`**: Configures Vite with `@vitejs/plugin-react` and `@tailwindcss/vite`.
2. **`tsconfig.app.json`**: ES2023 target, bundler module resolution, `react-jsx` transform, strict unused variable checks.
3. **`index.html`**: Entry point referencing `/src/main.tsx`.

---

## 3. Frontend Pages, Views, & Component Implementations

The application supports 16 distinct pages/views, mapped via a central switcher component (`Workspace` in `App.tsx`):

| Page Key | Indonesian Label | Responsible Component | Implementation Details | Access Level |
| :--- | :--- | :--- | :--- | :--- |
| `dashboard` | Ringkasan | `Dashboard` (`App.tsx`) + `DashboardCharts.tsx` | Summary stat cards (students, rombels, attendance) + lazy-loaded Recharts donut/bar charts. | All Roles |
| `tutor` | Tutor | `MasterData` (`MasterData.tsx`) | CRUD table & form schema for Tutor entities (nama, jenis kelamin, HP, alamat). | All (Admin: RW, Guru/KepSek: R) |
| `orang-tua` | Orang Tua | `MasterData` (`MasterData.tsx`) | CRUD table & form schema for Parent entities (nama bapak, nama ibu). | All (Admin: RW, Guru/KepSek: R) |
| `pokjar` | Pokjar | `MasterData` (`MasterData.tsx`) | CRUD table & form schema for Pokjar branches (pusat/binaan). | All (Admin: RW, Guru/KepSek: R) |
| `tahun-ajaran`| Tahun Ajaran | `MasterData` (`MasterData.tsx`) | CRUD table & form schema for Academic Years (start/end dates, active flag). | All (Admin: RW, Guru/KepSek: R) |
| `mapel` | Mata Pelajaran| `MasterData` (`MasterData.tsx`) | CRUD table & form schema for Subjects (nama mapel, kode mapel). | All (Admin: RW, Guru/KepSek: R) |
| `kelas` | Kelas | `ClassesView` (`OperationalViews.tsx`) | Rombel list, create rombel, open `ClassEditor` (change wali kelas), open `ClassHistory`. | Guru (filtered), Admin/KepSek |
| `kelas-mapel` | Mapel per Kelas| `ClassSubjects.tsx` | Checkbox matrix to assign subjects (`MataPelajaran`) to a specific `Kelas`. | Admin Only |
| `peserta-didik`| Peserta Didik | `StudentsView` (`OperationalViews.tsx`) | Student table, NIS/NISN, open `StudentEditor` form, open `StudentImport` modal. | Guru (filtered), Admin/KepSek |
| `penugasan` | Penugasan Guru| `AssignmentsView` (`OperationalViews.tsx`) | Assign tutor to class & subject. Supports bulk assignment across all matching classes. | All (Admin: RW, Guru/KepSek: R) |
| `presensi` | Presensi | `AttendanceWorkspace` (`Attendance.tsx`) | Record meeting attendance, status (berlangsung/libur/dipindah), HTML5 canvas signature pad, CSV export, PDF export, semester recap table (`AttendanceRecap`). | Guru (assigned wali), Admin, KepSek (R) |
| `kenaikan-kelas`| Kenaikan Kelas| `PromotionWizard` (`Promotion.tsx`) | Select source rombel & target year. Auto-suggests next grade rombel, handles individual decisions (naik/tinggal/lulus/pindah/keluar). | Admin Only |
| `akun` | Akun | `Accounts` (`Accounts.tsx`) | Create/update user accounts, set role (guru/admin/kepala_sekolah), link guru to tutor, activate/deactivate account. | Admin Only |
| `pengaturan-jadwal`| Pengaturan Jadwal| `ScheduleSettings.tsx` | Set automated weekly attendance generation day (Senin-Minggu) & time (WIB). | Admin Only |
| `audit-log` | Audit Log | `AuditLogs.tsx` | View latest 250 audit log entries. Filter by action and resource module. | Admin Only |
| `arsip` | Arsip | `ArchiveView` (`OperationalViews.tsx`) | View historical student class enrollment & attendance meeting counts by academic year & semester. | Admin Only |
| *Auth* | Login | `Login` (`App.tsx`) | Institution portal login form. Handles JWT token issuance & error alerts. | Unauthenticated |

---

## 4. Frontend Component Patterns, State Management, & API Communication

### 4.1 Routing & Navigation Pattern
- Custom state management using `useState('dashboard')` inside `App.tsx`.
- Sidebar navigation dynamically filters visible pages based on user role:
  - Role `guru`: restricted to `['dashboard', 'kelas', 'peserta-didik', 'presensi']`.
  - Roles `admin` / `kepala_sekolah`: all 16 pages visible.
- Restricted views display a standard alert (`<Restricted />`) if accessed by unauthorized roles.

### 4.2 State Management Pattern
- No global state container (Redux/Zustand) or context provider is used.
- Local state handles form inputs, modal visibility, loading states, and API responses.
- Authentication credentials (`token`, `user`) are stored at root level (`App.tsx`) and passed down via props.

### 4.3 API Request Layer
- Standardized fetch wrapper function (`request`) declared across component files:
  ```ts
  async function request(path: string, token: string, method = 'GET', body?: unknown)
  ```
- Features:
  - Automatic `apiBase` resolution (`import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080/api'`).
  - Sets `credentials: 'include'` for HttpOnly refresh cookies.
  - Injects `Authorization: Bearer <token>` header.
  - Parses JSON errors and handles 204 No Content responses.

---

## 5. Backend Architecture & REST Endpoints (`backend/cmd/server`)

### 5.1 Architecture & Database Design
- Framework: Go 1.23 + Fiber v2.
- Database: GORM ORM supporting SQLite (`pkbm-lms.db` for local dev/testing) and PostgreSQL (`DATABASE_URL` in production).
- Models (`main.go`):
  - `User`, `RefreshToken`, `AuditLog`
  - `Tutor`, `OrangTua`, `Pokjar`, `TahunAjaran`, `Kelas`, `RiwayatWaliKelas`
  - `MataPelajaran`, `KelasMapel`, `PenugasanGuruMapel`
  - `PesertaDidik`, `RiwayatKelasPesertaDidik`
  - `PengaturanJadwal`, `Presensi`, `PresensiDetail`

### 5.2 Security & Authentication Mechanism
- **Password Security:** Hashed using bcrypt with default cost.
- **Login Rate Limiting & Account Lockout:** Max 5 attempts per minute. Account locks for 15 minutes after 5 consecutive failures. Cloudflare Turnstile support in production.
- **Dual-Token JWT Auth:**
  - Access Token: Short-lived (15 minutes), passed in Bearer header.
  - Refresh Token: Long-lived (7 days), stored in HttpOnly, SameSite=Strict cookie. Has token hash rotation and reuse detection.
- **Role-Based Access Control (RBAC) Middleware:**
  - `s.auth`: Verifies JWT access token.
  - `s.admin`: Restricts route to `admin` role.
  - `s.managementRead`: Allows `admin` and `kepala_sekolah`.
  - `s.writable`: Blocks `kepala_sekolah` from write operations.
  - `s.canManageKelas`: Restricts `guru` role to rombels where they are assigned as `waliKelas`.

### 5.3 Background Scheduler
- Utilizes `github.com/robfig/cron/v3` set to `Asia/Jakarta` timezone.
- Runs every minute to evaluate `PengaturanJadwal` settings. Automatically creates weekly `Presensi` meeting records for active classes with an assigned `waliKelas`.

### 5.4 Backend REST Endpoints Summary (`cmd/server/routes.go`)

| Method | Endpoint Path | Description | Access Control |
| :--- | :--- | :--- | :--- |
| `POST` | `/api/auth/login` | User login, returns access token + sets refresh cookie | Public (Rate limited) |
| `POST` | `/api/auth/refresh` | Rotates refresh token & issues new access token | Public (Rate limited) |
| `POST` | `/api/auth/logout` | Revokes refresh token & clears cookie | Authenticated |
| `GET` | `/api/auth/me` | Returns current user profile | Authenticated |
| `GET` | `/api/dashboard` | Dashboard metrics (student/class/attendance counts & breakdowns) | Authenticated (Scope-filtered) |
| `GET` | `/api/tutor`, `/api/tutor/:id` | List / get tutors | Management Read |
| `POST/PUT/DEL`| `/api/tutor` | Tutor CRUD operations | Admin Only |
| `GET` | `/api/orang-tua` | List parents | Management Read |
| `POST/PUT/DEL`| `/api/orang-tua` | Parent CRUD operations | Admin Only |
| `GET` | `/api/pokjar` | List pokjars | Management Read |
| `POST/PUT/DEL`| `/api/pokjar` | Pokjar CRUD operations | Admin Only |
| `GET` | `/api/tahun-ajaran` | List academic years | Management Read |
| `POST/PUT` | `/api/tahun-ajaran` | Create/update academic year (handles active status toggle) | Admin Only |
| `GET` | `/api/mapel` | List subjects | Management Read |
| `POST/PUT/DEL`| `/api/mapel` | Subject CRUD operations | Admin Only |
| `GET` | `/api/users` | List user accounts | Management Read |
| `POST/PUT/DEL`| `/api/users` | Account CRUD operations | Admin Only |
| `GET` | `/api/kelas` | List rombels | Authenticated (Guru filtered) |
| `GET` | `/api/kelas/:id/riwayat-wali` | List historical wali kelas assignments for class | Authenticated |
| `POST/PUT/DEL`| `/api/kelas` | Rombel CRUD operations (tracks wali history automatically) | Admin Only |
| `POST` | `/api/kelas/duplicate` | Duplicate rombel structure for new academic year | Admin Only |
| `PUT` | `/api/kelas/:id/mapel` | Set curriculum subjects for class | Admin Only |
| `GET` | `/api/kelas-mapel` | List class-subject links | Management Read |
| `POST/DEL` | `/api/penugasan` | Create/delete teacher subject assignment | Admin Only |
| `POST` | `/api/penugasan/semua-kelas`| Bulk assign teacher to all matching classes for a subject | Admin Only |
| `GET` | `/api/peserta-didik` | List students | Authenticated (Guru filtered) |
| `POST/PUT/DEL`| `/api/peserta-didik` | Student CRUD operations | Admin Only |
| `POST` | `/api/peserta-didik/import` | Atomic bulk import of students from `.xlsx` file | Admin Only |
| `GET` | `/api/peserta-didik/template`| Download official `.xlsx` import template | Admin Only |
| `POST` | `/api/kenaikan-kelas` | Process batch student promotion/graduation | Admin Only |
| `GET/PUT` | `/api/settings/jadwal` | Get/update automated KBM schedule settings | Read: All / Update: Admin |
| `GET` | `/api/audit-logs` | Query audit log entries (action, resource filters) | Management Read |
| `GET` | `/api/arsip` | Historical academic & attendance archive | Management Read |
| `GET/POST/PUT`| `/api/presensi` | List / create / update attendance meetings | Authenticated |
| `POST` | `/api/presensi/:id/details` | Save/update student attendance detail status | Authenticated |
| `GET` | `/api/presensi/export` | Export attendance records as CSV | Authenticated |
| `GET` | `/api/presensi/rekap` | Query semester attendance summary per student | Authenticated |
| `GET` | `/api/presensi/rekap/pdf` | Generate & stream PDF report for semester recap | Authenticated |
| `GET` | `/api/presensi/:id/pdf` | Generate & stream PDF for single meeting with signature | Authenticated |

---

## 6. Build and Test Verification

### 6.1 Frontend Build Test
- **Command:** `cmd.exe /c npm run build` inside `d:\Project LMS PKBM Tunas Ilmu\frontend`
- **Result:** **SUCCESS (PASS)**
- **Output Snippet:**
  ```
  > frontend@0.0.0 build
  > tsc -b && vite build

  vite v8.2.0 building client environment for production...
  transforming...✓ 2392 modules transformed.
  rendering chunks...
  dist/index.html                            0.45 kB │ gzip:   0.29 kB
  dist/assets/index-B_BiqoEh.css            25.72 kB │ gzip:   5.76 kB
  dist/assets/index-BHeMaWKD.js            288.72 kB │ gzip:  85.86 kB
  dist/assets/DashboardCharts-kM5UN8TB.js  395.12 kB │ gzip: 102.99 kB
  ✓ built in 689ms
  ```

### 6.2 Backend Test Verification
- **Command:** `cmd.exe /c go test -v ./...` inside `d:\Project LMS PKBM Tunas Ilmu\backend\cmd\server`
- **Result:** **SUCCESS (PASS)**
- **Test Suites Breakdown:**
  1. `TestRefreshTokenRotatesAndRejectsReuse`: **PASS** (0.11s)
  2. `TestManagementReadAndWriteGuards`: **PASS** (0.00s)
  3. `TestGuruCannotManageDifferentClassAttendance`: **PASS** (0.00s)
  4. `TestImportSiswaIsAtomicWhenRowIsInvalid`: **PASS** (0.01s)
  5. `TestPromotionRejectsTargetClassFromAnotherYear`: **PASS** (0.00s)
  6. `TestUpdateKelasRecordsWaliHistory`: **PASS** (0.00s)
  7. `TestKelasCombinationMustBeUnique`: **PASS** (0.00s)
  8. `TestValidSignatureAcceptsOnlyPNGBase64`: **PASS** (0.00s)

---

## 7. Conclusions & Insights

1. **Clean & High-Quality Architecture:** The project is well-structured, combining a responsive React frontend styled with Tailwind CSS & Radix UI with a high-performance Go Fiber backend.
2. **Robust Security & Data Integrity:** Implements JWT rotation, rate-limiting, strict RBAC, audit logging, atomic transactions for Excel imports & student promotions, and unique GORM database indexes for rombels and attendance.
3. **Fully Passing Baseline:** Both frontend compilation and backend unit tests pass without any errors or warnings.
