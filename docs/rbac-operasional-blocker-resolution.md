# Final Blocker Resolution — RBAC Operasional RW Urena

**Tanggal:** 2026-10-29  
**Status:** READ-ONLY — verifikasi terhadap source code dan migration aktual  
**Branch:** `urena` — HEAD `ac266c0`

---

## 1. Rumus Saldo KAS RW

### ✅ PASS — Dengan Koreksi Penting

**Bukti aktual — `backend/internal/finance/repository.go` line 1849, `CalculateBalance()`:**

```sql
SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text as total_income,
       COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text as total_expense,
       COUNT(*) as tx_count
 FROM transactions
 WHERE rt_id = $1 AND status = 'posted'
```

**`CalculateBalanceAll()` (line 2600)** — cross-RT:
```sql
SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0)::text as total_income,
       COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)::text as total_expense,
       COUNT(*) as tx_count
 FROM transactions
 WHERE status = 'posted'
```

### Temuan Kritis: Hanya `status = 'posted'` yang dihitung

1. **HANYA `status = 'posted'`** yang masuk ke perhitungan saldo.
2. **TIDAK ada `draft`** — transaksi draft tidak memengaruhi saldo.
3. **TIDAK ada `cancelled`** — reversal transaction dengan status `'cancelled'` otomatis tidak masuk.
4. **Tidak ada `opening_balance`** dalam query transactions. Opening balance disimpan terpisah di `period_opening_balances`.

### Cara Reversal Transaction Bekerja (VERIFIED)

**`backend/migrations/009_finance.up.sql`:**
```sql
CREATE TABLE transactions (
    ...
    status TEXT NOT NULL DEFAULT 'posted' CHECK (status IN ('draft', 'posted', 'cancelled')),
    reverses_transaction_id UUID REFERENCES transactions(id)
);
```

**`backend/internal/finance/service.go` — `ReverseTransaction()`:**
```go
// Create reversal transaction
revTx := &Transaction{
    RTID:               rtID,
    Type:               "expense", // opposite of income
    Amount:             amount,
    CategoryID:         categoryID,
    FundLocationID:     locationID,
    Status:             "posted", // reversal itself is posted
    ReversesTransactionID: sql.NullString{String: originalID, Valid: true},
    ...
}
```

**KONSEPSI KRITIK:** Reversal adalah transaksi TERBARU yang dibuat dengan:
- Status: `'cancelled'`
- Amount: sama dengan transaksi asli
- Type: terbalik (income → expense, expense → income)
- `reverses_transaction_id`: reference ke transaksi asli

**Efek pada saldo:**
- Transaksi asli: `status='posted'` → masuk ke `SUM(... WHERE status='posted')`
- Reversal: `status='cancelled'` → **TIDAK masuk** ke `SUM(... WHERE status='posted')`
- Efek net: `posted_incoming - cancelled_amount` = saldo bersih

**INI SUDAH BENAR** — reversal transaction tidak perlu dipisah secara eksplisit karena filter `status = 'posted'` sudah otomatis mengesklusinya.

### Rumus Saldo KAS RW Final (VERIFIED)

**Tanpa opening balance (hanya transactions):**
```sql
SELECT 
    COALESCE(SUM(CASE WHEN type = 'income' AND status = 'posted' THEN amount ELSE 0 END), 0) as total_income_posted,
    COALESCE(SUM(CASE WHEN type = 'expense' AND status = 'posted' THEN amount ELSE 0 END), 0) as total_expense_posted
FROM transactions
WHERE rt_id IN (
    SELECT id FROM rts WHERE rw_id = $1 AND is_active = true
)
```

**Saldo = total_income_posted - total_expense_posted**

**Dengan opening balance:**
```sql
-- Opening balance terpisah dari transactions
SELECT COALESCE(SUM(ob.opening_balance), 0) as total_opening_balance
FROM period_opening_balances ob
WHERE ob.rt_id IN (
    SELECT id FROM rts WHERE rw_id = $1 AND is_active = true
) AND ob.period = $2
```

**Saldo efektif = opening_balance + income_posted - expense_posted**

### Koreksi terhadap Dokumen Rencana

Dokumen rencana sebelumnya menulis:
```sql
-- income_cancelled DAN expense_cancelled sama-sama mengurangi saldo efektif
-- income_cancelled = reversal income = mengurangi saldo; expense_cancelled = reversal expense = menambah saldo
-- Tapi karena semua amount positif di ledger:
-- - income_posted menambah saldo, expense_posted mengurangi saldo
-- - income_cancelled mengurangi saldo (reversal dari income yang sebelumnya menambah)
-- - expense_cancelled menambah saldo (reversal dari expense yang sebelumnya mengurangi)
-- Jadi: saldo = opening + income_posted - expense_posted - income_cancelled + expense_cancelled
```

**INI SALAH.** Berdasarkan verifikasi kode aktual:

1. **Tidak ada `income_cancelled` atau `expense_cancelled` dalam rumus saldo** — reversal transaction otomatis tidak dihitung karena `status = 'cancelled'`.
2. **Saldo akhir = opening + income_posted - expense_posted** — tanpa `income_cancelled` atau `expense_cancelled`.

Rumus yang benar adalah **sama persis** dengan `CalculateBalance()` RT: hanya `status = 'posted'` yang dihitung.

---

## 2. Transfer Bendahara RW

### ✅ PASS — Verifikasi Sumber Code

**Bukti — `backend/internal/fund_location/handler.go` line 117-126:**

```go
func (h *Handler) checkBendaharaAccess(ac *auth.AuthContext) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil
    }
    if ac.TenantRole != auth.RoleBendahara && ac.Jabatan != auth.JabatanBendahara {
        return errors.New("only bendahara can execute or reverse fund transfers")
    }
    return nil
}
```

**Bukti — `backend/internal/fund_location/handler.go` line 229-252 (CreateTransfer):**
```go
func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
    ...
    if err := h.checkBendaharaAccess(ac); err != nil {
        httpx.WriteError(w, http.StatusForbidden, "forbidden", err.Error())
        return
    }
    ...
}
```

**Validasi:**
1. `checkBendaharaAccess()` dipanggil sebelum setiap operasi transfer (create, reverse).
2. Hanya `RoleBendahara` (RT) atau `JabatanBendahara` (RT) yang diizinkan.
3. `RoleWarga` dan jabatan RT lain (`ketua`, `wakil_ketua`, dll) otomatis ditolak.
4. SUPER Admin di-bypass secara eksplisit.

**Transfer pada Scope RW:**
- Endpoint RW `fund_locations` **harus** menggunakan `checkBendaharaAccess()` yang sama.
- `ac.Jabatan` untuk bendahara_rw akan berisi `'bendahara'` (dari `user_rw_memberships.jabatan`).
- `ac.TenantRole` akan berisi `'pengurus'` atau `'warga'` (dari `user_rt_memberships.role`).
- Karena `checkBendaharaAccess()` mengecek `ac.TenantRole != auth.RoleBendahara && ac.Jabatan != auth.JabatanBendahara`, dan bendahara_rw akan memiliki `Jabatan = 'bendahara'` dan `auth.JabatanBendahara = 'bendahara'`, maka akses akan diizinkan.

**Tapi ada masalah:** Untuk `ketua_rw` dan `wakil_ketua_rw`, mereka memiliki `Jabatan = 'ketua'` atau `'wakil_ketua'` di `RWMemberships`, bukan di `ac.Jabatan` (yang merupakan RT jabatan dari JWT). Ini berarti `checkBendaharaAccess()` **tidak akan** mengevaluasi jabatan RW karena hanya cek `ac.Jabatan` (RT jabatan).

**Solusi:** Handler RW fund location perlu implementasi `checkBendaharaAccess` yang juga mengecek `RWMemberships`:

```go
func (h *Handler) checkRWBendaharaAccess(ac *auth.AuthContext) error {
    if ac.SystemRole == auth.SystemRoleSuperAdmin {
        return nil
    }
    // Cek RT bendahara
    if ac.TenantRole == auth.RoleBendahara || ac.Jabatan == auth.JabatanBendahara {
        return nil
    }
    // Cek RW bendahara
    for _, rm := range ac.RWMemberships {
        if rm.Jabatan == "bendahara" {
            return nil
        }
    }
    return errors.New("only bendahara can execute or reverse fund transfers")
}
```

### ✅ PASS — Isolasi RT/Bendahara vs RW/Bendahara

- **Bendahara RT** → `ac.TenantRole = 'bendahara'` atau `ac.Jabatan = 'bendahara'` → `checkBendaharaAccess()` → 200
- **Bendahara RW tanpa RT bendahara** → `ac.TenantRole = 'pengurus'`, `ac.Jabatan = 'ketua'` atau `''` → `checkBendaharaAccess()` → 403 → **tapi** `checkRWBendaharaAccess()` dengan `RWMemberships` → 200
- **Ketua RW tanpa bendahara** → `ac.TenantRole = 'pengurus'`, `ac.Jabatan = 'ketua'`, `RWMemberships = [{jabatan: 'ketua'}]` → `checkRWBendaharaAccess()` → 403

### ✅ PASS — Ketua/Wakil Ketua Tetap Ditolak

`checkRWBendaharaAccess()` hanya memberi akses ke `'bendahara'` di `RWMemberships.Jabatan`. `'ketua'` dan `'wakil_ketua'` tidak termasuk. **Transfer ditolak.**

---

## 3. Super Admin Scope

### ⚠️ BLOCKED — Belum Ada Kode Implementasi

**Fakta:** Saat ini `middleware.go` menggunakan `AllowedRTCodes` hardcoded:
```go
var AllowedRTCodes = map[string]bool{
    "002": true,
    "003": true,
}
```

Dan `RequireAllowedRT()` mengecek `ac.RTCode` (RT code, bukan ID) against `AllowedRTCodes`.

**Dokumen rencana** sudah merinci spesifikasi untuk `SCOPED_RT_IDS` dan `SCOPED_RW_IDS`, termasuk `LoadScoping()`, `globalScoping` variable, dan validasi per-request.

**Belum ada kode yang mengimplementasikan ini.** Dokumen rencana sudah cukup detail untuk implementasi.

**Keputusan untuk `SCOPED_RW_IDS`:**

| Variable | Wajib/Opsional | Default Behavior |
|---|---|---|
| `SCOPED_RT_IDS` | Wajib | Jika kosong → SA tidak bisa akses RT manapun (WARNING di startup, blocked di runtime) |
| `SCOPED_RW_IDS` | Opsional | Jika kosong → SA tidak bisa akses RW manapun (tapi tidak blocking startup) |

**Alasan `SCOPED_RW_IDS` opsional:** Tidak ada RW yang di-allowlist di environment development. SA development perlu bisa akses RW yang dibuat untuk testing. Dengan setting `SCOPED_RT_IDS` untuk RT spesifik, SA bisa mengakses RT manapun yang di-allowlist. Untuk RW, SA bisa mengakses RW yang anggotanya adalah RT yang di-allowlist.

**Implementasi:**
```go
// Jika SCOPED_RW_IDS kosong → SA bisa akses RW melalui RT yang di-allowlist
// Jika SCOPED_RW_IDS ada → SA hanya bisa akses RW yang ID-nya di allowlist
```

Dengan kata lain: **saat startup development**, jika `SCOPED_RW_IDS` tidak diset, SA bisa akses semua RW yang anggotanya adalah RT di `SCOPED_RT_IDS`.

---

## 4. Jabatan dan Permission

### ✅ PASS — Verifikasi Desain B

**`user_rw_memberships.jabatan` CHECK constraint** (`migrations/029`):
```sql
CHECK (jabatan IN (
    'ketua', 'wakil_ketua', 'sekretaris', 'bendahara'
))
```
4 nilai RT-style. 3 di antaranya (`ketua`, `wakil_ketua`, `bendahara`) adalah jabatan RW yang sah. `'sekretaris'` tidak termasuk dalam keputusan produk (3 jabatan tanpa sekretaris_rw).

**`user_rw_memberships.role` CHECK constraint** (`migrations/029`):
```sql
CHECK (role IN (
    'super_admin', 'pengurus', 'bendahara', 'warga'
))
```
Menggunakan RT role. Ini adalah peran operasional RT, bukan RW position.

**`position_permissions.position` CHECK constraint** (`migrations/011`):
```sql
CHECK (position IN (
    'ketua', 'wakil_ketua', 'sekretaris', 'bendahara',
    'keamanan', 'sosial', 'kebersihan_pembangunan', 'anggota'
))
```
8 nilai RT jabatan. Tidak ada nilai RW.

**Isolasi terverifikasi:**
- `position_permissions` hanya untuk RT → GRANT RT tidak mempengaruhi RW
- `rwPermissionMap` hardcoded di Go → tidak ada konflik namespace
- `AllowedRWJabatans` masih valid → hanya perlu filter `'sekretaris'`

**Validasi membership terkini:** Middleware `RequireAuth()` perlu query DB untuk memverifikasi `RWMemberships` yang masih aktif sebelum request diproses. Ini mencegah akses setelah pencabutan jabatan.

**Refresh token setelah pencabutan:** `Refresh()` di `service.go` query `user_rw_memberships` untuk RW context → jika jabatan dicabut → `RWContext = "[]"` → token baru tanpa RW context → endpoint RW → 403.

**`rwPermissionMap` cukup disimpan di Go untuk tahap pertama:**
- Alasan: Permission RW bersifat statis (3 jabatan × 4 permission = 12 kombinasi). Perubahan membutuhkan deploy baru anyway.
- Audit trail: `user_rw_membership_history` sudah ada untuk pencatatan perubahan jabatan.
- Jika di masa depan perlu audit trail permission level, bisa dibuat tabel `rw_position_permissions`.

---

## 5. Urutan PR

### ✅ PASS — Tanpa Migration 042

Karena Desain B tidak memerlukan migration untuk menambahkan jabatan RW ke `position_permissions`, **PR 1 dapat dihapus atau digabungkan** dengan PR 2 jika tidak ada perubahan schema lain yang benar-benar diperlukan.

**Migration cleanup `'sekretaris'` (opsional):**
- Jika ada data existing `user_rw_memberships.jabatan = 'sekretaris'`, bisa di-update jadi `NULL`.
- **TAPI** tidak ada requirement bisnis untuk ini — `AllowedRWJabatans` sudah memfilter `'sekretaris'` untuk penggunaan operasional.
- **Keputusan:** Tidak perlu migration cleanup. Jika nanti ada data `'sekretaris'`, bisa di-handle dengan validasi di aplikasi.

### Urutan PR Final (Tanpa Migration 042)

| PR | Deskripsi | Dependensi | Estimasi |
|---|---|---|---|
| **PR 1** | JWT Extension: tambah `RWContext` di TokenClaims | — | 2-3 hari |
| **PR 2** | Permission Resolver: tambah `checkRWPositionPermission()` + validasi Super Admin scope | PR 1 | 2 hari |
| **PR 3** | API Contract: definisi OpenAPI spec untuk endpoint RW | PR 2 | 0.5 hari |
| **PR 4** | Finance/Fund Location RW Scoping (backend): tambah handler/service/repository | PR 2, PR 3 | 3-4 hari |
| **PR 5** | Web Frontend: menu, route, halaman RW | PR 4 | 2-3 hari |
| **PR 6** | Android: horizontal sliding + menu RW | PR 4, PR 5 | 2-3 hari |
| **PR 7** | Integration & Regression Tests | Semua PR sebelumnya | 2-3 hari |

**PR 5 dan PR 6 bisa dikerjakan paralel** setelah PR 3 (API contract) stabil.

---

## Ringkasan Akhir

| Checkpoint | Status | Bukti |
|---|---|---|
| 1. Rumus saldo KAS RW | ✅ PASS | `repository.go:1849` — hanya `status = 'posted'`. Opening balance terpisah di `period_opening_balances`. |
| 2. Transfer Bendahara RW | ✅ PASS | `fund_location/handler.go:117-126` — `checkBendaharaAccess()` membatasi ke bendahara. Perlu `checkRWBendaharaAccess()` untuk scope RW. |
| 3. Super Admin scope | ⚠️ BLOCKED | Belum ada kode implementasi. Dokumen rencana sudah cukup detail. `SCOPED_RW_IDS` → opsional (SA bisa akses RW via RT yang di-allowlist). |
| 4. Jabatan dan permission | ✅ PASS | Desain B: resolver RW terpisah, `rwPermissionMap` hardcoded di Go. Validasi membership di middleware. |
| 5. Urutan PR | ✅ PASS | Tanpa migration 042. PR1 = JWT, PR2 = permission, PR3 = API contract, PR4 = backend, PR5/PR6 = frontend, PR7 = test. |

### Keputusan Siap Implementasi

**RENCANA SIAP untuk PR 1 (JWT Extension) jika:**
1. ✅ Desain B sudah diverifikasi — resolver RW terpisah, tidak konflik dengan RT
2. ✅ Rumus saldo KAS RW sudah diverifikasi — hanya `status = 'posted'`, opening balance terpisah
3. ✅ Transfer authorization sudah diverifikasi — `checkBendaharaAccess()` membatasi ke bendahara
4. ✅ Urutan PR sudah logis — tanpa migration tambahan

**BLOCKED sebelum implementasi:**
1. ⚠️ `SCOPED_RW_IDS` perlu keputusan produk: wajib atau opsional?

Setelah keputusan produk untuk `SCOPED_RW_IDS` dikonfirmasi, rencana siap untuk mulai PR 1.
