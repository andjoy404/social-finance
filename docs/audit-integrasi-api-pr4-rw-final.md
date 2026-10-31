# Laporan Audit Integrasi API PR 4 — RW Web & Android

**Tanggal:** 2026-10-30
**Status:** READ-ONLY — tidak ada file yang dimodifikasi
**Tujuan:** Menjawab pertanyaan kritis — apakah pengguna RW sudah melihat agregasi data RW melalui endpoint PR 4, atau masih melihat data RT?

---

## KESIMPULAN EKSEKUTIF

### Status: **NOT INTEGRATED — UI MASIH MENGGUNAKAN API RT**

Pengguna dengan konteks RW **TIDAK** melihat agregasi RW. Mereka masih melihat data RT berdasarkan `rt_id` dari JWT token. Keempat endpoint backend PR 4 RW yang terdaftar di backend **sama sekali tidak dipanggil** oleh UI Web maupun Android.

| Endpoint PR 4 | Backend Terdaftar? | API Client Web? | API Client Android? | Dipanggil UI? |
|---------------|-------------------|-----------------|---------------------|---------------|
| `GET /api/v1/rw/{rwId}/dashboard` | ✅ `main.go:753` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |
| `GET /api/v1/rw/{rwId}/kas?period=YYYY-MM` | ✅ `main.go:754` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |
| `GET /api/v1/rw/{rwId}/kas/transactions` | ✅ `main.go:755` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |
| `GET /api/v1/rw/{rwId}/locations?is_active=true` | ✅ `main.go:756` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |

### API yang SEBENARNYA DIGUNAKAN Saat Ini

| Menu | Web API | Android API | Endpoint |
|------|---------|-------------|----------|
| Beranda | `apiGetDashboardSummary(token, period)` di `api.ts:2138` | `kasApiClient.getDashboardSummary(period)` di `kas_api_client.dart:220` | `GET /api/v1/dashboard/summary` (RT-scoped) |
| KAS | `apiListTransactions(token, params)` di `api.ts:1984` | Via `kas_repository.dart` | `GET /api/v1/kas/transactions` (RT-scoped) |
| Lokasi Dana | `apiListFundLocations(token, params)` di `api.ts:2434` | `fundLocationsProvider.listLocations()` di `fund_location_api_client.dart:24` | `GET /api/v1/fund-locations` (RT-scoped) |

---

## 1. AUDIT BERANDA (DASHBOARD)

### Backend — Endpoint PR 4 (tersedia tapi tidak dipanggil)

**File:** `backend/cmd/api/main.go` baris 753
```go
r.Get("/api/v1/rw/{rwId}/dashboard", rwHandler.GetDashboard)
```

### Web — Yang SEDANG DIPANGGIL

**File:** `frontend/src/features/dashboard/Dashboard.tsx` baris 66

```tsx
apiGetDashboardSummary(session.accessToken, activePeriodStr)
```

**API Client:** `frontend/src/app/api.ts` baris 2138-2151

```typescript
export async function apiGetDashboardSummary(token: string, period?: string): Promise<ApiDashboardSummary> {
  const url = period ? `${BASE}/dashboard/summary?period=${encodeURIComponent(period)}` 
                  : `${BASE}/dashboard/summary`
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json<ApiDashboardSummary>(res)
}
```

**Bukti endpoint yang dipanggil:**
- URL: `GET /api/v1/dashboard/summary` (BUKAN `/api/v1/rw/{rwId}/dashboard`)
- Parameter: `token`, `period` — **TIDAK ADA `rw_id`**
- Tidak ada pemanggilan ke `/api/v1/rw/` sama sekali di `frontend/src/` (0 kecocokan)

**Scope data:** Baris 63 — `if (!session?.accessToken || !user?.rt?.id)` — scoped ke RT via JWT claim

### Android — Yang SEDANG DIPANGGIL

**File:** `mobile/lib/features/dashboard/presentation/screens/dashboard_screen.dart` baris 155

```dart
final json = await api.getDashboardSummary(periodParam);
```

**API Client:** `mobile/lib/features/kas/data/kas_api_client.dart` baris 220-226

```dart
Future<Map<String, dynamic>> getDashboardSummary([String? period]) async {
  final response = await dio.get(
    '/api/v1/dashboard/summary',
    queryParameters: period != null ? {'period': period} : null,
  );
  return response.data.cast<String, dynamic>();
}
```

**Bukti endpoint yang dipanggil:**
- URL: `GET /api/v1/dashboard/summary` (BUKAN `/api/v1/rw/{rwId}/dashboard`)
- Parameter: `period` — **TIDAK ADA `rw_id`**
- Tidak ada pemanggilan ke `/api/v1/rw/` sama sekali di `mobile/lib/` (0 kecocokan)

### Verifikasi

| Aspek | PR 4 (Expected) | Web (Actual) | Android (Actual) |
|-------|----------------|--------------|------------------|
| Endpoint | `/api/v1/rw/{rwId}/dashboard` | `/api/v1/dashboard/summary` | `/api/v1/dashboard/summary` |
| Parameter `rw_id` | ✅ Path param `{rwId}` | ❌ TIDAK ADA | ❌ TIDAK ADA |
| Parameter `period` | ✅ Query param | ✅ | ✅ |
| Data scope | Agregat seluruh RT di bawah RW | Data RT saja (JWT `rt_id`) | Data RT saja (JWT `rt_id`) |
| **Status** | **READY** | **NOT INTEGRATED** | **NOT INTEGRATED** |

---

## 2. AUDIT KAS (TRANSAKSI)

### Backend — Endpoint PR 4 (tersedia tapi tidak dipanggil)

**File:** `backend/cmd/api/main.go` baris 754-755
```go
r.Get("/api/v1/rw/{rwId}/kas", rwHandler.GetKAS)
r.Get("/api/v1/rw/{rwId}/kas/transactions", rwHandler.GetKASTransactions)
```

### Web — Yang SEDANG DIPANGGIL

**File:** `frontend/src/features/kas/KasList.tsx` (implementasi tidak ditampilkan di sini, tetapi import API-nya ada)

**API Client:** `frontend/src/app/api.ts` baris 1984-2015

```typescript
export async function apiListTransactions(
  token: string,
  params?: { page?: number; page_size?: number; category_id?: string; type?: string; status?: string; period?: string; ... },
): Promise<ApiTransactionPaginated> {
  const q = new URLSearchParams()
  // ... parameter handling
  const res = await authenticatedApiRequest(BASE + '/kas/transactions' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json(res)
}
```

**Bukti endpoint yang dipanggil:**
- URL: `GET /api/v1/kas/transactions` (BUKAN `/api/v1/rw/{rwId}/kas/transactions`)
- Parameter: `period`, `page`, `page_size`, `search`, `arus`, `metode` — **TIDAK ADA `rw_id`**

### Android — Yang SEDANG DIPANGGIL

**API Client:** `mobile/lib/features/kas/data/kas_api_client.dart` (endpoint yang sama: `/api/v1/kas/transactions`)

**Bukti endpoint yang dipanggil:**
- URL: `GET /api/v1/kas/transactions` (BUKAN `/api/v1/rw/{rwId}/kas/transactions`)
- Parameter: `period`, `arus`, `metode`, `search`, `page`, `pageSize` — **TIDAK ADA `rw_id`**

### Verifikasi

| Aspek | PR 4 (Expected) | Web (Actual) | Android (Actual) |
|-------|----------------|--------------|------------------|
| Endpoint | `/api/v1/rw/{rwId}/kas` / `.../kas/transactions` | `/api/v1/kas/transactions` | `/api/v1/kas/transactions` |
| Parameter `rw_id` | ✅ Path param `{rwId}` | ❌ TIDAK ADA | ❌ TIDAK ADA |
| Filter period | ✅ `period=YYYY-MM` | ✅ | ✅ |
| Filter arus | ✅ `arus=masuk|keluar` | ✅ | ✅ |
| Filter metode | ✅ `metode=transfer|cash` | ✅ | ✅ |
| Search | ✅ `search=...` | ✅ | ✅ |
| Pagination | ✅ `page`, `page_size` | ✅ | ✅ |
| Data scope | Agregat seluruh RT di bawah RW | Data RT saja | Data RT saja |
| **Status** | **READY** | **NOT INTEGRATED** | **NOT INTEGRATED** |

### Catatan KAS Read-Only untuk RW

KAS RW harus read-only. Saat ini, baik Web maupun Android tidak menampilkan form input transaksi (pemasukan/pengeluaran) di halaman KasList, sehingga secara default sudah read-only. Namun ini adalah read-only karena tidak ada tombol create, bukan karena enforced oleh permission.

---

## 3. AUDIT LOKASI DANA

### Backend — Endpoint PR 4 (tersedia tapi tidak dipanggil)

**File:** `backend/cmd/api/main.go` baris 756
```go
r.Get("/api/v1/rw/{rwId}/locations", rwHandler.GetLocations)
```

### Web — Yang SEDANG DIPANGGIL

**API Client:** `frontend/src/app/api.ts` baris 2434-2449

```typescript
export async function apiListFundLocations(
  token: string,
  params?: { category?: string; is_active?: boolean },
): Promise<FundLocation[]> {
  const q = new URLSearchParams()
  if (params?.category) q.set('category', params.category)
  if (params?.is_active != null) q.set('is_active', String(params.is_active))
  const res = await authenticatedApiRequest(BASE + '/fund-locations' + (q.toString() ? '?' + q.toString() : ''), {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json<FundLocation[]>(res)
}
```

**Bukti endpoint yang dipanggil:**
- URL: `GET /api/v1/fund-locations` (BUKAN `/api/v1/rw/{rwId}/locations`)
- Parameter: `category`, `is_active` — **TIDAK ADA `rw_id`**

### Android — Yang SEDANG DIPANGGIL

**API Client:** `mobile/lib/features/fund_location/data/fund_location_api_client.dart` baris 24

```dart
Future<List<FundLocation>> listLocations({
  String? category,
  bool? isActive,
}) async {
  final queryParams = <String, dynamic>{};
  if (category != null && category.isNotEmpty) {
    queryParams['category'] = category;
  }
  if (isActive != null) {
    queryParams['is_active'] = isActive;
  }
  final response = await dio.get(
    '/api/v1/fund-locations',
    queryParameters: queryParams,
  );
  // ...
}
```

**Bukti endpoint yang dipanggil:**
- URL: `GET /api/v1/fund-locations` (BUKAN `/api/v1/rw/{rwId}/locations`)
- Parameter: `category`, `is_active` — **TIDAK ADA `rw_id`**

### Verifikasi Lokasi Dana — Read-Only untuk RW

Lokasi Dana RW harus read-only (tidak boleh tambah, edit, hapus, transfer, atau ubah saldo). Saat ini:

**Web `FundLocationList.tsx` menampilkan:**
- Button "Tambah Lokasi" — action mutasi
- Button "Transfer Dana" — action mutasi
- Button "Alokasi Saldo Awal" — action mutasi
- Edit/Delete buttons di table rows — action mutasi

**Android `fund_location_screen.dart` menampilkan:**
- FAB "Tambah Lokasi" — action mutasi
- Button "Transfer" — action mutasi
- Button "Alokasi" di reconciliation card — action mutasi

**Jika RW hanya boleh read-only, semua aksi mutasi ini perlu dinonaktifkan untuk konteks RW.**

### Verifikasi

| Aspek | PR 4 (Expected) | Web (Actual) | Android (Actual) |
|-------|----------------|--------------|------------------|
| Endpoint | `/api/v1/rw/{rwId}/locations` | `/api/v1/fund-locations` | `/api/v1/fund-locations` |
| Parameter `rw_id` | ✅ Path param `{rwId}` | ❌ TIDAK ADA | ❌ TIDAK ADA |
| Filter category | ✅ `category=bank|brankas|cash` | ✅ | ✅ |
| Filter is_active | ✅ `is_active=true|false` | ✅ | ✅ |
| Data scope | Agregat lokasi dari seluruh RT | Data RT saja | Data RT saja |
| Read-only RW | Harus read-only | Ada tombol mutasi | Ada tombol mutasi |
| **Status** | **READY** | **NOT INTEGRATED** | **NOT INTEGRATED** |

---

## 4. AUDIT ROLE & KONTEKS

### Bagaimana Aplikasi Membedakan Pengguna Saat Ini

**Sumber data role dan konteks:**

| Sumber | Web | Android |
|--------|-----|---------|
| JWT Token | `rt_id` dalam JWT claim | `rt_id` dalam user object |
| API Response | `apiMe()` mengembalikan `rt: {id, name}` | `auth_repository_provider` |
| Context RW | `rw_id` di `ApiUser.rw_id` (baris 24 `api.ts`) | `rwId` di `ApiUser` (baris 11 `models.dart`) |
| RW Membership | `apiListRWMemberships(token, rwId)` | Belum ada API client untuk ini |

**Observasi:**
1. `rw_id` dan `rwId` sudah ada di models API, tapi **TIDAK digunakan untuk routing** ke endpoint PR 4.
2. Tidak ada mekanisme untuk memilih konteks RW vs RT saat ini.
3. Semua fitur operasional memanggil endpoint RT-scoped yang scoped oleh `rt_id` dari JWT.
4. Jika user memiliki jabatan di RW (bukan Ketua RT), user tersebut tetap melihat data RT-nya saja, bukan agregat RW.

### Perbedaan yang Perlu Ada

| Sumber | Saat Ini | Seharusnya (setelah PR 4) |
|--------|----------|---------------------------|
| JWT claim | `rt_id` dari JWT | `rw_id` harus tersedia jika user punya jabatan RW |
| Scope data | `rt_id` dari JWT | `rw_id` dari user context → endpoint PR 4 |
| Route | `/kas`, `/lokasi-dana`, `/dashboard` | `/rw/{rwId}/kas`, `/rw/{rwId}/locations`, `/rw/{rwId}/dashboard` |
| Authorization | `AllowedRTRoute` (role-based) | `AllowedRWRoute` (cek `rw_id` user vs path param) |

---

## 5. Matriks Hasil Integrasi

| Menu | Platform | API saat ini | API yang seharusnya | Endpoint yang dipanggil | Status Integrasi |
|------|----------|--------------|---------------------|------------------------|------------------|
| **Beranda** | Web | `apiGetDashboardSummary(token, period)` di `api.ts:2138` | `apiGetRWDashboard(token, rwId, period)` | `GET /api/v1/dashboard/summary` | **NOT INTEGRATED** |
| **Beranda** | Android | `kasApiClient.getDashboardSummary(period)` di `kas_api_client.dart:220` | `rwApiClient.getDashboard(rwId, period)` | `GET /api/v1/dashboard/summary` | **NOT INTEGRATED** |
| **KAS** | Web | `apiListTransactions(token, params)` di `api.ts:1984` | `apiListRWTransactions(token, rwId, params)` | `GET /api/v1/kas/transactions` | **NOT INTEGRATED** |
| **KAS** | Android | `kasRepository.getTransactions(params)` | `rwKasRepository.getTransactions(rwId, params)` | `GET /api/v1/kas/transactions` | **NOT INTEGRATED** |
| **Lokasi Dana** | Web | `apiListFundLocations(token, params)` di `api.ts:2434` | `apiListRWLocations(token, rwId, params)` | `GET /api/v1/fund-locations` | **NOT INTEGRATED** |
| **Lokasi Dana** | Android | `fundLocationsProvider.listLocations()` di `fund_location_api_client.dart:24` | `rwFundLocationsProvider.listLocations(rwId)` | `GET /api/v1/fund-locations` | **NOT INTEGRATED** |

---

## 6. Rekomendasi Implementasi Minimal

### WEB — Perubahan yang Diperlukan

#### 1. Buat API Client RW Baru

**File baru:** `frontend/src/app/rw-api.ts`

```typescript
import { authenticatedApiRequest, BASE, json, ApiError } from './api'

export interface ApiRWDashboardSummary {
  balance: string
  monthly_income: string
  monthly_expense: string
  chart: Array<{ month: string; income: string; expense: string }>
  dues_progress: { paid_households: number; total_households: number; percentage: number }
  payment_method_breakdown: { transfer_amount: string; cash_amount: string; transfer_count: number; cash_count: number }
  recent_transactions: Array<ApiTransaction>
  total_rt_count: number
}

export interface ApiRWTransaction {
  id: string
  type: 'income' | 'expense'
  amount: string
  description: string
  category_name?: string
  payment_method?: string
  occurred_at?: string
  created_at: string
  rt_id?: string       // ← penting: identitas RT asal
  rt_name?: string     // ← penting: nama RT asal
  // ... field lainnya
}

export interface ApiRWLocation {
  id: string
  name: string
  category: string
  balance: string
  rt_id: string        // ← penting: identitas RT asal
  rt_name?: string     // ← penting: nama RT asal
  is_active: boolean
  // ... field lainnya
}

export async function apiGetRWDashboard(token: string, rwId: string, period?: string): Promise<ApiRWDashboardSummary> {
  const url = period 
    ? `${BASE}/rw/${encodeURIComponent(rwId)}/dashboard?period=${encodeURIComponent(period)}`
    : `${BASE}/rw/${encodeURIComponent(rwId)}/dashboard`
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json<ApiRWDashboardSummary>(res)
}

export async function apiGetRWKAS(token: string, rwId: string, period?: string): Promise<{
  balance: string
  total_income: string
  total_expense: string
  period_count: number
}> {
  const url = period
    ? `${BASE}/rw/${encodeURIComponent(rwId)}/kas?period=${encodeURIComponent(period)}`
    : `${BASE}/rw/${encodeURIComponent(rwId)}/kas`
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json(res)
}

export async function apiListRWTransactions(
  token: string, 
  rwId: string, 
  params: { period?: string; arus?: string; metode?: string; search?: string; page?: number; page_size?: number }
): Promise<{ data: ApiRWTransaction[]; pagination: { page: number; page_size: number; total: number; total_pages: number } }> {
  const q = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== '') q.set(k, String(v))
  })
  const res = await authenticatedApiRequest(`${BASE}/rw/${encodeURIComponent(rwId)}/kas/transactions?${q.toString()}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json(res)
}

export async function apiListRWLocations(
  token: string, 
  rwId: string, 
  params?: { category?: string; is_active?: boolean }
): Promise<ApiRWLocation[]> {
  const q = new URLSearchParams()
  if (params?.category) q.set('category', params.category)
  if (params?.is_active != null) q.set('is_active', String(params.is_active))
  const res = await authenticatedApiRequest(`${BASE}/rw/${encodeURIComponent(rwId)}/locations?${q.toString()}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json<ApiRWLocation[]>(res)
}
```

#### 2. Buat Context/Provider untuk Konteks RW

```typescript
// frontend/src/app/RWContext.tsx
// Menyimpan rw_id saat ini, dipanggil berdasarkan:
// 1. Route parameter dari URL path
// 2. RW memberships dari apiMe() response
// 3. User selection jika user memiliki multiple RW roles
```

#### 3. Buat Komponen RW yang Mengadaptasi API Baru

**Struktur minimal:**
```
frontend/src/features/rw/
├── RwDashboard.tsx         ← REUSE dari Dashboard.tsx, ganti data source
├── RwKasList.tsx           ← REUSE dari KasList.tsx, ganti data source, tampilkan rt_name
└── RwFundLocationList.tsx  ← REUSE dari FundLocationList.tsx, ganti data source, disable mutasi
```

**Prinsip:**
- `RwDashboard.tsx`: Gunakan komponen UI yang sama dari `Dashboard.tsx`, tapi panggil `apiGetRWDashboard()` alih-alih `apiGetDashboardSummary()`. Tambahkan indikator "Agregat RW {nama}" di header.
- `RwKasList.tsx`: Gunakan komponen UI yang sama dari `KasList.tsx`, tapi panggil `apiListRWTransactions()`. Tampilkan kolom `rt_name` untuk menunjukkan asal transaksi. Disable semua form input (read-only).
- `RwFundLocationList.tsx`: Gunakan komponen UI yang sama dari `FundLocationList.tsx`, tapi panggil `apiListRWLocations()`. Tampilkan kolom `rt_name`. Disable semua tombol mutasi (Tambah, Edit, Delete, Transfer, Opening Balance).

#### 4. Tambah Route RW

```tsx
// frontend/src/app/App.tsx — tambahkan setelah route /rt
<Route path="rw/:rwId" element={<AllowedRWRoute rwRequired={true}><RwDashboard /></AllowedRWRoute>} />
<Route path="rw/:rwId/kas" element={<AllowedRWRoute rwRequired={true}><RwKasList /></AllowedRWRoute>} />
<Route path="rw/:rwId/lokasi-dana" element={<AllowedRWRoute rwRequired={true}><RwFundLocationList /></AllowedRWRoute>} />
```

### ANDROID — Perubahan yang Diperlukan

#### 1. Buat API Client RW Baru

**File baru:** `mobile/lib/features/rw/data/rw_api_client.dart`

```dart
class RwApiClient {
  final Dio dio;
  
  RwApiClient({required this.dio});
  
  /// GET /api/v1/rw/{rwId}/dashboard
  Future<Map<String, dynamic>> getDashboard(String rwId, [String? period]) async {
    final uri = '/api/v1/rw/$rwId/dashboard';
    final response = await dio.get(
      uri,
      queryParameters: period != null ? {'period': period} : null,
    );
    return response.data as Map<String, dynamic>;
  }
  
  /// GET /api/v1/rw/{rwId}/kas
  Future<Map<String, dynamic>> getKAS(String rwId, [String? period]) async {
    final uri = '/api/v1/rw/$rwId/kas';
    final response = await dio.get(
      uri,
      queryParameters: period != null ? {'period': period} : null,
    );
    return response.data as Map<String, dynamic>;
  }
  
  /// GET /api/v1/rw/{rwId}/kas/transactions
  Future<Map<String, dynamic>> getTransactions(String rwId, {
    String? period, String? arus, String? metode,
    String? search, int? page, int? pageSize,
  }) async {
    final queryParams = <String, dynamic>{};
    if (period != null) queryParams['period'] = period;
    if (arus != null) queryParams['arus'] = arus;
    if (metode != null) queryParams['metode'] = metode;
    if (search?.isNotEmpty ?? false) queryParams['search'] = search;
    if (page != null) queryParams['page'] = page;
    if (pageSize != null) queryParams['page_size'] = pageSize;
    
    final response = await dio.get(
      '/api/v1/rw/$rwId/kas/transactions',
      queryParameters: queryParams,
    );
    return response.data as Map<String, dynamic>;
  }
  
  /// GET /api/v1/rw/{rwId}/locations
  Future<List<Map<String, dynamic>>> getLocations(String rwId, {
    String? category, bool? isActive,
  }) async {
    final queryParams = <String, dynamic>{};
    if (category?.isNotEmpty ?? false) queryParams['category'] = category;
    if (isActive != null) queryParams['is_active'] = isActive;
    
    final response = await dio.get(
      '/api/v1/rw/$rwId/locations',
      queryParameters: queryParams,
    );
    // Parse response...
  }
}
```

#### 2. Buat Screen RW

**Struktur minimal:**
```
mobile/lib/features/rw/
├── data/
│   ├── rw_api_client.dart     ← API client baru
│   ├── rw_models.dart         ← Model response (jika berbeda dari RT)
│   └── rw_providers.dart      ← Riverpod providers baru
└── presentation/
    └── screens/
        ├── rw_dashboard_screen.dart    ← REUSE dari dashboard_screen.dart
        ├── rw_kas_screen.dart          ← REUSE dari kas_screen.dart
        └── rw_fund_location_screen.dart ← REUSE dari fund_location_screen.dart + disable mutasi
```

#### 3. Tambah Navigasi RW

```dart
// mobile/lib/app/navigation_shell.dart — tambahkan di switch routing
case '/home/rw/:rwId':
  return RWDashboardScreen(rwId: routingData.pathParameters['rwId']!);
case '/home/rw/:rwId/kas':
  return RWKasScreen(rwId: routingData.pathParameters['rwId']!);
case '/home/rw/:rwId/lokasi-dana':
  return RWFundLocationScreen(rwId: routingData.pathParameters['rwId']!);
```

---

## 7. Matriks Reuse UI

### Web — Komponen yang Bisa DI-REUSE

| Komponen RT | Lokasi | RW Adaptasi | Status |
|-------------|--------|-------------|--------|
| DashboardSummary cards | `Dashboard.tsx:340-360` | REUSE — ganti data source | ✅ REUSE dengan adapter |
| FinancialChart | `Dashboard.tsx:364-374` | REUSE — struktur data sama | ✅ REUSE langsung |
| Donut Payment Method | `Dashboard.tsx:412-491` | REUSE — struktur data sama | ✅ REUSE langsung |
| Iuran Progress | `Dashboard.tsx:382-408` | REUSE — struktur data sama | ✅ REUSE langsung |
| Recent Transactions table | `Dashboard.tsx:495-565` | REUSE — struktur data sama | ✅ REUSE langsung |
| KasList | `KasList.tsx` | REUSE — ganti API call | ✅ REUSE dengan adapter |
| KasIncome/Expense forms | `KasIncome.tsx`, `KasExpense.tsx` | DISABLE untuk RW (read-only) | ✅ REUSE dengan hide forms |
| FundLocationList | `FundLocationList.tsx` | REUSE — ganti API call + disable mutasi | ✅ REUSE dengan adapter + permission |

### Android — Widget yang Bisa DI-REUSE

| Widget RT | Lokasi | RW Adaptasi | Status |
|-----------|--------|-------------|--------|
| SummaryCard | `dashboard_screen.dart` | REUSE — ganti data source | ✅ REUSE dengan adapter |
| FinancialSummaryChart | `dashboard_screen.dart` | REUSE — struktur data sama | ✅ REUSE langsung |
| Donut payment | `dashboard_screen.dart` | REUSE — struktur data sama | ✅ REUSE langsung |
| KasTransactionCard | `kas_screen.dart` | REUSE — struktur data sama | ✅ REUSE langsung |
| KasScreen | `kas_screen.dart` | REUSE — ganti API call | ✅ REUSE dengan adapter |
| Form kas_masuk/keluar | `kas_masuk_screen.dart` | DISABLE untuk RW (read-only) | ✅ REUSE dengan hide forms |
| FundLocationScreen | `fund_location_screen.dart` | REUSE — ganti API call + disable mutasi | ✅ REUSE dengan adapter + permission |

---

## 8. Checklist Read-Only untuk Lokasi Dana RW

### Web — `FundLocationList.tsx`

| Aksi Mutasi | Ada di UI? | Perlu Disables untuk RW? |
|-------------|------------|-------------------------|
| Tambah Lokasi | ✅ Ada | ✅ Ya |
| Transfer Dana | ✅ Ada | ✅ Ya |
| Alokasi Saldo Awal | ✅ Ada | ✅ Ya |
| Edit Lokasi | ✅ Ada | ✅ Ya |
| Hapus Lokasi | ✅ Ada | ✅ Ya |
| Reverse Transfer | ✅ Ada | ✅ Ya |

### Android — `fund_location_screen.dart`

| Aksi Mutasi | Ada di UI? | Perlu Disables untuk RW? |
|-------------|------------|-------------------------|
| Tambah Lokasi (FAB) | ✅ Ada | ✅ Ya |
| Transfer | ✅ Ada | ✅ Ya |
| Alokasi (reconciliation card) | ✅ Ada | ✅ Ya |
| Edit Lokasi | ✅ Ada | ✅ Ya |
| Hapus Lokasi | ✅ Ada | ✅ Ya |
| Reverse Transfer | ✅ Ada | ✅ Ya |

---

## 9. Kesimpulan Akhir

### Status Per Menu

| Menu | Status | Deskripsi |
|------|--------|-----------|
| **Beranda** | **NOT INTEGRATED** | Masih menggunakan `GET /api/v1/dashboard/summary` (RT-scoped), bukan `GET /api/v1/rw/{rwId}/dashboard` |
| **KAS** | **NOT INTEGRATED** | Masih menggunakan `GET /api/v1/kas/transactions` (RT-scoped), bukan `GET /api/v1/rw/{rwId}/kas/transactions` |
| **Lokasi Dana** | **NOT INTEGRATED** | Masih menggunakan `GET /api/v1/fund-locations` (RT-scoped), bukan `GET /api/v1/rw/{rwId}/locations` |

### Kesimpulan Utama

1. **Backend PR 4 sudah siap** — 4 endpoint terdaftar di `backend/cmd/api/main.go:753-756`.
2. **UI Web dan Android BELUM terintegrasi** — semua fitur memanggil endpoint RT yang lama.
3. **Pengguna RW saat ini melihat data RT, bukan agregat RW** — gap kritis yang perlu segera ditangani.
4. **Komponen UI bisa di-REUSE** — perbedaan hanya pada data source (API call), bukan pada tampilan atau layout.
5. **Perlu permission check untuk read-only** — Lokasi Dana RW harus menonaktifkan semua tombol mutasi.
6. **Strategi implementasi minimal** — cukup buat layer API client baru yang memanggil endpoint PR 4, lalu gunakan kembali komponen UI RT yang sudah ada.

### Rekomendasi Prioritas

| Prioritas | Tugas | Platform |
|-----------|-------|----------|
| **HIGH** | Buat API client RW baru (`rw-api.ts` Web, `rw_api_client.dart` Android) | Web + Android |
| **HIGH** | Buat context/provider untuk menyimpan `rw_id` saat ini | Web + Android |
| **MEDIUM** | Buat komponen RW Dashboard dengan read-only indicator | Web + Android |
| **MEDIUM** | Buat komponen RW Kas dengan kolom `rt_name` dan disable form | Web + Android |
| **MEDIUM** | Buat komponen RW Lokasi Dana dengan disable semua mutasi | Web + Android |
| **LOW** | Tambah navigasi RW di sidebar Web dan bottom nav Android | Web + Android |
