# Iuran & KAS Module Plan

## 1. Status Dokumen

| Field | Value |
|-------|-------|
| **Status** | Planning — Not Implemented |
| **Created** | 2026-10-02 |
| **Scope** | Modul Iuran (RT Dues) + Modul KAS (Cash Management) |
| **Non-goals** | Migration, API implementation, Web/Android UI, permission finalization |

---

## 2. Goals

- Menyediakan struktur iuran yang dapat didefinisikan per RT, dengan jenis iuran, periode, dan nominal yang dapat dikelola.
- Mencatat tagihan iuran per rumah tangga (atau per warga, TBD) dan melacak status pembayarannya.
- Menyediakan modul Kas untuk mencatat pemasukan dan pengeluaran kas RT secara umum.
- Mengintegrasikan pembayaran iuran ke dalam transaksi Kas Masuk tanpa menyebabkan double-entry.
- Memastikan semua transaksi keuangan mengikuti prinsip immutability dan audit trail yang sama dengan Financial Ledger yang sudah ada.

---

## 3. Modul Iuran

### 3.1 Master Jenis Iuran

- Setiap RT dapat mendefinisikan jenis-jenis iuran yang berlaku.
- Setiap jenis iuran memiliki: nama, deskripsi (opsional), nominal default, periode (minimal bulanan).
- Jenis iuran bersifat per-RT (isolasi tenant).

### 3.2 Tagihan Iuran

- Tagihan dihasilkan berdasarkan jenis iuran dan periode.
- Setiap tagihan terikat pada: jenis iuran, periode, rumah tangga/warga, nominal.
- Tagihan yang belum dibayar menjadi tunggakan.

### 3.3 Pembayaran Iuran

- Perekaman pembayaran yang merujuk pada satu atau lebih tagihan.
- Status pembayaran per tagihan: `belum_bayar`, `sebagian`, `lunas`.
- Riwayat pembayaran tercatat sebagai audit trail.

### 3.4 Periode Iuran

- Minimal periode: bulanan.
- Periode ditandai dengan tahun dan bulan.
- Mendukung pembayaran untuk periode sebelumnya (tunggakan).

### 3.5 Nominal Iuran

- Nominal default dari jenis iuran.
- Nominal dapat disesuaikan per tagihan (TBD: apakah boleh).
- Nominal selalu positif; jenis menentukan income atau expense.

### 3.6 Tunggakan

- Daftar tagihan yang belum lunas.
- Dapat difilter berdasarkan: RT, periode, status, jenis iuran.
- Rekap total tunggakan per RT atau per jenis iuran.

### 3.7 Riwayat Pembayaran

- Log semua pembayaran yang tercatat.
- Dapat difilter berdasarkan: periode, RT, status, jenis iuran.
- Rekap pemasukan iuran per periode.

### 3.8 Laporan / Rekap

- Rekap tunggakan per RT / per jenis iuran.
- Rekap pemasukan iuran per periode.
- Export (v2+): PDF, CSV.

---

## 4. Modul KAS

### 4.1 Kas Masuk

- Mencatat setiap pemasukan kas.
- Field: tanggal, nominal, kategori, keterangan, referensi transaksi.
- Referensi transaksi: opsional, dapat merujuk ke transaksi iuran atau transaksi lainnya.

### 4.2 Kas Keluar

- Mencatat setiap pengeluaran kas.
- Field: tanggal, nominal, kategori, keterangan, referensi transaksi.
- Referensi transaksi: opsional.

### 4.3 Kategori Kas

- Kategori untuk klasifikasi transaksi kas (masuk & keluar).
- Contoh: "Iuran Warga", "Iuran Paksa", "Perlengkapan", "Konsumsi Rapat", "Perbaikan Fasilitas".
- CRUD kategori kas.

### 4.4 Saldo Berjalan

- Saldo = Total Kas Masuk − Total Kas Keluar.
- Saldo dihitung dari ledger, tidak disimpan sebagai derived value.
- Dapat ditampilkan per periode.

### 4.5 Ledger Kas

- Setiap transaksi kas masuk/keluar merupakan entri ledger yang immutable.
- Tidak ada edit atau delete pada transaksi yang sudah diposting.
- Koreksi dilakukan melalui reversal (transaksi pembalik).

### 4.6 Laporan Kas

- Ringkasan kas masuk dan kas keluar per periode.
- Saldo berjalan.
- Export (v2+).

---

## 5. Integrasi Iuran → KAS

### 5.1 Alur Pembayaran Iuran → Kas Masuk

```
Pembayaran Iuran (Rp50.000)
  → Transaksi Kas Masuk +Rp50.000
  → Kategori: "Iuran Warga"
  → Referensi: ID pembayaran iuran
```

### 5.2 Aturan Integrasi

- Setiap pembayaran iuran yang tercatat otomatis menghasilkan transaksi Kas Masuk.
- Transaksi Kas Masuk harus memiliki `reference_id` yang merujuk ke pembayaran iuran.
- **Dilarang** mencatat transaksi Kas Masuk dengan kategori iuran tanpa referensi ke pembayaran iuran yang valid — untuk mencegah double-entry.
- Transaksi kas umum (perlengkapan, konsumsi rapat, perbaikan fasilitas) **tidak harus** berasal dari modul Iuran.

### 5.3 Pencegahan Double Posting

- Setiap pembayaran iuran hanya dapat menghasilkan satu transaksi Kas Masuk.
- `reference_id` pada transaksi Kas Masuk harus unique per pembayaran.
- Idempotency pada endpoint pembayaran: jika `Idempotency-Key` sama, tidak membuat transaksi baru.
- Konfirmasi/manual override oleh Bendahara untuk kasus khusus (TBD).

---

## 6. Authorization

### 6.1 Prinsip Existing

Authorization untuk modul Iuran dan KAS harus mengikuti prinsip yang sudah ada:

1. **Backend adalah enforcement utama.** Semua permission dicek di backend middleware.
2. **Frontend Web/Android hanya melakukan visibility/action gating.** UI menyembunyikan tombol/fitur jika user tidak memiliki izin, tetapi backend tetap enforce.
3. **Permission berbasis permission code.** Tidak hardcoded role check di handler.
4. **Position-based permission via jabatan.** Pengguna dengan jabatan tertentu (ketua, wakil_ketua, sekretaris, bendahara) memiliki akses berbeda.
5. **Legacy role-based compatibility.** Pengguna pengurus tanpa jabatan tetap mengikuti authorization berbasis role seperti sebelumnya.

### 6.2 Permission Matrix — To Be Confirmed

| Action | Super Admin | Ketua RT | Wakil Ketua | Sekretaris | Bendahara | Keamanan | Sosial | Kebersihan | Warga |
|--------|-------------|----------|-------------|------------|-----------|----------|--------|------------|-------|
| Melihat modul Iuran | ? | ? | ? | ? | ? | ? | ? | ? | ? |
| Melihat modul KAS | ? | ? | ? | ? | ? | ? | ? | ? | ? |
| Membuat jenis iuran | ? | ? | ? | ? | ? | - | - | - | - |
| Mengubah jenis iuran | ? | ? | ? | ? | ? | - | - | - | - |
| Menghapus jenis iuran | ? | ? | ? | ? | ? | - | - | - | - |
| Membuat/mengubah tagihan | ? | ? | ? | ? | ? | - | - | - | - |
| Merekam pembayaran | ? | ? | ? | ? | ? | - | - | - | ? |
| Melihat riwayat pembayaran | ? | ? | ? | ? | ? | ? | ? | ? | ? |
| Melihat tunggakan | ? | ? | ? | ? | ? | - | - | - | ? |
| Membuat Kas Masuk | ? | ? | ? | ? | ? | - | - | - | - |
| Membuat Kas Keluar | ? | ? | ? | ? | ? | - | - | - | - |
| Menghapus transaksi kas | ? | ? | ? | ? | ? | - | - | - | - |

**Catatan:** Tandai `?` = perlu diputuskan. Tandai `-` = tidak relevan (posisi tertentu tidak memiliki akses warga-level).

### 6.3 Pertanyaan Authorization

1. Siapa yang boleh melihat modul Iuran? (Hanya Bendahara? Atau semua pengurus?)
2. Siapa yang boleh membuat/mengubah tagihan iuran?
3. Siapa yang boleh mencatat pembayaran iuran? (Apakah warga dapat mencatat pembayaran mereka sendiri?)
4. Siapa yang boleh melihat modul KAS?
5. Siapa yang boleh membuat transaksi Kas Masuk?
6. Siapa yang boleh membuat transaksi Kas Keluar?
7. Apakah Bendahara memiliki kontrol penuh terhadap KAS (termasuk menghapus/merevisi)?
8. Apakah Ketua memerlukan approval untuk transaksi Kas Keluar tertentu?
9. Apakah transaksi yang berasal dari pembayaran Iuran dapat diedit, atau harus immutable/reversal?
10. Apakah permission bersifat per-RT atau global untuk SUPER_ADMIN?

---

## 7. Candidate Domain Model

> **Ini hanya candidate entities. Bukan schema final. Bukan migration.**

### 7.1 Iuran

| Entity | Purpose | Key Relationships |
|--------|---------|-------------------|
| `iuran_type` | Jenis iuran (e.g., "Iuran Keamanan", "Iuran Kebersihan") | 1 RT → N jenis iuran |
| `iuran_bill` | Tagihan iuran per periode per KK/warga | N IuranBill → 1 IuranType, N IuranBill → 1 Household |
| `iuran_payment` | Pembayaran yang merujuk ke satu atau lebih tagihan | N IuranPayment → N IuranBill (many-to-many via junction) |
| `iuran_payment_status` | Status pembayaran per tagihan | 1 IuranBill → 1 Status (belum_bayar / sebagian / lunas) |

### 7.2 KAS

| Entity | Purpose | Key Relationships |
|--------|---------|-------------------|
| `kas_category` | Kategori transaksi kas | 1 RT → N kategori |
| `kas_transaction` | Transaksi kas masuk/keluar | N KasTransaction → 1 KasCategory |
| `kas_reference` | Referensi ke sumber transaksi | Optional: merujuk ke iuran_payment atau null |

### 7.3 Hubungan Antar Modul

```
iuran_type (1) ──< iuran_bill (N) ──< iuran_payment (M) >── iuran_bill (N)
                                                      │
                                                      │ reference_id
                                                      ▼
                                              kas_transaction (Kas Masuk)
                                                      │
                                                      │ kas_category
                                                      ▼
                                                 kas_category
```

---

## 8. Draft API — Not Implemented

> Semua endpoint di bawah ini adalah **proposal draft**. Belum diimplementasi.

### 8.1 Iuran

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/rt/:id/iuran-types` | Daftar jenis iuran per RT (Draft) |
| `POST` | `/api/v1/rt/:id/iuran-types` | Membuat jenis iuran (Draft) |
| `GET` | `/api/v1/rt/:id/iuran-types/:id` | Detail jenis iuran (Draft) |
| `PUT` | `/api/v1/rt/:id/iuran-types/:id` | Mengubah jenis iuran (Draft) |
| `DELETE` | `/api/v1/rt/:id/iuran-types/:id` | Menghapus jenis iuran (Draft) |
| `GET` | `/api/v1/rt/:id/iuran-bills` | Daftar tagihan (filter: periode, status, RT) (Draft) |
| `GET` | `/api/v1/rt/:id/iuran-bills/:id` | Detail tagihan (Draft) |
| `POST` | `/api/v1/rt/:id/iuran-bills` | Membuat tagihan (Draft) |
| `PUT` | `/api/v1/rt/:id/iuran-bills/:id` | Mengubah tagihan (Draft) |
| `GET` | `/api/v1/rt/:id/iuran-payments` | Daftar pembayaran (Draft) |
| `POST` | `/api/v1/rt/:id/iuran-payments` | Merekam pembayaran (Draft) |
| `GET` | `/api/v1/rt/:id/iuran-overdue` | Daftar tunggakan (Draft) |
| `GET` | `/api/v1/rt/:id/iuran-summary` | Rekap pemasukan iuran per periode (Draft) |

### 8.2 KAS

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/rt/:id/kas-categories` | Daftar kategori kas (Draft) |
| `POST` | `/api/v1/rt/:id/kas-categories` | Membuat kategori kas (Draft) |
| `GET` | `/api/v1/rt/:id/kas-transactions` | Daftar transaksi kas (Draft) |
| `GET` | `/api/v1/rt/:id/kas-transactions/:id` | Detail transaksi kas (Draft) |
| `POST` | `/api/v1/rt/:id/kas-transactions` | Membuat transaksi kas masuk (Draft) |
| `GET` | `/api/v1/rt/:id/kas-summary` | Ringkasan saldo kas per periode (Draft) |

### 8.3 Notes

- Semua endpoint memerlukan `rt_id` dari JWT (tenant isolation).
- SUPER_ADMIN endpoint tidak memerlukan `rt_id` di JWT, tetapi `rt_id` sebagai path parameter.
- Endpoint pembayaran menerima `Idempotency-Key` header untuk mencegah duplikasi.
- Semua transaksi kas masuk/keluar immutable setelah posting.

---

## 9. Web & Android Navigation Plan

### 9.1 Web (React)

```
Menu Utama
├── Iuran
│   ├── Daftar Tagihan          → /iuran/bills
│   ├── Pembayaran              → /iuran/payments
│   ├── Tunggakan               → /iuran/overdue
│   └── Laporan                 → /iuran/reports
└── KAS
    ├── Kas Masuk               → /kas/income
    ├── Kas Keluar              → /kas/expense
    ├── Saldo                   → /kas/balance
    └── Laporan                 → /kas/reports
```

### 9.2 Android (Flutter)

```
Bottom Navigation / Menu
├── Iuran
│   ├── Daftar Tagihan
│   ├── Pembayaran
│   ├── Tunggakan
│   └── Laporan
└── KAS
    ├── Kas Masuk
    ├── Kas Keluar
    ├── Saldo
    └── Laporan
```

**Catatan:** Routing dan UI belum diimplementasi. Ini hanya rencana struktur navigasi.

---

## 10. Business Rules — To Be Confirmed

### 10.1 Per KK vs Per Warga

- Apakah iuran dihitung per rumah tangga (KK) atau per warga?
- Jika per KK, apakah semua anggota rumah tangga mendapat tagihan yang sama?
- Jika per warga, apakah ada batasan jumlah warga per tagihan?

### 10.2 Nominal Iuran

- Apakah nominal dapat berbeda antar rumah tangga untuk jenis iuran yang sama?
- Apakah ada jenis iuran yang wajib dan jenis yang opsional?
- Apakah nominal dapat disesuaikan per periode?

### 10.3 Pembayaran

- Apakah pembayaran sebagian (partial payment) diperbolehkan untuk satu tagihan?
- Bagaimana menangani pembayaran untuk bulan/periode sebelumnya (tunggakan)?
- Apakah satu pembayaran dapat mencakup beberapa tagihan dari periode berbeda?
- Apakah pembayaran dapat dibatalkan setelah dikonfirmasi?
- Jika dibatalkan, apakah memerlukan approval?

### 10.4 Reversal & Correction

- Bagaimana reversal dilakukan untuk pembayaran yang salah?
- Apakah transaksi kas harus immutable setelah posting?
- Bagaimana menangani koreksi transaksi kas yang sudah diposting?
- Apakah reversal membuat entri baru atau menandai entri lama sebagai "reversed"?

### 10.5 Ledger & Saldo

- Apakah saldo kas dihitung dari ledger (kalkulasi) atau disimpan sebagai derived value?
- Bagaimana menangani transaksi kas yang belum diposting vs sudah diposting?
- Apakah perlu status "draft" untuk transaksi kas sebelum diposting?

### 10.6 Posting Otomatis vs Manual

- Apakah pembayaran iuran otomatis langsung masuk ke Kas Masuk?
- Atau memerlukan konfirmasi/posting oleh Bendahara?
- Bagaimana menangani pembayaran yang diterima secara manual (tidak melalui sistem)?

### 10.7 Double Posting Prevention

- Bagaimana mencegah pembayaran iuran yang sama tercatat dua kali?
- Apakah menggunakan `Idempotency-Key` pada endpoint pembayaran?
- Apakah ada validasi di level database untuk mencegah duplikasi?

### 10.8 Approval

- Apakah transaksi Kas Keluar memerlukan approval dari Ketua?
- Apakah ada threshold nominal yang memerlukan approval?
- Apakah SUPER_ADMIN dapat override approval?

### 10.9 Attachment / Bukti

- Apakah satu transaksi kas dapat memiliki attachment (foto kwitansi, foto bukti transfer)?
- Apakah pembayaran iuran memerlukan bukti pembayaran (fotografi)?
- Jika ya, bagaimana penyimpanan file (v2+)?

---

## 11. Implementation Roadmap

### Phase A: Requirements & Business Rules

Finalisasi business rules (Section 10) dan permission matrix (Section 6.2). Approval dari stakeholder.

### Phase B: Database Schema & Migration

Desain table schema berdasarkan candidate domain model (Section 7). Menulis migration files (up/down). Menjalankan migration di environment development. Validasi schema constraints (FK, CHECK, UNIQUE).

### Phase C: Backend Repository & Service

Domain model Go structs. Repository methods untuk CRUD `iuran_type`, `iuran_bill`, `iuran_payment`, `kas_transaction`, `kas_category`. Service layer dengan business logic. Unit tests untuk service layer.

### Phase D: Permission Enforcement

Menambah permission codes ke `position_permissions`. Menambah middleware permission checks ke handler. Menguji authorization matrix.

### Phase E: API

Handler HTTP untuk semua endpoint (Section 8). Validation, error handling, pagination. Integration tests untuk API endpoints.

### Phase F: Web UI

Navigasi menu Iuran & KAS di sidebar. Halaman daftar, detail, create, edit. Rekap/laporan (basic tables). Test authorization gates di UI.

### Phase G: Android UI

Navigasi menu Iuran & KAS. Halaman daftar, detail, create, edit. Rekap/laporan. Test authorization gates di UI.

### Phase H: Integration Test & E2E

Test alur lengkap: buat jenis iuran → buat tagihan → bayar → kas masuk → laporan. Test tenant isolation. Test authorization (warga tidak dapat akses, bendahara dapat akses penuh). Test idempotency: duplicate payment tidak membuat double transaction.

### Phase I: Audit & Checkpoint Commit

Final audit: compare implementation vs design doc. Commit checkpoint. Update documentation.

---

## 12. Risks & Design Considerations

### 12.1 Financial Data Integrity

- **Risiko:** Pembayaran iuran tercatat sebagai transaksi kas ganda (double posting).
- **Mitigasi:** `reference_id` unique constraint pada `kas_transaction` yang merujuk ke `iuran_payment`. Idempotency key pada endpoint pembayaran.

### 12.2 Duplicate Payment

- **Risiko:** Warga membayar iuran yang sama dua kali (misalnya via transfer bank dua kali).
- **Mitigasi:** Validasi manual oleh Bendahara untuk konfirmasi pembayaran. `Idempotency-Key` pada API. Flag "pending_verification" untuk pembayaran yang perlu dikonfirmasi.

### 12.3 Duplicate Cash Posting

- **Risiko:** Transaksi kas masuk tercatat tanpa merujuk ke pembayaran iuran yang valid.
- **Mitigasi:** Validasi `reference_id` harus pointing ke record yang ada di `iuran_payment`. Error jika tidak ditemukan.

### 12.4 Reversal & Correction

- **Risiko:** Mengedit transaksi kas yang sudah diposting mengubah ledger history.
- **Mitigasi:** Transaksi kas immutable setelah posting. Koreksi只能通过 reversal (transaksi pembalik dengan jenis berlawanan).

### 12.5 Audit Trail

- **Risiko:** Kehilangan jejak audit untuk pembayaran iuran dan transaksi kas.
- **Mitigasi:** Semua entri (iuran_bill, iuran_payment, kas_transaction) memiliki `created_at`, `updated_at`, `is_active`. Tidak ada DELETE fisik. Soft delete via `is_active`.

### 12.6 Concurrency & Idempotency

- **Risiko:** Dua request pembayaran iuran yang sama diproses secara concurrent.
- **Mitigasi:** `Idempotency-Key` header. Database-level unique constraint pada kombinasi `payment_id + period`.

### 12.7 Authorization

- **Risiko:** Warga mengakses endpoint pembayaran dan membuat pembayaran palsu.
- **Mitigasi:** Semua endpoint memerlukan permission code. Backend middleware enforce. Frontend hanya visibility gate.

### 12.8 Historical Data

- **Risiko:** Migrasi data dari sistem lama ke format iuran baru.
- **Mitigasi:** Script migration khusus untuk memetakan data lama ke `iuran_type`, `iuran_bill`, `iuran_payment`. (Jika diperlukan.)

---

## 13. Explicit Non-Changes

Dokumentasi ini **tidak mengubah** hal-hal berikut:

| Item | Status |
|------|--------|
| Warga module | Tidak diubah |
| Existing authorization system | Tidak diubah |
| Database schema / migration | Tidak dibuat |
| API implementation | Tidak dibuat |
| Web UI (React) | Tidak dibuat |
| Android UI (Flutter) | Tidak dibuat |
| Permission codes | Tidak ditentukan (To Be Confirmed) |
| Existing financial ledger | Tidak diubah |
| Production source code | Tidak diubah |

**Ini adalah dokumen perencanaan. Semua konten di atas adalah kandidat dan belum final.**

---

*End of Document*
