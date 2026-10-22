---
name: KAS Web Report authorization check implemented
description: KasReport.tsx added role/jabatan-based authorization gate before rendering KAS report data
type: project
category: practice_standard
keywords:
  - "KasReport.tsx authorization"
  - "canExportKasPdf"
  - "bendahara role gate"
  - "ketua jabatan gate"
  - "frontend RBAC KAS"
usage_scenarios:
  - "Audit authorization halaman KAS Web"
  - "Review RBAC guard untuk laporan keuangan"
  - "Implementasi authorization halaman baru"
---

## Authorization Rule
```typescript
const canExportKasPdf = user?.role === 'bendahara' || user?.jabatan === 'ketua'
```

## Implementation (2026-10-22)
- **File**: `frontend/src/features/kas/KasReport.tsx` (line 55–68)
- **Logic**: Early return dengan "Akses Ditolak" UI jika user tidak memenuhi kriteria
- **Allowed**: `role === 'bendahara'` ATAU `jabatan === 'ketua'`
- **Blocked**: role === 'ketua' (tanpa jabatan), warga, null user, SUPER_ADMIN tanpa jabatan
- **Pattern**: Mengikuti pattern `IuranList.tsx` dan `IuranDetail.tsx`

## Backend (Already Secure)
- `ListTransactions` handler: `getAuthContext(r)` + tenant isolation via `ac.RTID`
- `GetBalance` handler: `getAuthContext(r)` + tenant isolation via `ac.RTID`
- Backend memfilter data berdasarkan RT ID dari JWT token, BUKAN dari client
- Tidak ada漏洞 authorization di backend

## Test Coverage (8/8 PASS)
1. Bendahara + RT sesuai = ✅ boleh akses
2. Ketua RT (jabatan) + RT sesuai = ✅ boleh akses
3. role === 'ketua' tapi jabatan !== 'ketua' = ❌ ditolak
4. Warga = ❌ ditolak
5. User role/jabatan benar tapi RT beda = ✅ frontend passes, backend filter (tenant isolation)
6. User null (tidak login) = ❌ ditolak
7. SUPER_ADMIN tanpa role bendahara/jabatan ketua = ❌ ditolak
8. Bonus: role bendahara + jabatan ketua = ✅ boleh akses

## Validation
- `tsc --noEmit`: ✅ clean
- `npm run build`: ✅ clean (5.35s)
- `npm test -- --run`: ✅ 8/8 authorization tests PASS

## Important Notes
- Frontend authorization berdasarkan role/jabatan, BUKAN RT
- RT isolation enforcement dilakukan di backend via `getAuthContext(r)` dari JWT
- Jika perlu menambahkan akses ke role lain (misal RW), update `canExportKasPdf` logic
