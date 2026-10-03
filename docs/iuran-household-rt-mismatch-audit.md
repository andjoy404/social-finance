# RT Source of Truth Audit — Final (2026-10-04)

## Problem Statement

Dua modul (Warga dan Iuran) menampilkan data yang sama tapi melalui source RT berbeda. Apakah ini menyebabkan mismatch? Audit ini melacak seluruh lifecycle RT dari create hingga display.

---

## 1. Current Data Flow — Lifecycle Lengkap

### 1.1 CREATE Household (CreateHousehold)

**Endpoint:** `POST /api/v1/households`
**File:** `backend/internal/household/handler.go`, line 185

**RTID Source:**

```
SUPER_ADMIN:  req.RequestedRTID (request body) → diverifikasi ke rts table → req.RTID
Tenant user:  getRTID(r) → JWT claim ac.RTID → req.RTID
Tenant BLOCK: req.RequestedRTID di-reject → "rt_id is not allowed for tenant users"
```

**File:** `backend/internal/household/repository.go`, `HouseholdCreate`, line 188

**4 SQL INSERT dalam satu transaction:**

| # | Tabel | Field rt_id | Source |
|---|-------|-------------|--------|
| 1 | `physical_houses` | `physical_houses.rt_id = in.RTID` | Dari handler |
| 2 | `households` | `households.rt_id = in.RTID` | Dari handler |
| 3 | `household_occupancies` | **NO rt_id column** | Transitive via FK |
| 4 | `residents` | `residents.rt_id = in.RTID` | Dari handler |

**CRITICAL:** `household_occupancies` TIDAK punya kolom `rt_id`. RT isolation untuk occupancy enforcement secara transitive:

```
household_occupancies.physical_house_id → physical_houses.rt_id
household_occupancies.household_id → households.rt_id
```

### 1.2 CREATE Physical House

**Physical house TIDAK punya endpoint standalone.** Dibuat otomatis saat `HouseholdCreate` (repository.go line 218):

```sql
INSERT INTO physical_houses (rt_id, house_number, address, is_active)
VALUES ($1, $2, $3, true)
-- $1 = in.RTID (dari auth context)
```

Atau saat `HouseholdMove` (repository.go line 812):

```sql
INSERT INTO physical_houses (rt_id, house_number, address, is_active)
VALUES ($1, $2, $3, true)
-- $1 = rtID (dari handler resolveRTID)
```

### 1.3 UPDATE Household (UpdateHousehold)

**Endpoint:** `PATCH /api/v1/households/{id}`
**File:** `backend/internal/household/handler.go`, line 431
**Repository:** `backend/internal/household/repository.go`, `HouseholdUpdate`, line 460

**UpdateHouseholdInput** (`backend/internal/household/model.go`, line 147):

```go
type UpdateHouseholdInput struct {
    HouseNumber     *string          `json:"house_number,omitempty"`
    HeadName        *string          `json:"head_name,omitempty"`
    FullName        *string          `json:"full_name,omitempty"`
    Nik             *string          `json:"nik,omitempty"`
    Phone           *string          `json:"phone,omitempty"`
    Email           *string          `json:"email,omitempty"`
    Address         *string          `json:"address,omitempty"`
    OccupancyStatus *OccupancyStatus `json:"occupancy_status,omitempty"`
    IsActive        *bool            `json:"is_active,omitempty"`
}
```

**TIDAK ADA field `rt_id` di `UpdateHouseholdInput`.**

**SQL UPDATE yang dijalankan** (repository.go):

| Step | SQL | Field yang diupdate |
|------|-----|-------------------|
| 1 | `UPDATE physical_houses SET house_number = $1...` | house_number, address |
| 2 | `UPDATE physical_houses SET address = $1...` | address |
| 3 | `UPDATE household_occupancies SET occupancy_status = $1...` | occupancy_status |
| 4 | `UPDATE residents SET full_name/phone/nik/email = ...` | resident fields |
| 5 | `UPDATE households SET head_name/is_active = ...` | head_name, is_active |

**KESIMPULAN: `households.rt_id` TIDAK DIUBAH oleh UpdateHousehold. `physical_houses.rt_id` JUGA TIDAK DIUBAH. RT immutable setelah create.**

### 1.4 MOVE Household (MoveHousehold)

**Endpoint:** `POST /api/v1/households/{id}/move`
**File:** `backend/internal/household/repository.go`, `HouseholdMove`, line 771

MoveHousehold **TIDAK mengubah RT**. Memindahkan household ke physical house **dalam RT yang sama**:

```sql
-- Step 2: Cari target house IN THE SAME RT
SELECT id FROM physical_houses
WHERE rt_id = $1 AND house_number = $2 AND is_active = true
-- $1 = rtID (dari handler resolveRTID, sama dengan RT existing)
```

Tidak ada operasi `MoveHouseholdAcrossRTs`.

### 1.5 CREATE Bill (CreateBill)

**Endpoint:** `POST /api/v1/bills`
**File:** `backend/internal/finance/handler.go`, `CreateBill`
**Service:** `backend/internal/finance/service.go`, `CreateBill`
**Repository:** `backend/internal/finance/repository.go`, `BillCreate`, line 240

**SQL INSERT:**

```sql
INSERT INTO bills (rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
VALUES ($1, $2, $3, $4, $5, $6, 'unpaid')
-- $1 = rtID (dari auth context JWT ac.RTID)
-- $2 = in.HouseholdOccupancyID (dari request body)
```

**CRITICAL GAP: Tidak ada validasi bahwa `household_occupancy_id` milik household yang ber-RT sama dengan `rt_id`.**

Namun dalam praktiknya aman karena:

| Method | Safety Net |
|--------|-----------|
| `GenerateBills` | Menggunakan `GetActiveCurrentOccupancyIDs(ctx, tx, rtID)` yang JOIN ke `households h WHERE h.rt_id = $1` — hanya occupancy dari RT yang benar dikembalikan |
| `CreateBill` langsung | **TIDAK ADA validasi cross-RT**. Jika bendahara calling API langsung dengan `household_occupancy_id` dari RT lain, bill akan dibuat dengan `bills.rt_id` = user's RT tapi `bills.household_occupancy_id` → RT lain |

### 1.6 GENERATE Bills (GenerateBills)

**File:** `backend/internal/finance/service.go`, `GenerateBills`, line 139

```go
occupancyIDs, err := GetActiveCurrentOccupancyIDs(ctx, tx, rtID)
```

**File:** `backend/internal/finance/repository.go`, `GetActiveCurrentOccupancyIDs`, line 573

```sql
SELECT ho.id
FROM household_occupancies ho
JOIN households h ON ho.household_id = h.id
WHERE h.rt_id = $1 AND h.is_active = true AND ho.end_date IS NULL
-- ← FILTER BY households.rt_id!
```

**Ini adalah safety net utama.** GenerateBills hanya membuat bill untuk occupancy yang household-nya punya `rt_id` yang sama dengan RT user.

---

## 2. Meaning of Each RT Field

### `households.rt_id`

**Merepresentasikan: "RT tempat warga terdaftar sebagai household"**

- Ini adalah master data identitas household
- FK to `rts(id)`
- Digunakan untuk tenant isolation di Warga module
- **Immutable** — tidak bisa diupdate atau di-move antar RT

### `physical_houses.rt_id`

**Merepresentasikan: "RT lokasi fisik rumah"**

- Ini adalah master data lokasi fisik
- FK to `rts(id)`
- Unique index: `(rt_id, house_number) WHERE is_active = true`
- **Immutable** — tidak bisa diupdate

### `household_occupancies` (NO rt_id)

- Hanya bridge table: `physical_house_id` + `household_id`
- RT di-derive secara transitive via join chain
- Tidak punya kolom rt_id sendiri (by design)

### `bills.rt_id`

**Merepresentasikan: "RT yang memiliki kewajiban iuran"**

- FK to `rts(id)`
- Diisi dari auth context (JWT claim), bukan dari data referensial
- **Potential inconsistency**: Tidak ada constraint yang memaksa `bills.rt_id == households.rt_id` via `household_occupancy_id`

### `residents.rt_id`

**Merepresentasikan: "RT tempat warga terdaftar"**

- FK to `rts(id)`
- Digunakan untuk tenant isolation dan `GetOccupancyIDForUser`

---

## 3. Warga RT Source

### Backend: Query Warga

**File:** `backend/internal/household/repository.go`

`householdBaseSelect` (line 116) — digunakan oleh HouseholdList, HouseholdGetByID:

```sql
SELECT h.id, h.rt_id, h.head_name, h.is_active, ...
       ph.house_number, ph.address, ho.occupancy_status,
       r.id, r.rt_id, r.full_name, ...
       r.rt, r.rw, r.rt_name
FROM households h
LEFT JOIN household_occupancies ho ON ho.household_id = h.id AND ho.end_date IS NULL
LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
LEFT JOIN LATERAL (
    SELECT r.id, r.rt_id, r.full_name, ...,
           res_rt.rt, res_rt.rw, res_rt.name AS rt_name
    FROM residency_periods rp
    JOIN residents r ON r.id = rp.resident_id
    LEFT JOIN rts res_rt ON r.rt_id = res_rt.id
    WHERE rp.household_occupancy_id = ho.id AND rp.end_date IS NULL
      AND (rp.relationship_to_head = 'HEAD' OR ...)
    ORDER BY rp.created_at ASC LIMIT 1
) r ON true
```

**Filter:** `WHERE h.rt_id = $1` (HouseholdList, line 397)
**RT berasal dari:** `households.rt_id`

### Mobile Warga: API Call

**File:** `mobile/lib/features/warga/data/warga_repository.dart`

| Method | Endpoint | RT Context |
|--------|----------|-----------|
| GET | `/api/v1/households` | No rt_id — derived from JWT |
| GET | `/api/v1/residents` | No rt_id — derived from JWT |
| POST | `/api/v1/households` | `rt_id` only for super_admin |
| PATCH | `/api/v1/households/{id}` | No rt_id sent |

**Mobile app mengirim NO rt_id untuk tenant operations.** Server yang filter via JWT.

---

## 4. Iuran RT Source

### Backend: Query Iuran

**File:** `backend/internal/finance/repository.go`

`BillList` (line 282):

```sql
SELECT b.id, b.rt_id, b.household_occupancy_id, ...
       d.name as due_name, ph.house_number, h.head_name
FROM bills b
JOIN dues d ON b.due_id = d.id
JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
JOIN households h ON ho.household_id = h.id
LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
WHERE b.rt_id = $1
-- ← FILTER ONLY ON bills.rt_id
-- ← JOIN ke households dan physical_houses, TAPI tidak pakai rt_id mereka di WHERE
```

`BillGetByID` (line 258):

```sql
SELECT b.id, b.rt_id, ...
FROM bills b
JOIN dues d ON b.due_id = d.id
JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
JOIN households h ON ho.household_id = h.id
LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
WHERE b.id = $1 AND b.rt_id = $2
-- ← FILTER ONLY ON b.rt_id
```

**Iuran menggunakan `bills.rt_id` untuk filter, BUKAN `households.rt_id`.**

Response fields:

| Field | Source Table | Field |
|-------|-------------|-------|
| `rt_id` | `bills` | `b.rt_id` |
| `house_number` | `physical_houses` | `ph.house_number` |
| `head_name` | `households` | `h.head_name` |

### Mobile Iuran: Mock Data

**File:** `mobile/lib/features/iuran/presentation/screens/iuran_screen.dart`, line 56:

```dart
final bills = IuranMockData.bills;
```

**Mobile Iuran menggunakan 100% mock data. Tidak ada API integration.**

---

## 5. Create Bill Validation

### GenerateBills (AMAN)

Safety net: `GetActiveCurrentOccupancyIDs` filter by `h.rt_id`:

```sql
SELECT ho.id FROM household_occupancies ho
JOIN households h ON ho.household_id = h.id
WHERE h.rt_id = $1 AND h.is_active = true AND ho.end_date IS NULL
```

Hanya occupancy dari RT yang benar dikembalikan → bills yang dibuat otomatis valid RT.

### CreateBill (GAP)

Tidak ada validasi cross-RT:

```go
// repository.go line 240
func BillCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateBillInput) (*Bill, error) {
    tx.QueryRowContext(ctx,
        `INSERT INTO bills (rt_id, household_occupancy_id, ...) VALUES ($1, $2, ...)`,
        rtID, in.HouseholdOccupancyID, ...)  // ← NO cross-validation!
}
```

Jika seseorang calling `CreateBill` langsung dengan `household_occupancy_id` dari RT lain → bill akan dibuat dengan data tidak konsisten.

---

## 6. Existing Data Mismatch — Audit Seed

### Seed Data Consistency Check

**File:** `backend/internal/devseed/seed.go` dan `data.go`

| Chain | RT Alignment |
|-------|-------------|
| `physical_houses.rt_id` → `household_occupancies` | ✓ Same RT |
| `household_occupancies` → `households.rt_id` | ✓ Same RT |
| `bills.rt_id` (seeded) → `household_occupancy_id` | ✓ Same RT |

**Semua seed data konsisten.** Tidak ada mismatch di seed.

### Real-world Mismatch Risk

Karena **RT immutable** (tidak bisa diupdate), mismatch hanya bisa terjadi via:

1. **Direct `CreateBill` API call** dengan `household_occupancy_id` dari RT lain — **possible security gap**
2. **SUPER_ADMIN cross-RT queries** (`BillListAll`, `BillGetByIDAll`) — trust the `rt_id` in the record, no cross-validation

---

## 7. Required Invariant

Berdasarkan business logic yang ditemukan:

### Invariant 1: RT Immutability (ALREADY ENFORCED)

```
households.rt_id — immutable after creation
physical_houses.rt_id — immutable after creation
residents.rt_id — immutable after creation
```

Tidak ada API endpoint yang mengupdate `rt_id` mana pun. RT determined once at create time. ✅

### Invariant 2: Consistency for Same Location (NOT ENFORCED)

```
Untuk occupancy yang sama:
    physical_houses.rt_id == households.rt_id
```

Secaris bisnis, satu fisik house harus berada di satu RT. Dalam `HouseholdCreate`, keduanya ditulis dari `in.RTID` yang sama → selalu match saat create.

**TAPI** tidak ada constraint yang memaksa ini jika ada future code yang mengubah salah satu secara langsung.

### Invariant 3: Bill RT = Occupancy's Household RT (NOT ENFORCED)

```
bills.rt_id == (SELECT h.rt_id FROM households h
                JOIN household_occupancies ho ON h.id = ho.household_id
                WHERE ho.id = bills.household_occupancy_id)
```

**Ini adalah invariant yang paling penting dan paling lemah.** GenerateBills menjaga ini via `GetActiveCurrentOccupancyIDs`, tapi `CreateBill` tidak memvalidasi.

---

## 8. Options Comparison

### Option A: Tambah Validasi di CreateBill (RECOMMENDED)

**Perubahan:**
- `backend/internal/finance/repository.go` — Tambah validasi cross-RT di `BillCreate`

**Kelebihan:**
- Perubahan minimal (1 fungsi, ~10 baris SQL tambahan)
- Tidak mengubah schema
- Tidak breaking API
- Langsung menutup security gap

**Kekurangan:**
- `bills.rt_id` tetap denormalized
- Masih bergantung pada `GenerateBills` yang sudah aman

### Option B: Hapus `bills.rt_id`, Gunakan JOIN

**Perubahan:**
- Drop column `bills.rt_id`
- Update semua query Bill untuk JOIN `household_occupancies → households → rt_id`
- Breaking change di API response
- Migration data

**Kelebihan:**
- Source of truth tunggal dari `household_occupancies`
- Tidak ada duplikasi data

**Kekurangan:**
- Breaking change besar
- Semua query Bill harus diubah
- Migration complex
- Performance impact (JOIN per query)

### Option C: Sinkronisasi RT Fields

**Perubahan:**
- Tambah constraint/check untuk ensure `households.rt_id == physical_houses.rt_id` via occupancy chain

**Kelebihan:**
- Enforce consistency secara database level

**Kekurangan:**
- Belum ada cara untuk mengubah salah satu RT (immutable), jadi constraint hampir selalu pass
- Overkill untuk kasus yang tidak mungkin terjadi

---

## 9. Recommended Minimal Fix

### Priority 1: Tambah Validasi Cross-RT di CreateBill

```go
// backend/internal/finance/repository.go — BillCreate
func BillCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateBillInput) (*Bill, error) {
    // Validate: household_occupancy must belong to requested RT
    var occupancyHouseholdRT string
    err := tx.QueryRowContext(ctx, `
        SELECT h.rt_id
        FROM household_occupancies ho
        JOIN households h ON ho.household_id = h.id
        WHERE ho.id = $1 AND ho.end_date IS NULL
    `, in.HouseholdOccupancyID).Scan(&occupancyHouseholdRT)

    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("household_occupancy not found or inactive")
        }
        return nil, fmt.Errorf("validate occupancy: %w", err)
    }

    if occupancyHouseholdRT != rtID {
        return nil, fmt.Errorf("household_occupancy does not belong to requested RT")
    }

    // ... existing insert logic
}
```

**Impact:**
- 1 file changed: `backend/internal/finance/repository.go`
- ~15 lines added
- No schema change
- No API contract change
- No frontend change needed

### Priority 2 (Low): Tambah Warning di Frontend

Jika ada future feature untuk "Create Bill manually" di UI, tambahkan warning jika RT occupancy berbeda dari user's RT.

---

## 10. Files That Would Need Changes

| File | Change | Priority |
|------|--------|----------|
| `backend/internal/finance/repository.go` | Tambah validasi cross-RT di `BillCreate` | HIGH |
| `backend/internal/finance/repository.go` (opsional) | Tambah validasi di `BillListAll`/`BillGetByIDAll` untuk SUPER_ADMIN | LOW |

---

## Verdict

### Question A: Untuk Warga, RT berasal dari mana?

**`households.rt_id`** — melalui query `HouseholdList` yang filter `WHERE h.rt_id = $1`.

### Question B: Untuk Iuran, RT berasal dari mana?

**`bills.rt_id`** — melalui query `BillList` yang filter `WHERE b.rt_id = $1`.

### Question C: Apakah keduanya seharusnya sama?

**YA.** Satu occupancy hanya bisa terhubung ke satu household → satu RT. Warga filter via `households.rt_id`, Iuran via `bills.rt_id`. Untuk data yang sama, RT harus identik.

### Question D: Apakah `bills.rt_id` merupakan legitimate denormalized field atau duplicate berbahaya?

**Denormalized field yang SAFE dalam praktiknya, tapi ada gap teoretis.** GenerateBills menjaga konsistensi via `GetActiveCurrentOccupancyIDs`. Gap hanya ada jika ada direct `CreateBill` API call dengan cross-RT `occupancy_id`.

### Question E: Apakah API Create Bill seharusnya memvalidasi `requested RT == household occupancy RT`?

**YA.** Validasi wajib sebagai defense-in-depth.

---

## RT Architecture Diagram

```
rts (MASTER)
 │ id, rt, rw, name
 │
 ├── households.rt_id ──────────┐
 │   └── head_name, is_active   │
 │                              ├──→ Warga: filter by households.rt_id
 │                              │
 ├── physical_houses.rt_id ─────┤
 │   └── house_number, address  │
 │                              │
 ├── residents.rt_id ───────────┤
 │   └── full_name, phone, nik  │
 │                              │
 ├── bills.rt_id ───────────────┤ ← Denormalized from auth context
 │   └── household_occupancy_id │   (JWT claim ac.RTID)
 │                              │
 └── dues.rt_id ────────────────┘
        │
        ▼
household_occupancies (NO rt_id column)
  │ physical_house_id → physical_houses.id
  │ household_id → households.id
  │
  └── RT derived transitively:
        household_occupancies.household_id → households.rt_id
        household_occupancies.physical_house_id → physical_houses.rt_id
```

### Key Relationships

| Relationship | Enforced | Notes |
|-------------|----------|-------|
| `households.rt_id` ↔ `rts(id)` | FK constraint | |
| `physical_houses.rt_id` ↔ `rts(id)` | FK constraint | |
| `bills.rt_id` ↔ `rts(id)` | FK constraint | |
| `occupancy.physical_house_id` ↔ `physical_houses.id` | FK constraint | |
| `occupancy.household_id` ↔ `households.id` | FK constraint | |
| `households.rt_id` ↔ `physical_houses.rt_id` (same location) | **NO** | Rely on `HouseholdCreate` to write both from same `in.RTID` |
| `bills.rt_id` ↔ `occupancy.household_id` (same RT) | **NO** | Only enforced by `GenerateBills` via `GetActiveCurrentOccupancyIDs` |

### Immutability Summary

| Entity | rt_id Field | Can Change? |
|--------|------------|-------------|
| `households` | `rt_id` | **NO** — never in any UPDATE |
| `physical_houses` | `rt_id` | **NO** — never in any UPDATE |
| `residents` | `rt_id` | **NO** — never in any UPDATE |
| `bills` | `rt_id` | **NO** — only set at INSERT |

**RT is immutable across the entire system.** Mismatch can only occur if someone crafts a direct API call with cross-RT data.

---

## Final Verification

```bash
git status --short
# (empty — working tree clean)

git log -5 --oneline
# 98d3569 (HEAD -> main) docs: audit Iuran/Household RT mismatch root cause
# 7e8db9d (origin/main, origin/HEAD) feat: open iuran payment in modal
# ec73921 feat: open iuran detail in modal
# 90e3613 fix: return empty payment lists as arrays
# aac906f fix: resolve iuran frontend build errors
```

✅ Working tree clean
✅ No changes made (read-only audit only)
