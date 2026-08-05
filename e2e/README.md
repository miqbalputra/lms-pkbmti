# E2E Test Suite — LMS PKBM Tunas Ilmu

This directory contains the opaque-box, requirement-driven 4-tier E2E testing runner and guide for **LMS PKBM Tunas Ilmu**.

## Test Architecture & Tiers

The test suite is structured into 4 distinct tiers:

- **Tier 1: Feature Coverage** (`e2e_tier1_feature_test.go`)
  - Validates full REST API contracts across all 14 system features (Auth, CRUD Tutors, Parents, Pokjars, Years, Subjects, Classes, Students, Assignments, Attendance Canvas, Promotion, Accounts, Settings, Audit Logs).
  - Minimum 5 test cases per feature.
- **Tier 2: Boundary & Corner Cases** (`e2e_tier2_boundary_test.go`)
  - Validates error handling, invalid inputs, edge values, corrupted signature payloads, role restriction guards (RBAC / IDOR), and lockout policies.
- **Tier 3: Cross-Feature Combinations** (`e2e_tier3_combination_test.go`)
  - Validates pairwise interactions and state persistence across modules (e.g. Wali update -> history tracking; class duplication -> subject binding; promotion -> archive retrieval).
- **Tier 4: Real-World Application Scenarios** (`e2e_tier4_scenario_test.go`)
  - Validates 10 complete end-to-end user workflows matching real school operational cycles.

## Execution Commands

### 1. Run Complete E2E Test Suite via Go
```powershell
cd backend/cmd/server
go test -v ./...
```

### 2. Run Specific Test Tiers
```powershell
# Tier 1: Feature Coverage
go test -v ./backend/cmd/server -run TestE2E_Tier1

# Tier 2: Boundary Cases
go test -v ./backend/cmd/server -run TestE2E_Tier2

# Tier 3: Combinations
go test -v ./backend/cmd/server -run TestE2E_Tier3

# Tier 4: Real-World Scenarios
go test -v ./backend/cmd/server -run TestE2E_Tier4
```

### 3. Run Automated Windows Script
```cmd
e2e\run_e2e.bat
```

### 4. Verify Frontend Build Integrity
```cmd
cd frontend
cmd /c npm run build
```
