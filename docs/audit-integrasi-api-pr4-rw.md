# Laporan Audit Integrasi API PR 4 — RW Web & Android

**Tanggal:** 2026-10-30
**Status:** READ-ONLY — tidak ada file yang dimodifikasi
**Tujuan:** Memverifikasi apakah 4 endpoint backend PR 4 sudah dipanggil oleh UI Web dan Android

---

## EKSEKUTIF SUMMARY

### Status: **NOT INTEGRATED — UI MASIH MENGGUNAKAN API RT**

Keempat endpoint backend PR 4 RW **TIDAK DIPANGGIL sama sekali** oleh UI Web maupun Android.

| Endpoint PR 4 | Backend Terdaftar? | API Client Web? | API Client Android? | Dipanggil UI? |
|---------------|-------------------|-----------------|---------------------|---------------|
| `GET /api/v1/rw/{rwId}/dashboard` | ✅ `backend/cmd/api/main.go:753` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |
| `GET /api/v1/rw/{rwId}/kas` | ✅ `backend/cmd/api/main.go:754` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |
| `GET /api/v1/rw/{rwId}/kas/transactions` | ✅ `backend/cmd/api/main.go:755` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |
| `GET /api/v1/rw/{rwId}/locations` | ✅ `backend/cmd/api/main.go:756` | ❌ Tidak ada | ❌ Tidak ada | ❌ TIDAK |

### API yang SEDANG DIGUNAKAN (sebelum PR 4)

| Menu | Web API Client | Android API Client | Endpoint yang Dipanggil |
|------|---------------|-------------------|------------------------|
| Beranda | `apiGetDashboardSummary()` di `api.ts:2138` | `kasApiClient.getDashboardSummary()` di `kas_api_client.dart:220` | `GET /api/v1/dashboard/summary` (RT-scoped) |
| KAS | `apiListTransactions()` di `api.ts:1984` | `kasRepository.getTransactions()` via `kas_api_client.dart` | `GET /api/v1/kas/transactions` (RT-scoped) |
| Lokasi Dana | `apiListFundLocations()` di `api.ts:2434` | `fundLocationsProvider.listLocations()` di `fund_location_api_client.dart:24` | `GET /api/v1/fund-locations` (RT-scoped) |

### Kesimpulan Kritis

**Pengguna dengan konteks RW SAAT INI masih melihat data RT, bukan data agregat RW.** Dashboard, KAS, dan Lokasi Dana memanggil endpoint yang scoped ke `rt_id` dari JWT token, bukan ke `rw_id` dari PR 4.

---

## 1. AUDIT BERANDA (DASHBOARD)

### Backend — PR 4 Endpoint

**File:** `backend/cmd/api/main.go` baris 753
```go
r.Get("/api/v1/rw/{rwId}/dashboard", rwHandler.GetDashboard)
```
✅ Terdaftar, menggunakan `rwHandler.GetDashboard`

### Web — Current Implementation

**File:** `frontend/src/features/dashboard/Dashboard.tsx` baris 66

```tsx
apiGetDashboardSummary(session.accessToken, activePeriodStr)
```

**API Client:** `frontend/src/app/api.ts` baris 2138

```typescript
export async function apiGetDashboardSummary(token: string, period?: string): Promise<ApiDashboardSummary> {
  const url = period ? `${BASE}/dashboard/summary?period=${encodeURIComponent(period)}` 
                  : `${BASE}/dashboard/summary`
  // ...
}
```

**Endpoint yang dipanggil:** `GET /api/v1/dashboard/summary` (bukan `/api/v1/rw/{rwId}/dashboard`)

**Parameter yang dikirim:**
- `token` — JWT access token
- `period` — bulan/tahun atau "all"

**Parameter yang TIDAK dikirim:**
- `rw_id` — tidak ada
- Tidak ada cara membedakan konteks RW vs RT

**Data source:** User RT-scoped via JWT claim `rt_id` (baris 63: `if (!session?.accessToken || !user?.rt?.id)`)

### Android — Current Implementation

**File:** `mobile/lib/features/dashboard/presentation/screens/dashboard_screen.dart` baris 155

```dart
final json = await api.getDashboardSummary(periodParam);
```

**API Client:** `mobile/lib/features/kas/data/kas_api_client.dart` baris 220

```dart
Future<Map<String, dynamic>> getDashboardSummary([String? period]) async {
  final response = await dio.get(
    '/api/v1/dashboard/summary',
    queryParameters: period != null ? {'period': period} : null,
  );
  // ...
}
```

**Endpoint yang dipanggil:** `GET /api/v1/dashboard/summary` (bukan `/api/v1/rw/{rwId}/dashboard`)

**Parameter yang dikirim:**
- `period` — bulan/tahun atau "all"

**Parameter yang TIDAK dikirim:**
- `rw_id` — tidak ada

### Perbandingan Beranda

| Aspek | Backend PR 4 (Expected) | Web (Actual) | Android (Actual) | Status |
|-------|------------------------|--------------|------------------|--------|
| Endpoint | `/api/v1/rw/{rwId}/dashboard` | `/api/v1/dashboard/summary` | `/api/v1/dashboard/summary` | ❌ MISMATCH |
| Parameter rw_id | `{rwId}` path param | TIDAK ADA | TIDAK ADA | ❌ MISSING |
| Parameter period | Query `period=YYYY-MM` | ✅ `period` query | ✅ `period` query | ✅ MATCH |
| Data scope | Agregat seluruh RT di bawah RW | Data RT saja (JWT `rt_id`) | Data RT saja (JWT `rt_id`) | ❌ MISMATCH |
| Loading state | — | ✅ `useState(true)` + `loading` | ✅ `CircularProgressIndicator` | ✅ MATCH |
| Error state | — | ✅ `.catch(console.error)` | ✅ `catch (_)` fallback | ✅ MATCH |
| Empty state | — | Tidak ada explicit empty state | Tidak ada explicit empty state | ⚠️ MINOR |

### Kesimpulan Beranda
**NOT INTEGRATED**

Web dan Android memanggil `GET /api/v1/dashboard/summary` yang RT-scoped, bukan `GET /api/v1/rw/{rwId}/dashboard` dari PR 4.

---

## 2. AUDIT KAS (TRANSAKSI)

### Backend — PR 4 Endpoint

**File:** `backend/cmd/api/main.go` baris 754-755
```go
r.Get("/api/v1/rw/{rwId}/kas", rwHandler.GetKAS)
r.Get("/api/v1/rw/{rwId}/kas/transactions", rwHandler.GetKASTransactions)
```
✅ Terdaftar, menggunakan `rwHandler`

### Web — Current Implementation

**File:** `frontend/src/features/kas/KasList.tsx`

```typescript
import { apiListTransactions, ApiKasTransaction } from '../kas/kas-api'

const loadTransactions = useCallback(async () => {
  if (!token) return
  setTransactions(await apiListTransactions(token, {
    period: selectedPeriod,
    page: page,
    page_size: pageSize,
    search: searchInput || undefined,
    arus: filterArus || undefined,
    metode: filterMetode || undefined,
  }))
}, [token, selectedPeriod, page, pageSize, searchInput, filterArus, filterMetode])
```

**API Client:** `frontend/src/app/api.ts` baris 1984

```typescript
export async function apiListTransactions(
  token: string,
  params?: { page?: number; page_size?: number; category_id?: string; type?: string; status?: string; period?: string; ... },
): Promise<ApiTransactionPaginated> {
  // ...
  const res = await authenticatedApiRequest(BASE + '/kas/transactions' + (q.toString() ? '?' + q.toString() : ''), {
    // ...
  })
}
```

**Endpoint yang dipanggil:** `GET /api/v1/kas/transactions` (bukan `/api/v1/rw/{rwId}/kas/transactions`)

**Parameter yang dikirim:**
- `token`, `period`, `page`, `page_size`, `search`, `arus`, `metode`

**Parameter yang TIDAK dikirim:**
- `rw_id` — tidak ada

### Android — Current Implementation

**File:** `mobile/lib/features/kas/presentation/screens/kas_screen.dart`

```dart
final transactions = await api.getTransactions(
  periodParam: period,
  arus: _filterArus.value,
  metode: _filterMetode.value,
  search: _searchController.text,
  page: currentPage,
  pageSize: pageSize,
);
```

**API Client:** `mobile/lib/features/kas/data/kas_repository.dart` / `kas_api_client.dart`

**Endpoint yang dipanggil:** `GET /api/v1/kas/transactions` (bukan `/api/v1/rw/{rwId}/kas/transactions`)

**Parameter yang dikirim:**
- `period`, `arus`, `metode`, `search`, `page`, `pageSize`

**Parameter yang TIDAK dikirim:**
- `rw_id` — tidak ada

### Perbandingan KAS

| Aspek | Backend PR 4 (Expected) | Web (Actual) | Android (Actual) | Status |
|-------|------------------------|--------------|------------------|--------|
| Endpoint | `/api/v1/rw/{rwId}/kas` / `.../kas/transactions` | `/api/v1/kas/transactions` | `/api/v1/kas/transactions` | ❌ MISMATCH |
| Parameter rw_id | `{rwId}` path param | TIDAK ADA | TIDAK ADA | ❌ MISSING |
| Period filter | Query `period=YYYY-MM` | ✅ | ✅ | ✅ |
| Arus filter | Query `arus=masuk|keluar` | ✅ | ✅ | ✅ |
| Metode filter | Query `metode=transfer|cash` | ✅ | ✅ | ✅ |
| Search | Query `search=...` | ✅ | ✅ | ✅ |
| Pagination | Query `page`, `page_size` | ✅ | ✅ | ✅ |
| Data scope | Agregat seluruh RT di bawah RW | Data RT saja | Data RT saja | ❌ MISMATCH |
| Read-only (RW) | Harus read-only | ✅ (no create/edit buttons) | ✅ (no create/edit buttons) | ✅ |

### Kesimpulan KAS
**NOT INTEGRATED**

Web dan Android memanggil `GET /api/v1/kas/transactions` yang RT-scoped, bukan `GET /api/v1/rw/{rwId}/kas/transactions` dari PR 4.

---

## 3. AUDIT LOKASI DANA

### Backend — PR 4 Endpoint

**File:** `backend/cmd/api/main.go` baris 756
```go
r.Get("/api/v1/rw/{rwId}/locations", rwHandler.GetLocations)
```
✅ Terdaftar, menggunakan `rwHandler.GetLocations`

### Web — Current Implementation

**File:** `frontend/src/features/fund_location/FundLocationList.tsx`

```typescript
const loadLocations = useCallback(async () => {
  if (!token) return
  setLocations(await apiListFundLocations(token, {
    ...(filterCategory ? { category: filterCategory } : {}),
    ...(filterActive !== undefined ? { is_active: filterActive } : {}),
  }))
}, [token, filterCategory, filterActive])
```

**API Client:** `frontend/src/app/api.ts` baris 2434

```typescript
export async function apiListFundLocations(
  token: string,
  params?: { category?: string; is_active?: boolean },
): Promise<FundLocation[]> {
  const res = await authenticatedApiRequest(BASE + '/fund-locations' + (q.toString() ? '?' + q.toString() : ''), {
    // ...
  })
}
```

**Endpoint yang dipanggil:** `GET /api/v1/fund-locations` (bukan `/api/v1/rw/{rwId}/locations`)

### Android — Current Implementation

**File:** `mobile/lib/features/fund_location/presentation/screens/fund_location_screen.dart`

```dart
final fundLocations = await api.listLocations(
  category: _filterCategory,
  isActive: _filterActive,
);
```

**API Client:** `mobile/lib/features/fund_location/data/fund_location_api_client.dart`

```dart
Future<List<FundLocation>> listLocations({
  String? category,
  bool? isActive,
}) async {
  final response = await dio.get(
    '/api/v1/fund-locations',
    queryParameters: queryParams,
  );
  // ...
}
```

**Endpoint yang dipanggil:** `GET /api/v1/fund-locations` (bukan `/api/v1/rw/{rwId}/locations`)

### Perbandingan Lokasi Dana

| Aspek | Backend PR 4 (Expected) | Web (Actual) | Android (Actual) | Status |
|-------|------------------------|--------------|------------------|--------|
| Endpoint | `/api/v1/rw/{rwId}/locations` | `/api/v1/fund-locations` | `/api/v1/fund-locations` | ❌ MISMATCH |
| Parameter rw_id | `{rwId}` path param | TIDAK ADA | TIDAK ADA | ❌ MISSING |
| Filter category | Query `category=bank|brankas|cash` | ✅ | ✅ | ✅ |
| Filter is_active | Query `is_active=true|false` | ✅ | ✅ | ✅ |
| Data scope | Agregat lokasi dari seluruh RT | Data RT saja | Data RT saja | ❌ MISMATCH |
| Read-only (RW) | Harus read-only | Ada tombol Create/Edit/Delete/Transfer | Ada tombol Create/Transfer/Opening Balance | ❌ PERLU REVISI |

### Kesimpulan Lokasi Dana
**NOT INTEGRATED**

Web dan Android memanggil `GET /api/v1/fund-locations` yang RT-scoped, bukan `GET /api/v1/rw/{rwId}/locations` dari PR 4.

**Catatan:** Lokasi Dana di Web dan Android saat ini menampilkan semua aksi mutasi (Create, Edit, Delete, Transfer, Opening Balance, Reverse Transfer). Jika RW hanya boleh read-only, perlu menambahkan permission check.

---

## 4. AUDIT PERMISSION & KONTEKS RW

### Web — Role & Authorization

**File:** `frontend/src/app/App.tsx`

Routing berdasarkan role:
```tsx
{isSuperAdmin && <Route path="rw" element={<RwList />} />}
<Route path="*" element={<AllowedRTRoute />}>
  <Route path="kas" element={<KasRoutes />} />
  <Route path="lokasi-dana" element={<FundLocationRoutes />} />
</Route>
```

**Observasi:**
- `/rw` route hanya untuk Super Admin
- `/kas`, `/lokasi-dana`, `/dashboard` untuk `AllowedRTRoute` (bukan warga)
- Tidak ada perbedaan route atau endpoint antara konteks RT vs RW
- User yang memiliki jabatan di RW (bukan Ketua RT) tetap menggunakan endpoint RT-scoped

### Android — Role & Authorization

**File:** `mobile/lib/app/navigation_shell.dart` + `mobile/lib/features/rw/presentation/screens/rw_list_screen.dart`

```dart
if (isAllowedRT(user)) {
  [Beranda, Kas, LokasiDana?, Iuran, Warga, Notifikasi?, Profil]
} else {
  [Beranda, Notifikasi?, Profil]
}
```

**Observasi:**
- Bottom nav tidak membedakan konteks RW vs RT
- RW management hanya tersedia di `rw_list_screen.dart` tanpa guard role
- Semua fitur operasional menggunakan route yang sama untuk RT dan RW

### Perbedaan Konteks RW vs RT

| Sumber | Saat Ini | Seharusnya (PR 4) |
|--------|----------|-------------------|
| JWT claim | `rt_id` (bukan `rw_id`) | `rw_id` harus tersedia |
| Scope data | `rt_id` dari JWT | `rw_id` dari user context |
| Route | `/kas`, `/lokasi-dana`, `/dashboard` | `/rw/{rwId}/kas`, `/rw/{rwId}/locations`, `/rw/{rwId}/dashboard` |
| Authorization | `AllowedRTRoute` (role-based) | Harus cek `rw_id` user vs path param |

### File Auth yang Relevan

**Web — `frontend/src/app/api.ts`** baris 521-530:
```typescript
interface ApiUpdateRTBody {
  name?: string
  rw_id?: string | null  // ← rw_id ada di API models
}
```
`rw_id` sudah ada di models, tapi TIDAK digunakan untuk routing API RW.

**Android — `mobile/lib/core/api/models.dart`** baris 11:
```dart
final String? rwId;
```
`rwId` sudah ada di models, tapi TIDAK digunakan untuk memanggil API RW PR 4.

---

## 5. Matriks Hasil Integrasi

| Menu | Platform | API saat ini | API yang seharusnya | Endpoint yang dipanggil | Status Integrasi |
|------|----------|--------------|---------------------|------------------------|------------------|
| Beranda | Web | `apiGetDashboardSummary(token, period)` | `apiGetRWDashboard(token, rwId, period)` | `GET /api/v1/dashboard/summary` | **NOT INTEGRATED** |
| Beranda | Android | `kasApiClient.getDashboardSummary(period)` | `rwApiClient.getDashboard(rwId, period)` | `GET /api/v1/dashboard/summary` | **NOT INTEGRATED** |
| KAS | Web | `apiListTransactions(token, params)` | `apiListRWTransactions(token, rwId, params)` | `GET /api/v1/kas/transactions` | **NOT INTEGRATED** |
| KAS | Android | `kasRepository.getTransactions(params)` | `rwKasRepository.getTransactions(rwId, params)` | `GET /api/v1/kas/transactions` | **NOT INTEGRATED** |
| Lokasi Dana | Web | `apiListFundLocations(token, params)` | `apiListRWLocations(token, rwId, params)` | `GET /api/v1/fund-locations` | **NOT INTEGRATED** |
| Lokasi Dana | Android | `fundLocationsProvider.listLocations()` | `rwFundLocationsProvider.listLocations(rwId)` | `GET /api/v1/fund-locations` | **NOT INTEGRATED** |

---

## 6. Rekomendasi Implementasi Minimal

### WEB — Perubahan Minimal

#### 1. Buat API Client RW Baru

**File baru:** `frontend/src/app/rw-api.ts`

```typescript
// GET /api/v1/rw/{rwId}/dashboard
export async function apiGetRWDashboard(token: string, rwId: string, period?: string): Promise<RWDashboardSummary> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/dashboard${period ? `?period=${encodeURIComponent(period)}` : ''}`
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json(res)
}

// GET /api/v1/rw/{rwId}/kas
export async function apiGetRWKAS(token: string, rwId: string, period?: string): Promise<RWKASSummary> {
  const url = `${BASE}/rw/${encodeURIComponent(rwId)}/kas${period ? `?period=${encodeURIComponent(period)}` : ''}`
  const res = await authenticatedApiRequest(url, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json(res)
}

// GET /api/v1/rw/{rwId}/kas/transactions
export async function apiListRWTransactions(token: string, rwId: string, params: { period?: string; arus?: string; metode?: string; search?: string; page?: number; page_size?: number }): Promise<RWTransactionPaginated> {
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

// GET /api/v1/rw/{rwId}/locations
export async function apiListRWLocations(token: string, rwId: string, params?: { category?: string; is_active?: boolean }): Promise<RWLocation[]> {
  const q = new URLSearchParams()
  if (params?.category) q.set('category', params.category)
  if (params?.is_active != null) q.set('is_active', String(params.is_active))
  const res = await authenticatedApiRequest(`${BASE}/rw/${encodeURIComponent(rwId)}/locations?${q.toString()}`, {
    method: 'GET',
    headers: { Authorization: `Bearer ${token}` },
  })
  return await json(res)
}
```

#### 2. Buat Context RW

**File baru:** `frontend/src/app/RWContext.tsx`

```typescript
// Menyimpan rw_id saat ini berdasarkan user's RW memberships
// Memanggil apiMe() untuk mendapatkan rw_id dari response
export function useRWContext(): { rwId: string | null; isLoading: boolean }
```

#### 3. Buat Komponen Dashboard RW

**File baru:** `frontend/src/features/rw/RwDashboard.tsx`

```tsx
// REUSE dengan adapter data dari RwDashboard.tsx
// Menggunakan komponen dari Dashboard.tsx (FinancialChart, metric cards, dll)
// Hanya ganti data source: apiGetRWDashboard() vs apiGetDashboardSummary()
```

**Struktur minimal:**
```
frontend/src/features/rw/
├── RwDashboard.tsx      ← REUSE dari Dashboard.tsx, ganti API call
├── RwKasList.tsx        ← REUSE dari KasList.tsx, ganti API call
├── RwFundLocationList.tsx ← REUSE dari FundLocationList.tsx, ganti API call + disable mutasi
└── rw-api.ts             ← API client baru
```

#### 4. Tambah Route RW

**File:** `frontend/src/app/App.tsx`

```tsx
// Tambahkan setelah route /rt
<Route path=":rwId" element={<RwDashboard />} />
<Route path=":rwId/kas" element={<RwKasList />} />
<Route path=":rwId/lokasi-dana" element={<RwFundLocationList />} />
```

### ANDROID — Perubahan Minimal

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
  Future<List<FundLocation>> getLocations(String rwId, {
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

**File baru:** `mobile/lib/features/rw/presentation/screens/rw_dashboard_screen.dart`

```dart
// REUSE dari dashboard_screen.dart
// Hanya ganti data source: RwApiClient.getDashboard() vs kasApiClient.getDashboardSummary()
```

**Struktur minimal:**
```
mobile/lib/features/rw/
├── data/
│   ├── rw_api_client.dart        ← API client baru
│   ├── rw_models.dart            ← Model response (jika berbeda dari RT)
│   └── rw_providers.dart         ← Riverpod providers baru
└── presentation/
    └── screens/
        ├── rw_dashboard_screen.dart    ← REUSE dari dashboard_screen.dart
        ├── rw_kas_screen.dart          ← REUSE dari kas_screen.dart
        └── rw_fund_location_screen.dart ← REUSE dari fund_location_screen.dart + disable mutasi
```

#### 3. Tambah Navigasi RW

**File:** `mobile/lib/app/navigation_shell.dart`

```dart
// Tambahkan route RW
case '/home/rw/:rwId':
  return RWDashboardScreen(rwId: routingData.pathParameters['rwId']!);
case '/home/rw/:rwId/kas':
  return RWKasScreen(rwId: routingData.pathParameters['rwId']!);
case '/home/rw/:rwId/lokasi-dana':
  return RWFundLocationScreen(rwId: routingData.pathParameters['rwId']!);
```

---

## 7. Reuse Strategy

### Web — Komponen yang Bisa DI-REUSE

| Komponen RT | Lokasi File | RW Adaptasi | Status |
|-------------|-------------|-------------|--------|
| DashboardSummary cards | `Dashboard.tsx` baris 340-360 | REUSE — ganti data source | ✅ REUSE dengan adapter |
| FinancialChart | `Dashboard.tsx` baris 364-374 | REUSE — struktur data sama | ✅ REUSE langsung |
| Donut Payment Method | `Dashboard.tsx` baris 412-491 | REUSE — struktur data sama | ✅ REUSE langsung |
| Iuran Progress | `Dashboard.tsx` baris 382-408 | REUSE — struktur data sama | ✅ REUSE langsung |
| Recent Transactions table | `Dashboard.tsx` baris 495-565 | REUSE — struktur data sama | ✅ REUSE langsung |
| KasList | `KasList.tsx` | REUSE — ganti API call | ✅ REUSE dengan adapter |
| KasIncome/Expense forms | `KasIncome.tsx`, `KasExpense.tsx` | **DISABLE** untuk RW (read-only) | ✅ REUSE dengan hide forms |
| FundLocationList | `FundLocationList.tsx` | REUSE — ganti API call + disable mutasi | ✅ REUSE dengan adapter + permission |

### Android — Widget yang Bisa DI-REUSE

| Widget RT | Lokasi File | RW Adaptasi | Status |
|-----------|-------------|-------------|--------|
| SummaryCard | `dashboard_screen.dart` | REUSE — ganti data source | ✅ REUSE dengan adapter |
| FinancialSummaryChart | `dashboard_screen.dart` | REUSE — struktur data sama | ✅ REUSE langsung |
| Donut payment | `dashboard_screen.dart` | REUSE — struktur data sama | ✅ REUSE langsung |
| KasTransactionCard | `kas_screen.dart` | REUSE — struktur data sama | ✅ REUSE langsung |
| KasScreen | `kas_screen.dart` | REUSE — ganti API call | ✅ REUSE dengan adapter |
| Form kas_masuk/keluar | `kas_masuk_screen.dart` | **DISABLE** untuk RW (read-only) | ✅ REUSE dengan hide forms |
| FundLocationScreen | `fund_location_screen.dart` | REUSE — ganti API call + disable mutasi | ✅ REUSE dengan adapter + permission |

---

## 8. Permission & Read-Only Requirements

### Lokasi Dana RW — Read-Only Check

**Web — `FundLocationList.tsx`** saat ini menampilkan:
- Button "Tambah Lokasi" (line ~750) — **PERLU DISABLE untuk RW**
- Button "Transfer Dana" (line ~760) — **PERLU DISABLE untuk RW**
- Button "Alokasi Saldo Awal" (line ~770) — **PERLU DISABLE untuk RW**
- Edit/Delete buttons di table rows — **PERLU DISABLE untuk RW**

**Android — `fund_location_screen.dart`** saat ini menampilkan:
- FAB "Tambah Lokasi" (line ~168) — **PERLU DISABLE untuk RW**
- Button "Transfer" (line ~390) — **PERLU DISABLE untuk RW**
- Button "Alokasi" di reconciliation card (line ~370) — **PERLU DISABLE untuk RW**

### Rekomendasi Permission Check

**Web:**
```tsx
const isRWContext = rwId !== null  // dari context
const canMutate = !isRWContext && isPengurusOrBendahara

// Di semua tempat yang menampilkan action buttons:
{canMutate && <Button>...</Button>}
```

**Android:**
```dart
final bool isRWContext = widget.rwId != null;
final bool canMutate = !isRWContext && _canManage();

// Di semua tempat yang menampilkan action buttons:
if (canMutate) ElevatedButton(...)
```

---

## 9. Kesimpulan Akhir

### Status Per Menu

| Menu | Status | Deskripsi |
|------|--------|-----------|
| **Beranda** | **NOT INTEGRATED** | Masih menggunakan `GET /api/v1/dashboard/summary` (RT-scoped) |
| **KAS** | **NOT INTEGRATED** | Masih menggunakan `GET /api/v1/kas/transactions` (RT-scoped) |
| **Lokasi Dana** | **NOT INTEGRATED** | Masih menggunakan `GET /api/v1/fund-locations` (RT-scoped) |

### Kesimpulan Utama

1. **Backend PR 4 sudah siap** — 4 endpoint terdaftar dan berfungsi.
2. **UI Web dan Android BELUM terintegrasi** — masih memanggil API RT yang lama.
3. **Pengguna RW melihat data RT, bukan agregat RW** — ini adalah gap kritis.
4. **Komponen UI bisa di-REUSE** — perbedaan hanya pada data source (API call), bukan pada tampilan atau layout.
5. **Perlu permission check** — Lokasi Dana RW harus read-only (disable semua tombol mutasi).
6. **Tidak ada duplikasi yang tidak perlu** — cukup buat layer adapter (API client baru) yang memanggil endpoint PR 4, lalu gunakan kembali komponen UI RT yang sudah ada.

### Rekomendasi Prioritas

1. **HIGH:** Buat API client RW baru untuk Web dan Android.
2. **HIGH:** Buat context/provider untuk menyimpan `rw_id` saat ini.
3. **MEDIUM:** Buat komponen RW Dashboard yang mengadaptasi API client baru.
4. **MEDIUM:** Buat komponen RW Kas yang mengadaptasi API client baru + disable form.
5. **MEDIUM:** Buat komponen RW Lokasi Dana yang mengadaptasi API client baru + disable semua aksi mutasi.
6. **LOW:** Tambah navigasi RW di sidebar Web dan bottom nav Android.
