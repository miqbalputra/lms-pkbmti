# Technical Analysis & Architectural Plan: Milestone 1 — Core Design System, Sidebar Layout, & Auth / Login View

**Project**: LMS PKBM Tunas Ilmu  
**Module**: Milestone 1 (Design System, App Shell, & Auth)  
**Author**: Explorer Agent (`teamwork_preview_explorer_m1_1`)  
**Date**: August 2, 2026  
**Status**: Completed Analysis & Blueprint  

---

## 1. Executive Summary

Milestone 1 establishes the foundational user interface and user experience of **LMS PKBM Tunas Ilmu**. The objective is to replace the current basic/prototype layout and inline CSS definitions with a production-grade **shadcn/ui** design system using **Tailwind v4** and **OKLCH** color tokens.

### Key Objectives:
1. **Design System & Tokens**: Clean up legacy CSS rules in `index.css` & `App.css`, establishing a unified OKLCH Slate/Zinc semantic color palette with Tailwind v4 `@theme inline` mappings.
2. **Primitive Component Expansion**: Introduce 6 missing primitive components aligned with Radix UI (`sheet.tsx`, `dropdown-menu.tsx`, `tooltip.tsx`, `tabs.tsx`, `alert-dialog.tsx`, `sonner.tsx`) and Turnstile integration.
3. **Responsive Collapsible Navigation**: Implement a modern Sidebar navigation with desktop collapsible mode (`w-64` expand / `w-16` collapse with Tooltips), mobile slide-in drawer (`Sheet`), active item highlight, badge indicators, and role-based menu filtering.
4. **Modern Top Navbar**: Build a rich header containing breadcrumbs, active academic year info badge, system online indicator, user avatar dropdown menu, and an alert dialog for logout confirmation.
5. **Polished Auth / Login View**: Redesign the Login page with a split hero visual, form field icons, password toggle button, Cloudflare Turnstile container widget, clear inline error alerts, and animated loading states.

---

## 2. Code Base & Styling Audit

### 2.1 Analysis of `frontend/src/App.tsx`
- **Current State**:
  - `App.tsx` currently contains monolithic inline components: `App`, `Login`, `Workspace`, `Restricted`, `Dashboard`, and the `request` API fetch helper.
  - Sits inside a hardcoded HTML container structure (`<div className='app'> <aside> ... </aside> <main className='content'> ... </main> </div>`).
  - Navigation menu visibility calculation relies on basic ternary checks (`user.role === 'guru' ? ['dashboard', 'kelas', 'peserta-didik', 'presensi'] : pages`).
  - Lacks state management for collapsed desktop navigation, mobile drawer state, user popover/dropdowns, or modal dialogs.
  - `Login` component is minimal, lacking Cloudflare Turnstile widget integration, password visibility toggles, proper input icons, and disabled loading feedback during authentication requests.

### 2.2 Analysis of `frontend/src/index.css` & `frontend/src/App.css`
- **`index.css`**:
  - Currently configures `@theme inline` for Tailwind v4 and `:root` variables using `oklch(...)`.
  - **Issue**: Contains legacy global rules (`.app`, `.app aside`, `.brand`, `nav button`, `.account`, `.content`, `.login`, `.charts`, `@media(max-width:900px)`). These fixed layout rules (`grid-template-columns: 16rem 1fr`, `padding: 12vw`) override modern utility-first Tailwind classes and hamper responsive flexibility.
- **`App.css`**:
  - Contains 185 lines of legacy starter styles from Vite template (`.counter`, `.hero`, `#center`, `#next-steps`, `#docs`, `#spacer`, `.ticks`). None of these are used by the LMS application.

### 2.3 Existing UI Components (`frontend/src/components/ui/*`)
The existing setup includes 12 primitive files:
- `alert.tsx`
- `badge.tsx`
- `button.tsx`
- `card.tsx`
- `checkbox.tsx`
- `dialog.tsx`
- `input.tsx`
- `label.tsx`
- `page.tsx`
- `select.tsx`
- `separator.tsx`
- `table.tsx`

---

## 3. Inventory of Missing Primitive Components & Dependencies

To enable the target UI design and fully align with PRD §2.1 and §6.1, the following components and packages must be introduced:

| Component File | Base Package / Primitives | Primary Use Case in LMS |
|---|---|---|
| `sheet.tsx` | `@radix-ui/react-dialog` *(already installed)* | Mobile responsive navigation drawer |
| `dropdown-menu.tsx` | `@radix-ui/react-dropdown-menu` | Header user menu, role switch info, table quick actions |
| `tooltip.tsx` | `@radix-ui/react-tooltip` | Desktop collapsed sidebar icon hints, header action tooltips |
| `tabs.tsx` | `@radix-ui/react-tabs` | Tabbed navigation in workspaces (Master Data, Settings, Reports) |
| `alert-dialog.tsx` | `@radix-ui/react-alert-dialog` | Logout confirmation modal, destructive deletion dialogs |
| `sonner.tsx` | `sonner` | Global toast notification feedback system |
| `turnstile.tsx` | `react-turnstile` or native iframe wrapper | Cloudflare Turnstile CAPTCHA container in Login view |

### Package Dependencies to Install / Configure:
```json
{
  "dependencies": {
    "@radix-ui/react-alert-dialog": "^1.1.6",
    "@radix-ui/react-dropdown-menu": "^2.1.6",
    "@radix-ui/react-tabs": "^1.1.3",
    "@radix-ui/react-tooltip": "^1.1.8",
    "react-turnstile": "^1.1.4",
    "sonner": "^2.0.1"
  }
}
```

---

## 4. Architectural Blueprints for Root Layout Overhaul

### 4.1 Theme Token & CSS Standardization (`index.css` & `App.css`)

#### Revised `index.css` OKLCH Slate-Zinc Palette
```css
@import "tailwindcss";

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-sidebar: var(--sidebar-background);
  --color-sidebar-foreground: var(--sidebar-foreground);
  --color-sidebar-primary: var(--sidebar-primary);
  --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);
  --color-sidebar-accent: var(--sidebar-accent);
  --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
  --color-sidebar-border: var(--sidebar-border);
}

:root {
  --background: oklch(0.99 0.002 240);
  --foreground: oklch(0.20 0.02 240);
  --card: oklch(1 0 0);
  --card-foreground: oklch(0.20 0.02 240);
  --popover: oklch(1 0 0);
  --popover-foreground: oklch(0.20 0.02 240);
  --primary: oklch(0.38 0.09 155);
  --primary-foreground: oklch(0.98 0.005 155);
  --secondary: oklch(0.95 0.02 155);
  --secondary-foreground: oklch(0.25 0.06 155);
  --muted: oklch(0.96 0.005 240);
  --muted-foreground: oklch(0.48 0.02 240);
  --accent: oklch(0.94 0.03 155);
  --accent-foreground: oklch(0.25 0.06 155);
  --destructive: oklch(0.57 0.20 27);
  --destructive-foreground: oklch(0.98 0.01 27);
  --border: oklch(0.91 0.01 240);
  --input: oklch(0.91 0.01 240);
  --ring: oklch(0.38 0.09 155);

  --sidebar-background: oklch(1 0 0);
  --sidebar-foreground: oklch(0.25 0.02 240);
  --sidebar-primary: oklch(0.38 0.09 155);
  --sidebar-primary-foreground: oklch(0.98 0.005 155);
  --sidebar-accent: oklch(0.95 0.02 155);
  --sidebar-accent-foreground: oklch(0.25 0.06 155);
  --sidebar-border: oklch(0.91 0.01 240);
}

* { border-color: var(--border); }
body {
  margin: 0;
  min-width: 320px;
  background-color: var(--background);
  color: var(--foreground);
  font-family: Inter, ui-sans-serif, system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}
```

*Action*: Purge fixed `.app` layout rules from `index.css` and clear `App.css`.

---

### 4.2 Collapsible Responsive Sidebar Layout Architecture

#### Structural Component: `Sidebar.tsx` / App Shell
- **Desktop Mode (`md:flex`)**:
  - Dynamic width transition: `collapsed ? 'w-16' : 'w-64'` (with `transition-all duration-300`).
  - Collapse Trigger: Button in header or bottom sidebar with `PanelLeftClose` / `PanelLeftOpen` icon.
  - When collapsed:
    - Logo hides name label, showing only the badge icon ("TI").
    - Navigation menu items display icon-only.
    - Wrapped with `<Tooltip>` showing page title on hover.
  - When expanded:
    - Logo displays full brand title "PKBM TUNAS ILMU" & subtitle.
    - Navigation menu items display Icon + Text + Active Indicator pill.
- **Mobile Mode (`Sheet`)**:
  - Menu icon in top bar opens `<Sheet side="left">`.
  - Contains brand header, full scrollable navigation list, and active role info.
  - Automatically closes upon navigation item click.

#### Menu Item Filtering & Categorization:
```typescript
interface NavItem {
  id: string;
  label: string;
  icon: LucideIcon;
  roles: ('admin' | 'kepala_sekolah' | 'guru')[];
  category?: string;
}

const navItems: NavItem[] = [
  { id: 'dashboard', label: 'Ringkasan', icon: LayoutDashboard, roles: ['admin', 'kepala_sekolah', 'guru'] },
  { id: 'kelas', label: 'Kelas', icon: School, roles: ['admin', 'kepala_sekolah', 'guru'] },
  { id: 'peserta-didik', label: 'Peserta Didik', icon: GraduationCap, roles: ['admin', 'kepala_sekolah', 'guru'] },
  { id: 'presensi', label: 'Presensi', icon: ClipboardCheck, roles: ['admin', 'kepala_sekolah', 'guru'] },
  { id: 'tutor', label: 'Tutor', icon: Users, roles: ['admin', 'kepala_sekolah'] },
  { id: 'orang-tua', label: 'Orang Tua', icon: Users, roles: ['admin', 'kepala_sekolah'] },
  { id: 'pokjar', label: 'Pokjar', icon: Building2, roles: ['admin', 'kepala_sekolah'] },
  { id: 'tahun-ajaran', label: 'Tahun Ajaran', icon: CalendarDays, roles: ['admin', 'kepala_sekolah'] },
  { id: 'mapel', label: 'Mata Pelajaran', icon: BookOpen, roles: ['admin', 'kepala_sekolah'] },
  { id: 'kelas-mapel', label: 'Mapel per Kelas', icon: BookOpen, roles: ['admin', 'kepala_sekolah'] },
  { id: 'penugasan', label: 'Penugasan Guru', icon: UserCog, roles: ['admin', 'kepala_sekolah'] },
  { id: 'kenaikan-kelas', label: 'Kenaikan Kelas', icon: GraduationCap, roles: ['admin', 'kepala_sekolah'] },
  { id: 'akun', label: 'Manajemen Akun', icon: UserCog, roles: ['admin', 'kepala_sekolah'] },
  { id: 'pengaturan-jadwal', label: 'Pengaturan Jadwal', icon: Settings, roles: ['admin', 'kepala_sekolah'] },
  { id: 'audit-log', label: 'Audit Log', icon: ShieldCheck, roles: ['admin', 'kepala_sekolah'] },
  { id: 'arsip', label: 'Arsip Data', icon: ArchiveIcon, roles: ['admin', 'kepala_sekolah'] },
];
```

---

### 4.3 Modern Header / Navbar Specification

- **Left Section**:
  - Hamburger toggle button (`Menu`) for mobile drawer trigger.
  - Collapse toggle button (`PanelLeft`) for desktop sidebar.
  - Breadcrumb / Active Page Indicator: `Ringkasan / PKBM Tunas Ilmu`.
  - Active Academic Year Badge: `<Badge variant="outline" className="bg-primary/5 text-primary border-primary/20">T.A. 2026/2027 • Ganjil</Badge>`.
- **Right Section**:
  - Online Status Pill (`Sistem Aktif` with glowing emerald dot).
  - Quick action trigger buttons (e.g. Refresh data).
  - **User Dropdown Menu (`<DropdownMenu>`)**:
    - Trigger: Avatar badge displaying user initials (e.g., `AD` for Admin) + Username + `ChevronDown`.
    - Dropdown Content:
      - Header label: Signed in user email/username.
      - Role Badge: `ADMINISTRATOR`, `KEPALA SEKOLAH`, or `GURU (WALI KELAS)`.
      - Separator.
      - Action Item: `Pengaturan Akun` (Navigates to `akun` page if Admin).
      - Separator.
      - Destructive Action Item: `Keluar` (Triggers `AlertDialog`).
- **Logout Confirmation Modal (`<AlertDialog>`)**:
  - Title: `Konfirmasi Keluar`
  - Description: `Apakah Anda yakin ingin keluar dari Sistem Informasi LMS PKBM Tunas Ilmu? Sesi Anda akan diakhiri.`
  - Actions: `Batal` (Cancel) and `Ya, Keluar` (Triggers POST `/auth/logout` API request, clears state).

---

### 4.4 Auth / Login View Overhaul Specification

- **Visual Layout**: Responsive 2-column hero split grid (`lg:grid-cols-2 min-h-screen`).
  - **Left Hero Banner**:
    - Background: `bg-primary` with subtle dark overlay pattern.
    - Content: Brand seal icon, main heading "Ruang kerja pendidikan yang teratur.", description of features (Presensi Sabtu, Tanda Tangan Tutor, Management Rombel, Multi-Pokjar).
    - Footer info: PKBM Tunas Ilmu © 2026.
  - **Right Form Container**:
    - Centered form container with max-w-md `Card`.
    - Card Header: Brand seal, `CardTitle` ("Masuk ke LMS PKBM Tunas Ilmu"), `CardDescription` ("Gunakan akun institusi Anda untuk melanjutkan.").
    - Card Content:
      - **Field 1 (Username/Email)**: `<Label>`, `<Input>` with leading `User` icon.
      - **Field 2 (Password)**: `<Label>`, `<Input type={showPassword ? 'text' : 'password'}>` with leading `Lock` icon and trailing `Eye` / `EyeOff` button.
      - **Field 3 (Cloudflare Turnstile Widget)**:
        - `<div className="my-3 flex justify-center">` container.
        - Loads `react-turnstile` component with `sitekey={import.meta.env.VITE_TURNSTILE_SITE_KEY || '1x00000000000000000000AA'}`.
        - Stores `turnstileToken` state, passed with submit payload.
      - **Inline Feedback Alert**: `<Alert variant="destructive">` displaying login errors if present.
      - **Submit Button**: `<Button className="w-full" disabled={loading}>` with `Loader2` spin animation during API request.
      - **Developer Helper Note**: Collapsible or subtle helper text with demo accounts (`admin / Admin123`, `kepala / Kepala123`, `guru / Guru123`).

---

## 5. Implementation Roadmap for Implementer

| Step | Action Item | Target Files | Verification Method |
|---|---|---|---|
| 1 | Install missing Radix & Sonner UI dependencies | `frontend/package.json` | `npm run build` |
| 2 | Add missing primitives (`sheet.tsx`, `dropdown-menu.tsx`, `tooltip.tsx`, `tabs.tsx`, `alert-dialog.tsx`, `sonner.tsx`) | `frontend/src/components/ui/*` | TypeScript compilation |
| 3 | Overhaul CSS variables & purge legacy rules | `frontend/src/index.css`, `frontend/src/App.css` | Verify layout responsiveness |
| 4 | Create modular Layout components (`AppHeader.tsx`, `AppSidebar.tsx`, `AppShell.tsx`) | `frontend/src/components/layout/*` | Test sidebar collapse & sheet drawer |
| 5 | Redesign Login view component with Turnstile & password toggle | `frontend/src/pages/Login.tsx` or `App.tsx` | Test auth state & Turnstile container |
| 6 | Integrate `<Toaster />` and `<AlertDialog>` in `App.tsx` | `frontend/src/App.tsx` | Test logout workflow & toast feedback |
| 7 | Full build & TypeScript check | `frontend` workspace | `cmd /c npm run build` |

---

## 6. Build & Verification Status

- Current frontend build state verified using `cmd /c npm run build` in `d:\Project LMS PKBM Tunas Ilmu\frontend`.
- Output: **Build succeeded in 533ms without any errors** (2392 modules transformed).
