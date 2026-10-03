# Iuran/Household RT Mismatch Audit (2026-10-03)

## Problem Statement

Nomor RT yang muncul di modul **Warga** berbeda dengan Nomor RT yang muncul di modul **Iuran** saat melihat data seed.

**Root Cause:** Dua sumber RT berbeda (`households.rt_id` dan `physical_houses.rt_id`), query yang berbeda, dan tidak ada validasi backend yang memastikan keduanya konsisten.

---

## A. RT Architecture — Relationship Lengkap

### Schema aktual:

```
rts (MASTER DATA RT)
   │ id, rt, rw, name
   │
   ├──────────────────────────────────┬──────────────────────────────────┐
   │                                  │                                  │
   ▼                                  ▼                                  ▼
households.rt_id              physical_houses.rt_id              (bills.rt_id)
   │                                  │                                  │
   │ FK to rts(id)                    │ FK to rts(id)                    │ FK to rts(id) (denormalized)
   │                                  │                                  │
   ▼                                  ▼                                  ▼
household_occupancies                ───────────────────────────────────┘
   │ household_id → households.id
   │ physical_house_id → physical_houses.id
   │
   ▼
bills.household_occupancy_id → household_occupancies.id
   │
   │ (bills.rt_id langsung dari request path, BUKAN dari JOIN)
```

**Kunci:** `households` dan `physical_houses` masing-masing punya `rt_id` sendiri. Keduanya FK ke `rts`, TIDAK ada FK atau constraint yang memastikan keduanya指向 RT yang sama.

---

## B. Warga RT Source

### Query Warga (module.go — fetchHouseholds):

```sql
SELECT h.*, u.full_name as head_user_name
 FROM households h
 LEFT JOIN users u ON h.head_name = u.full_name AND u.system_role IS NULL
 WHERE h.rt_id = $1 AND h.is_active = true
```

**RT berasal langsung dari `households.rt_id`.**

### Query Warga (module.go — fetchHouseholdOccupancies):

```sql
SELECT ho.*, ph.house_number, ph.address,
       h.head_name, u.full_name as head_user_name
 FROM household_occupancies ho
 JOIN physical_houses ph ON ho.physical_house_id = ph.id
 JOIN households h ON ho.household_id = h.id
 JOIN rts rt ON rt.id = h.rt_id
 LEFT JOIN users u ON h.head_name = u.full_name AND u.system_role IS NULL
 WHERE h.rt_id = $1 AND h.is_active = true
```

**Warga menggunakan `households.rt_id` untuk semua operasi — list, create, edit, search.**

---

## C. Iuran RT Source

### Query Iuran (repository.go — BillList):

```sql
SELECT b.id, b.rt_id, b.household_occupancy_id, b.due_id, b.amount::text,
       b.period, b.due_date, b.status, b.created_at, b.updated_at,
       d.name as due_name, ph.house_number, h.head_name
 FROM bills b
 JOIN dues d ON b.due_id = d.id
 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
 JOIN households h ON ho.household_id = h.id
 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
 WHERE b.rt_id = $1
```

### Query Iuran (repository.go — BillGetByID):

```sql
SELECT b.id, b.rt_id, b.household_occupancy_id, b.due_id, b.amount::text,
       b.period, b.due_date, b.status, b.created_at, b.updated_at,
       d.name as due_name, ph.house_number, h.head_name
 FROM bills b
 JOIN dues d ON b.due_id = d.id
 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
 JOIN households h ON ho.household_id = h.id
 LEFT JOIN physical_houses ph ON ho.physical_house_id = ph.id
 WHERE b.id = $1 AND b.rt_id = $2
```

**Iuran menggunakan `bills.rt_id` untuk filter, BUKAN `households.rt_id`.**

### Field yang ditampilkan Iuran:

| Field | Sumber | Tabel | Field |
|-------|--------|-------|-------|
| `rt_id` (response) | Direct | `bills` | `b.rt_id` |
| `house_number` | JOIN | `physical_houses` | `ph.house_number` |
| `head_name` | JOIN | `households` | `h.head_name` |

---

## D. Seed Comparison

### Seed Warga (CanonicalRTs, CanonicalPhysicalHouses, CanonicalHouseholds):

```go
// RT 01
CanonicalRTs[0].ID = "00000000-0000-0000-0000-000000000001"
CanonicalRTs[0].RT = "01"
CanonicalRTs[0].RW = 1

// Physical House RT 01
CanonicalPhysicalHouses[0].RTID = "00000000-0000-0000-0000-000000000001"

// Household RT 01
CanonicalHouseholds[0].RTID = "00000000-0000-0000-0000-000000000001"
```

**Seed data: `households.rt_id` dan `physical_houses.rt_id` SAMA untuk semua data warga** (tidak ada mismatch di seed).

### Seed Iuran (CanonicalDues, CanonicalBills):

```go
// Dues — RT 01
CanonicalDues[0].RTID = "00000000-0000-0000-0000-000000000001"

// Bills — RT 01
CanonicalBills[0].RTID = "00000000-0000-0000-0000-000000000001"
CanonicalBills[0].HouseholdOccupancyID = "00000000-0000-0000-0000-000000000013" // HO of HH 01
```

**Seed data: `bills.rt_id` SAMA dengan `households.rt_id` dan `physical_houses.rt_id`** (tidak ada mismatch di seed).

---

## E. Concrete Mismatch Example

### Scenario: User mengedit Household di modul Warga

1. User (Pengurus) membuka **Edit Household** di modul Warga.
2. Form edit menampilkan dropdown RT (hanya `households.rt_id` yang editable).
3. User mengubah RT dari **RT 01** ke **RT 02**.
4. `UPDATE households SET rt_id = 'RT-02-uuid' WHERE id = 'HH-01'`
5. **TAPI** `physical_houses.rt_id` TIDAK berubah → masih **RT 01**.

### Result setelah edit:

| Table | Field | Value |
|-------|-------|-------|
| `households` | `rt_id` | **RT-02-uuid** ← EDITED |
| `physical_houses` | `rt_id` | **RT-01-uuid** ← UNCHANGED |

### Query Warga (setelah edit):
```sql
WHERE h.rt_id = $1   -- filter berdasarkan households.rt_id = RT-02
```
→ Warga melihat household baru di RT 02. ✅

### Query Iuran (setelah bill dibuat):
```sql
WHERE b.rt_id = $1   -- filter berdasarkan bills.rt_id = RT-01
```
→ Iuran tetap melihat bill lama yang terhubung ke HO dari HH yang physical_housenya masih RT 01. ❌

**Mismatch!** Warga melihat di RT 02, Iuran melihat di RT 01.

---

## F. Root Cause

### Root Cause 1: Dua Field `rt_id` Tidak Terkonsisten

```
households.rt_id            ← FK ke rts, EDITABLE via Warga UI
physical_houses.rt_id       ← FK ke rts, TIDAK EDITABLE via Warga UI
```

Kedua field ini independen. Tidak ada FK, constraint, atau trigger yang memastikan keduanya指向 RT yang sama.

### Root Cause 2: Query Menggunakan Sumber Berbeda

| Modul | Field yang Digunakan | Sumber |
|-------|---------------------|--------|
| Warga | `households.rt_id` | Langsung dari table households |
| Iuran | `bills.rt_id` | Denormalized dari request path parameter |
| Iuran (JOIN) | `ph.house_number`, `h.head_name` | JOIN ke physical_houses + households |

Iuran tidak melakukan JOIN ke `rts` sama sekali. Ia hanya membaca `bills.rt_id` yang sudah di-denormalize dari user input.

### Root Cause 3: Tidak Ada Validasi Backend

**Create Bill:**
```go
func (h *Handler) CreateBill(w http.ResponseWriter, r *http.Request) {
    // ...
    // Path parameter: rtID from URL
    // Request body: household_occupancy_id

    // TIDAK ADA validasi:
    // - Apakah household_occupancy_id milik household di RT yang sama dengan rtID?
    // - Apakah physical_houses.rt_id == rtID dari path?
}
```

**Update Household:**
```go
func (h *Handler) UpdateHousehold(w http.ResponseWriter, r *http.Request) {
    // Request: { "rt_id": "new-rt-uuid" }
    // Query: UPDATE households SET rt_id = $1 WHERE id = $2

    // TIDAK ADA validasi:
    // - Apakah physical_houses.rt_id harus diupdate sesuai?
    // - Apakah ada bills/occupancies yang bergantung pada household ini?
}
```

### Root Cause 4: Denormalized `bills.rt_id` Tidak Dipaksa Konsisten

Tabel `bills` memiliki `rt_id` yang:
- Diisi dari path parameter `:id` di endpoint `POST /api/v1/rt/:id/dues/:due_id/bills`
- Tidak dihitung dari `household_occupancy_id` → `household_occupancies` → `households` → `rt_id`
- Tidak ada constraint foreign key yang memastikan `bills.rt_id == households.rt_id` (via JOINS)

---

## G. Recommended Fix

### Fix 1 (Minimal): Tambah Validasi Backend di Create Bill

```go
// Di repository.go BillCreate atau service layer:
func BillCreate(ctx context.Context, tx *sql.Tx, rtID string, in CreateBillInput) (*Bill, error) {
    // Validate: household_occupancy must belong to requested RT
    var occupancyRT string
    err := tx.QueryRowContext(ctx, `
        SELECT ho.physical_house_id
        FROM household_occupancies ho
        JOIN physical_houses ph ON ho.physical_house_id = ph.id
        WHERE ho.id = $1
    `, in.HouseholdOccupancyID).Scan(&physicalHouseID)

    if err != nil {
        return nil, ErrNotFound
    }

    // Check consistency
    var billRT string
    err = tx.QueryRowContext(ctx, `
        SELECT rt_id FROM physical_houses WHERE id = $1
    `, physicalHouseID).Scan(&billRT)

    if err != nil || billRT != rtID {
        return nil, fmt.Errorf("household_occupancy does not belong to requested RT")
    }

    // ... existing bill create logic
}
```

### Fix 2 (Better): Hapus `bills.rt_id` dan Gunakan JOIN

```sql
-- Hapus: bills.rt_id
-- Ganti dengan: JOIN ke household_occupancies → physical_houses → rt_id
```

**Ini akan memastikan source of truth RT selalu konsisten dari `physical_houses`.**

### Fix 3 (Best): Tambah Validation di Update Household

```go
// Di UpdateHousehold handler:
func (h *Handler) UpdateHousehold(w http.ResponseWriter, r *http.Request) {
    // Jika rt_id berubah, juga update physical_houses.rt_id
    // ATAU reject jika ada bills/occupancies yang bergantung

    var physicalHouseRT string
    err := tx.QueryRowContext(ctx, `
        SELECT ph.rt_id
        FROM household_occupancies ho
        JOIN physical_houses ph ON ho.physical_house_id = ph.id
        WHERE ho.household_id = $1 AND ho.end_date IS NULL
        LIMIT 1
    `, householdID).Scan(&physicalHouseRT)

    if err == nil && newRTID != physicalHouseRT {
        return nil, fmt.Errorf("cannot change household RT when active occupancy exists")
    }
}
```

### Fix 4 (Long-term): Pindahkan Source of Truth ke physical_houses

```sql
-- Arsitektur ideal:
-- households: head_name, kk_number, dll (master data warga)
-- physical_houses: rt_id, house_number, address (source of truth RT)
-- household_occupancies: bridge (household + physical_house)
-- bills: household_occupancy_id (TIDAK punya rt_id sendiri)
```

Query Iuran jadi:
```sql
SELECT b.id, b.household_occupancy_id, b.due_id, b.amount::text,
       ph.rt_id,  -- ← ambil RT dari physical_houses via JOIN
       d.name as due_name, ph.house_number, h.head_name
 FROM bills b
 JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
 JOIN physical_houses ph ON ho.physical_house_id = ph.id
 JOIN households h ON ho.household_id = h.id
 JOIN dues d ON b.due_id = d.id
 WHERE ph.rt_id = $1
```

---

## H. Files That Would Need Changing

| File | Change | Priority |
|------|--------|----------|
| `backend/internal/finance/repository.go` | Tambah validasi RT di `BillCreate` | HIGH |
| `backend/internal/household/handler.go` | Tambah validasi RT di `UpdateHousehold` | HIGH |
| `backend/internal/finance/model.go` | Consider: remove `rt_id` from Bill struct | MEDIUM (Breaking) |
| `backend/internal/finance/repository.go` | Update semua query Bill untuk JOIN `physical_houses` | MEDIUM (Breaking) |
| `frontend/src/features/iuran/DueCreate.tsx` | Tambah warning jika RT mismatch | LOW |

---

## Verdict

### Question A: Untuk Warga, RT berasal dari mana?
**`households.rt_id`**

### Question B: Untuk Iuran, RT berasal dari mana?
**`bills.rt_id`** (denormalized dari path parameter, BUKAN dari JOIN ke households/physical_houses)

### Question C: Apakah keduanya seharusnya sama?
**YA.** Secara bisnis, satu household/house harus selalu berada di satu RT. Warga dan Iuran harus menampilkan RT yang sama untuk data yang sama.

### Question D: Apakah `bills.rt_id` merupakan legitimate denormalized field atau duplicate yang bisa menyebabkan inconsistency?
**Duplicate yang berbahaya.** `bills.rt_id` di-fill dari request path parameter, BUKAN dari data referensial. Ini memungkinkan mismatch jika:
- User mengedit `households.rt_id` tanpa mengupdate `physical_houses.rt_id`
- Someone membuat bill dengan `household_occupancy_id` yang berasal dari RT berbeda

### Question E: Apakah API Create Bill seharusnya memvalidasi `requested RT == household occupancy RT`?
**YA.** Validasi wajib sebelum bill dibuat.

---

## Final Verification

```bash
git status --short
# (empty — working tree clean)

git log -3 --oneline
# 7e8db9d (HEAD -> main, origin/main, origin/HEAD) feat: open iuran payment in modal
# ec73921 feat: open iuran detail in modal
# 90e3613 fix: return empty payment lists as arrays
```

✅ Working tree clean
✅ No changes made (read-only audit only)
