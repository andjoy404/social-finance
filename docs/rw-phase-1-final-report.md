# RW Phase 1 Foundation & Integration — Final Report

**Date:** 2024-10-31  
**Branch:** `urena`  
**Scope:** Secure RW scoping/authorization, fix hardcoded fallback, enable multi-RW navigation, ensure dual RT+RW role contexts

---

## 1. Completed Work

### 1.1 ✅ Dynamic RT Fallback (Previously Completed)
**File:** `backend/internal/auth/service.go` (~lines 150–180)

Replaced hardcoded UUID `a95c1cb5-2f67-4ea5-9197-bfec537b6d7b` with dynamic lookup. For SUPER_ADMIN users with multiple RT memberships, the resolver now queries `rts.rw_id` and prefers the RT whose RW matches one of the user's active RW memberships. Falls back to `memberships[0]` if no match.

**Key logic:**
```go
rwMemberships, err := loadRWMemberships(ctx, tx, user.ID)
if err == nil && len(rwMemberships) > 0 {
    rwIDs := make(map[string]struct{}, len(rwMemberships))
    for _, rm := range rwMemberships {
        rwIDs[rm.RWID] = struct{}{}
    }
    for i := range memberships {
        var rtRWID string
        err := tx.QueryRowContext(ctx,
            `SELECT rw_id FROM rts WHERE id = $1 LIMIT 1`,
            memberships[i].RTID,
        ).Scan(&rtRWID)
        if err == nil {
            if _, ok := rwIDs[rtRWID]; ok {
                m = &memberships[i]
                break
            }
        }
    }
}
if m == nil {
    m = &memberships[0]
}
```

---

### 1.2 ✅ Web `RwDataView` Param Extraction
**File:** `frontend/src/app/App.tsx`

**Change:** Route prop updated from `rwId=":rwId"` to no prop — `RwDataView` now safely extracts `rwId` from `useParams()`.

**Before:**
```tsx
<Route path="rw-view/:rwId/*" element={<RwDataView rwId=":rwId" />}>
```

**After:**
```tsx
<Route path="rw-view/:rwId/*" element={<RwDataView />}>
```

**Why:** `rwId=":rwId"` passed the literal string `":rwId"` to the component. With `useParams()`, the component correctly receives the actual URL parameter value.

---

### 1.3 ✅ Web Sidebar Multi-RW Navigation
**File:** `frontend/src/components/Sidebar.tsx`

**Changes:**
1. Added `useNavigate` import for future navigation needs
2. Extracted `rwMemberships` array from `user?.rwMemberships`
3. Replaced single-link "Lihat Data RW" with dropdown-style rendering for multi-RW users

**Behavior:**
- **Single RW membership:** Shows link to that RW's dashboard
- **Multiple RW memberships:** Shows parent link with `DownOutlined` indicator, followed by indented child links for each RW with name, RW ID, and jabatan
- **No RW membership:** No RW data item rendered

**UI structure:**
```
▼ Lihat Data RW (3)
  ↳ RW 01 (Ketua RW)
  ↳ RW 02 (Bendahara)
  ↳ RW 03
```

---

### 1.4 ✅ Android Bottom Navigation Horizontal Scrolling
**File:** `mobile/lib/app/navigation_shell.dart`

**Problem:** `BottomAppBar` with `Row` + `Flexible` widgets breaks/overflows when tabs exceed screen width.

**Fix:** Wrapped tab row in `SingleChildScrollView` with `scrollDirection: Axis.horizontal` and `BouncingScrollPhysics()`.

**Before:**
```dart
Row(
  mainAxisAlignment: MainAxisAlignment.spaceAround,
  children: List.generate(tabs.length, (index) => Flexible(child: ...)),
)
```

**After:**
```dart
SingleChildScrollView(
  scrollDirection: Axis.horizontal,
  physics: const BouncingScrollPhysics(),
  child: Row(
    mainAxisAlignment: MainAxisAlignment.start,
    children: List.generate(tabs.length, (index) => Padding(...)),
  ),
)
```

**Why:** 
- `mainAxisAlignment: MainAxisAlignment.start` — prevents `Flexible` from forcing equal distribution
- `Padding` instead of `Flexible` — allows tabs to wrap naturally
- `BouncingScrollPhysics` — smooth iOS-style scroll with bounce

---

### 1.5 ✅ Android RW Route Match & Navigation Shell
**File:** `mobile/lib/app/navigation_shell.dart`

**Problem:** `_resolveCurrentMatch` and `_isDirectlyOnTopLevelTab` didn't account for RW routes (`/rw/:rwId/*`), so the Data RW tab wouldn't highlight when on an RW screen.

**Fixes:**
1. **`_resolveCurrentMatch`:** Added check — if location starts with `/rw/` and we're not on a `/home/*` tab, match the RW data tab
2. **`_isDirectlyOnTopLevelTab`:** Added early return for `/rw/*` paths so system back correctly exits RW context

---

## 2. Verified (No Changes Needed)

### 2.1 Backend Scoping Authorization
**File:** `backend/internal/auth/scoping.go`

Verified that:
- `RequireRWScope` middleware correctly validates RW memberships from JWT claims
- `hasRWScopePermission` properly defines RW jabatan permissions
- `LoadScoping` enforces `SCOPED_RT_IDS` and `SCOPED_RW_IDS` as mandatory (fail-closed)
- `extractRWIDFromPath` correctly parses `/api/v1/rw/{rwId}/...` patterns

### 2.2 Android Routes
**File:** `mobile/lib/app/app.dart`

Verified that RW routes are already correctly registered:
- `/rw/:rwId/dashboard` → `RwDashboardScreen`
- `/rw/:rwId/kas` → `RwKasScreen`
- `/rw/:rwId/lokasi-dana` → `RwFundLocationScreen`

The Data RW tab in `navigation_shell.dart` correctly navigates to these routes after RW selection.

### 2.3 JWT/Claims Flow
**File:** `backend/internal/auth/service.go` + `middleware.go`

Verified:
- `loadRWMemberships()` returns `[]RWMembership` with fields `RWID`, `Name`, `Jabatan`
- `buildRWContextJSON` serializes RW memberships into JWT `rw_context` claim
- JWT middleware deserializes `RWContext` into `AuthContext.RWMemberships`
- `/api/v1/auth/me` returns `rw_memberships` in response body

### 2.4 AuthUser Structure
**File:** `frontend/src/app/AuthContext.tsx`

Verified:
- `AuthUser.rwMemberships` is `RWMembership[]` with `rw_id`, `rw_name`, `jabatan`
- `AuthUser.rwId` is a single value (legacy, kept for backwards compatibility)
- `mapUser()` correctly extracts `rw_memberships` from API response

---

## 3. Remaining Open Items (Phase 2+)

| Item | Priority | Notes |
|------|----------|-------|
| Android RW screens share bottom nav state | Medium | RW routes are `ShellRoute`-child `GoRoute`s; bottom nav context may need explicit passing for RW-specific tab highlighting |
| RW dashboard data API integration | Low | UI exists but needs backend `/api/v1/rw/{rwId}/beranda` endpoint |
| RW KAS data API integration | Low | UI exists but needs backend `/api/v1/rw/{rwId}/kas` endpoint |
| RW Fund Location data API integration | Low | UI exists but needs backend `/api/v1/rw/{rwId}/lokasi-dana` endpoint |
| Web Sidebar collapsed state with RW dropdown | Low | When sidebar is collapsed, RW child links are hidden; consider showing RW count badge |
| Unit tests for RW param extraction | Low | Add tests for `RwDataView` with various URL patterns |

---

## 4. Commit Instructions

### Staging (verify before committing)
```bash
cd /Users/Andriyan/Documents/apx/devl/social-finance
git status
```

### Commit
```bash
git add -A
git commit -m "feat: RW Phase 1 — dynamic RT fallback, multi-RW web/Android navigation, scoping verification

- Replace hardcoded UUID in service.go with dynamic RT lookup via loadRWMemberships
- Fix RwDataView param extraction (useParams instead of prop)
- Add multi-RW dropdown in Web Sidebar for rwMemberships array
- Fix Android bottom nav overflow with SingleChildScrollView
- Add RW route matching in Android navigation shell
- Verify RequireRWScope middleware and hasRWScopePermission logic
- All changes on urena branch, no branch creation

Co-authored-by: Chatbox <chatbox@chatboxai.com>"
```

### Push (if explicitly requested)
```bash
git push origin urena
```

---

## 5. Files Modified

| File | Change Type | Lines Changed |
|------|-------------|---------------|
| `backend/internal/auth/service.go` | Previously edited | ~30 lines (dynamic RT fallback) |
| `frontend/src/app/App.tsx` | Edited | 3 lines (remove `rwId` prop) |
| `frontend/src/components/Sidebar.tsx` | Edited | ~50 lines (multi-RW dropdown) |
| `mobile/lib/app/navigation_shell.dart` | Edited | ~15 lines (scroll + RW route match) |
| `docs/rw-phase-1-final-report.md` | Created | This file |

---

## 6. Architecture Summary

```
┌─────────────────────────────────────────────────────┐
│                  JWT Claims Flow                     │
│                                                     │
│  Login → loadRWMemberships()                        │
│        → buildRWContextJSON()                       │
│        → JWT { rw_context: [...] }                  │
│                                                     │
│  /api/v1/rw/:rwId/... → RequireRWScope middleware   │
│        → validate JWT RWMemberships                 │
│        → check SCOPED_RW_IDS                        │
│        → verify DB is_active                        │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│                  Multi-RW Navigation                 │
│                                                     │
│  Web: Sidebar → rwMemberships array → dropdown      │
│  Android: Bottom nav → selection dialog → /rw/:id   │
│  Both: Route → RwDataView → selectedRwId state      │
└─────────────────────────────────────────────────────┘
```
