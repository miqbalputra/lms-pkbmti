## 2026-08-02T12:56:15Z
You are assigned to implement Milestone 1: Core Design System, Sidebar Layout, & Auth / Login View for LMS PKBM Tunas Ilmu.
Your working directory is: d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m1_1

Read the detailed technical specifications and blueprints in:
- `d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_explorer_m1_1\analysis.md`
- `d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_explorer_m1_1\handoff.md`

Tasks:
1. Update `frontend/package.json` to add any needed dependencies (`@radix-ui/react-alert-dialog`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-tabs`, `@radix-ui/react-tooltip`, `sonner`, `react-turnstile` or native iframe widget), run install/build check.
2. Create missing primitive components in `frontend/src/components/ui/`:
   - `sheet.tsx`
   - `dropdown-menu.tsx`
   - `tooltip.tsx`
   - `tabs.tsx`
   - `alert-dialog.tsx`
   - `sonner.tsx` or toast feedback component
3. Refactor `frontend/src/index.css` and `frontend/src/App.css`:
   - Update `:root` and `@theme inline` OKLCH Slate/Zinc color tokens as specified in `analysis.md`.
   - Remove legacy fixed CSS grid/layout rules that interfere with utility classes.
4. Create modular App Shell components:
   - Collapsible desktop navigation (`w-64` expand / `w-16` collapse with tooltips), mobile drawer (`Sheet`), active tab highlight, role-based item filtering.
   - Header with active page breadcrumbs, academic year badge (`T.A. 2026/2027 • Ganjil`), online status pill, user avatar dropdown menu (`<DropdownMenu>`), and logout confirmation modal (`<AlertDialog>`).
5. Overhaul Auth / Login page component:
   - 2-column hero split grid / modern centered card layout.
   - Field icons (`User`, `Lock`), password toggle button (`Eye`/`EyeOff`).
   - Cloudflare Turnstile container widget (`turnstileToken` state).
   - Inline feedback alerts for login errors.
   - Animated loading spinner on submit button (`Loader2`).
6. Run `npm run build` in `frontend` and `go test ./...` in `backend/cmd/server`. Ensure BOTH pass cleanly with 0 errors.

MANDATORY INTEGRITY WARNING:
DO NOT CHEAT. All implementations must be genuine. DO NOT hardcode test results, create dummy/facade implementations, or circumvent the intended task. A Forensic Auditor will independently verify your work. Integrity violations WILL be detected and your work WILL be rejected.

Write your handoff report to `d:\Project LMS PKBM Tunas Ilmu\.agents\teamwork_preview_worker_m1_1\handoff.md` and send a message to parent.
