# Web Frontend Warga Module — UI Pattern Inspection Report

> **Purpose:** Audit all Warga UI patterns so the Iuran/KAS modules can visually match Warga exactly. No new design patterns should be created for Iuran/KAS.
>
> **Date:** 2026-10-02
>
> **Working directory:** `frontend/src`
>
> **Technology:** React + TypeScript + Vite (no UI component library — custom components throughout)

---

## Files Read

| # | File | Purpose |
|---|------|---------|
| 1 | `features/warga/ResidentList.tsx` | Main list page |
| 2 | `features/warga/HouseholdCreate.tsx` | Create form (modal) |
| 3 | `features/warga/HouseholdEdit.tsx` | Edit form (modal) |
| 4 | `features/warga/ResidentDetail.tsx` | Detail page |
| 5 | `features/warga/ResidentDetailFields.tsx` | Detail field components |
| 6 | `features/warga/SpecialResidentCreate.tsx` | Special resident create (modal) |
| 7 | `features/warga/SpecialResidentEdit.tsx` | Special resident edit (modal) |
| 8 | `features/warga/WargaLayout.tsx` | Layout wrapper for warga routes |
| 9 | `features/warga/DataChoiceModal.tsx` | Modal with selectable cards |
| 10 | `features/warga/HouseholdMove.tsx` | Move household modal |
| 11 | `features/warga/WargaImportModal.tsx` | Import residents modal |
| 12 | `features/warga/HouseholdDetail.tsx` | Household detail page |
| 13 | `components/SearchBox.tsx` | Search input component |
| 14 | `components/FilterDropdown.tsx` | Filter dropdown component |
| 15 | `components/Pagination.tsx` | Paginated table controls |
| 16 | `components/RowActionMenu.tsx` | Per-row action menu |
| 17 | `components/Modal.tsx` | Generic modal dialog |
| 18 | `components/AppCard.tsx` | Card container component |
| 19 | `components/SectionHeader.tsx` | Section title component |
| 20 | `components/SummaryCard.tsx` | Summary metric card |
| 21 | `components/Badge.tsx` | Status badge |
| 22 | `components/Sidebar.tsx` | App sidebar navigation |
| 23 | `components/PageHeader.tsx` | Page header component |
| 24 | `components/SearchableSelect.tsx` | Searchable dropdown select |
| 25 | `app/App.tsx` | Routing configuration |
| 26 | `app/AuthContext.tsx` | Auth context provider |
| 27 | `app/wargaAuth.ts` | Warga-specific authorization |
| 28 | `app/ThemeProvider.tsx` | Dark/light theme provider |
| 29 | `styles/tokens.css` | CSS design tokens (colors, spacing, typography) |
| 30 | `styles/data-choice.css` | DataChoiceModal card styles |
| 31 | `styles/global.css` | Global styles |

---

## 1. ResidentList.tsx — Main List Page Pattern

### 1.1 Page Structure

```
PageHeader          ← title + subtitle text
  └─ SearchBox      ← search input (debounced)
Filters              ← horizontal row: status, nikFilter, role, search, reset button
  └─ AppCard > Table ← data table with sticky header
Table footer         ← total count + page size selector + pagination
FAB                  ← floating action button (Add Warga)
Modal<open>          ← DataChoiceModal (if user is pengurus)
```

**Key code snippet — PageHeader usage:**
```tsx
<PageHeader title="Daftar Warga" subtitle="Kelola data warga di RT Anda" />
```

The `PageHeader` component renders a `<section>` with `display: flex`, `flex-direction: column`, `gap: var(--sf-sp-sm)`. Title uses CSS class `page-title` (font-size 1.25rem, font-weight 600). Subtitle uses `page-subtitle` (font-size 0.875rem, color var(--sf-text-muted)).

**Key code snippet — Filter row:**
```tsx
<div style={{ display: "flex", gap: "var(--sf-sp-md)", alignItems: "center", flexWrap: "wrap" }}>
  <FilterDropdown ... />   {/* Status filter */}
  <input ... />            {/* NIK filter (text input) */}
  <FilterDropdown ... />   {/* Role filter */}
  <Button variant="primary">Cari</Button>
  <Button variant="secondary">Reset</Button>
</div>
```

Filters are rendered as an inline flex row with `gap: var(--sf-sp-md)` (12px), wrapping on narrow screens.

### 1.2 Table Columns & Structure

**Columns (in order):**
| # | Column | Component | Key | Notes |
|---|--------|-----------|-----|-------|
| 1 | No. | `<td>` with `rowIndex + 1` | `row-idx-{idx}` | Auto-numbered |
| 2 | NIK | `<td>` | `row-nik-{id}` | Text display |
| 3 | Nama | `<td>` with `<a>` link | `row-name-{id}` | Clickable → navigates to detail |
| 4 | Tempat, Tanggal Lahir | `<td>` | `row-tgl_lahir-{id}` | Joined `tempat_tanggal_lahir` field |
| 5 | Jenis Kelamin | `<td>` with `<Badge>` | `row-jk-{id}` | Green "L" / Red "P" badge |
| 6 | Agama | `<td>` | `row-agama-{id}` | Text |
| 7 | Status Perkawinan | `<td>` | `row-status_perkawinan-{id}` | Text |
| 8 | Alamat | `<td>` | `row-alamat-{id}` | Text (truncated via CSS `max-width: 240px; overflow: hidden; text-overflow: ellipsis;`) |
| 9 | RT | `<td>` | `row-rt-{id}` | Text |
| 10 | Status | `<td>` with `<Badge>` | `row-status-{id}` | Active=green, NonAktif=red |
| 11 | | `<RowActionMenu>` | `row-actions-{id}` | Dropdown with Edit/Hapus options |

**Table styling (from `tokens.css`):**
```css
--sf-table-row-min-height: 40px;
--sf-table-header-font-size: 0.625rem;    /* 10px */
--sf-table-header-font-weight: 600;
--sf-table-header-letter-spacing: 0.0625em;
--sf-table-header-color: var(--sf-text-muted);
--sf-table-cell-font-size: 0.75rem;        /* 12px */
--sf-table-cell-font-weight: 400;
--sf-table-row-hover: color-mix(in srgb, var(--sf-accent) 8%, var(--sf-bg));
```

The table uses `<table>` with `className="data-table"` and has:
- Sticky header (`position: sticky; top: 0`)
- Header row with all columns styled with header CSS variables
- Body rows with hover state using accent color mix
- Cells use `text-align: left` with `padding: var(--sf-sp-sm) var(--sf-sp-md)` (8px 12px)
- Row action column has `white-space: nowrap`
- First cell (row number) has `color: var(--sf-text-muted); font-weight: 500; text-align: center;`

### 1.3 Search Box

**Component:** `SearchBox` from `components/SearchBox.tsx`

**Key code snippet — SearchBox instantiation:**
```tsx
<SearchBox
  value={search}
  onChange={(e) => setSearch(e.target.value)}
  onSearch={() => setPage(1)}
  placeholder="Cari NIK atau nama warga"
  debounceMs={300}
  clearable
  icon={<MagnifyingGlass size={16} />}
  fullWidth
/>
```

**SearchBox component behavior:**
- Uses `<input>` with `className="search-input"` (styled in global.css)
- Debounced onChange (default 300ms) — fires search callback after debounce
- Has a clear button (×) when `clearable` prop is true
- Has a search button (magnifying glass icon) — triggers the `onSearch` callback
- `fullWidth` prop makes it take full container width
- Has `onKeyDown` support for Enter key triggering search
- Input styling: border, border-radius: var(--sf-radius-sm), padding: var(--sf-sp-sm), focus ring: var(--sf-focus-ring)

### 1.4 Filters — FilterDropdown Component

**Component:** `FilterDropdown` from `components/FilterDropdown.tsx`

**Key code snippet — FilterDropdown instantiation:**
```tsx
<FilterDropdown
  label="Status"
  value={statusFilter}
  onChange={handleStatusFilterChange}
  options={[
    { value: "", label: "Semua Status" },
    { value: "active", label: "Aktif" },
    { value: "inactive", label: "Non-Aktif" },
  ]}
  style={{ minWidth: "150px" }}
/>
```

**FilterDropdown component:**
- Renders a `<select>` element styled with CSS classes
- Has a label above the select
- Supports custom width via `style` prop
- Standard dropdown with `onChange` callback
- Option values are lowercase string identifiers

### 1.5 Pagination

**Component:** `Pagination` from `components/Pagination.tsx`

```tsx
<Pagination
  page={pagination.page}
  totalPages={pagination.total_pages}
  perPage={pagination.per_page}
  totalItems={pagination.total}
  onPrevious={handlePrevPage}
  onNext={handleNextPage}
  onPageSizeChange={handlePageSizeChange}
  pageSizeOptions={[10, 20, 50]}
/>
```

**Pagination behavior:**
- Displays current page info: "Menampilkan 1-10 dari 50"
- Has previous/next buttons
- Page size selector dropdown with options [10, 20, 50]
- Previous/next buttons disabled at boundaries (page 1 or last page)
- Hook `usePersistedPageSize` stores selected page size in localStorage under key `{routeKey}_pageSize`

### 1.6 Empty / Loading / Error States

**Loading state:**
```tsx
if (loading) return <div className="loading-container">Loading...</div>;
```
Uses class `loading-container` styled in global.css (centered with spinner).

**Empty state:**
```tsx
if (data.length === 0) return (
  <div className="empty-state-container">
    <p>Tidak ada warga yang ditemukan.</p>
  </div>
);
```
Uses class `empty-state-container` styled in global.css (centered, muted text).

**Error state:**
```tsx
{error && (
  <div className="error-message">
    <p>Error: {error}</p>
    <Button variant="secondary" onClick={() => setError(null)}>Dismiss</Button>
  </div>
)}
```
Uses class `error-message` styled in global.css (red background/border, error text color).

### 1.7 Floating Action Button (FAB)

**Does exist.** Renders a fixed-position button:
```tsx
<Fab
  onClick={() => setShowDataChoiceModal(true)}
  icon={<Plus size={20} />}
  tooltip="Tambah Warga"
/>
```

Uses a `Fab` component with:
- Fixed position: `bottom: 24px; right: 24px`
- Circular shape with accent color background
- Plus icon
- Tooltip text on hover
- Styled as a floating action button with shadow

### 1.8 Action Menu — RowActionMenu

**Component:** `RowActionMenu` from `components/RowActionMenu.tsx`

**Key code snippet:**
```tsx
<RowActionMenu
  items={[
    { label: "Lihat Detail", icon: <Eye size={14} />, onClick: () => ... },
    { label: "Edit", icon: <Pencil size={14} />, onClick: () => ... },
    { label: "Nonaktifkan", icon: <Ban size={14} />, onClick: () => handleNonaktif(id) },
    { label: "Hapus", icon: <Trash2 size={14} />, onClick: () => handleDelete(id), danger: true },
  ]}
/>
```

**RowActionMenu behavior:**
- Renders a button with three dots (kebab menu icon) as the trigger
- On click/hover, opens a dropdown menu overlay
- Each menu item has an icon (small, 14px), label, and optional `danger` style (red text/background for destructive actions)
- Menu overlays near the trigger button
- Clicking outside closes the menu

### 1.9 Detail Navigation

**Pattern:** Clicking the resident name (a `<a>` link within the table cell) navigates using `react-router-dom` Link component:
```tsx
<Link to={`/home/warga/${id}`}>{resident.nama}</Link>
```

The route is defined as `/home/warga/:id` in App.tsx. Uses react-router's `Link` component (client-side navigation, no page reload).

---

## 2. HouseholdCreate.tsx — Create Form Pattern

### 2.1 Form Layout — Modal Wrapping

**Pattern:** The form is wrapped inside a `Modal` component with a `DialogContent`.

```tsx
<Modal open={open} onClose={handleClose} size="md">
  <DialogContent>
    <ModalHeader title="Tambah Kepala Keluarga" onClose={handleClose} />
    <ModalBody>
      <form onSubmit={handleSubmit}>
        {/* Form fields */}
      </form>
    </ModalBody>
    <ModalFooter>
      <Button variant="secondary" onClick={handleClose}>Batal</Button>
      <Button type="submit" disabled={submitting}>Simpan</Button>
    </ModalFooter>
  </DialogContent>
</Modal>
```

### 2.2 Form Layout — Inline Fields

**Pattern:** Fields use a two-column grid layout where pairs of related fields share a row:

```tsx
<div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--sf-sp-lg)" }}>
  <FormGroup label="Nama Lengkap" required>
    <input ... />
  </FormGroup>
  <FormGroup label="NIK" required>
    <input ... />
  </FormGroup>
  <FormGroup label="Tempat Lahir">
    <input ... />
  </FormGroup>
  <FormGroup label="Tanggal Lahir">
    <input type="date" ... />
  </FormGroup>
  {/* ... more fields */}
</div>
```

**FormGroup component:** Wraps a label + input pair. Label has `required` asterisk (*). Spacing between fields: `var(--sf-sp-lg)` (16px) both horizontally and vertically.

**Full-width fields** (like Alamat, RT) span the full grid width.

### 2.3 Input Fields — Component & Validation

**Component:** Standard HTML `<input>` elements wrapped in `FormGroup`.

**Validation:** React Hook Form (`useForm`) with `zod` resolver:
```tsx
const { control, handleSubmit, formState: { errors } } = useForm({
  resolver: zodResolver(householdCreateSchema),
  defaultValues: { namaLengkap: "", nik: "", ... }
});

<Controller control={control} name="nik" render={({ field, fieldState }) => (
  <FormGroup label="NIK" required error={fieldState.error?.message}>
    <input {...field} />
  </FormGroup>
)} />
```

**Inline error display:** Error messages render below the input with `color: var(--sf-danger)` (red).

### 2.4 Submit / Cancel Buttons

**Positioning:** In `ModalFooter`, right-aligned with gap:

```tsx
<ModalFooter>
  <Button variant="secondary" onClick={handleClose}>Batal</Button>
  <Button type="submit" disabled={submitting}>
    {submitting ? "Menyimpan..." : "Simpan"}
  </Button>
</ModalFooter>
```

- **Batal** (Cancel): `variant="secondary"`, styled with border, transparent background
- **Simpan** (Save): default `variant="primary"`, accent color background, disabled during submit with loading text "Menyimpan..."

### 2.5 Loading State During Submit

```tsx
const [submitting, setSubmitting] = useState(false);
// ...
<Button type="submit" disabled={submitting}>
  {submitting ? "Menyimpan..." : "Simpan"}
</Button>
```

Button text changes to "Menyimpan..." and is disabled during submission.

### 2.6 Error Display

**Pattern:** Toast notification on API error, inline errors on validation:
```tsx
const { toast } = useToast();
// ...
if (error.response) {
  toast.error("Gagal menambahkan kepala keluarga");
}
```

Uses a `useToast()` hook for global toast notifications. Validation errors appear inline below each field.

---

## 3. HouseholdEdit.tsx — Edit Form Pattern

**Identical to HouseholdCreate.** Same modal wrapping, same form layout (2-column grid), same validation (zod + react-hook-form), same button pattern.

**Differences from Create:**
- Pre-populated with existing data via `defaultValues` from API
- Submit calls `PUT /households/:id` instead of `POST /households`
- Has a `useQuery` to fetch existing household data on mount
- Title changes from "Tambah Kepala Keluarga" to "Edit Kepala Keluarga"
- No differences in UI pattern — purely data-source and HTTP method difference

---

## 4. ResidentDetail.tsx — Detail Page Pattern

### 4.1 Detail Layout

```tsx
<PageHeader title="Detail Warga" subtitle={resident.nama} backLink="/home/warga">
  <Button variant="primary" onClick={() => setEditModalOpen(true)}>
    <Pencil size={16} /> Edit
  </Button>
</PageHeader>

<div style={{ display: "flex", flexDirection: "column", gap: "var(--sf-sp-lg)" }}>
  {/* Summary cards */}
  <SummaryCard title="Status" value="Aktif" icon={<User size={20} />} variant="success" />
  <SummaryCard title="NIK" value={resident.nik} icon={<IdCard size={20} />} variant="info" />

  {/* Detail sections */}
  <AppCard>
    <SectionHeader title="Informasi Pribadi" />
    {/* Field rows: Nama, NIK, Tempat/Tgl Lahir, etc. */}
  </AppCard>

  <AppCard>
    <SectionHeader title="Informasi Keluarga" />
    {/* Field rows */}
  </AppCard>
</div>
```

**Field display pattern:** Each detail field uses a label-value pair in a flex row:
```tsx
<div className="detail-row">
  <span className="detail-label">NIK</span>
  <span className="detail-value">{resident.nik}</span>
</div>
```

`detail-row` has `display: flex; justify-content: space-between; padding: var(--sf-sp-sm) 0; border-bottom: 1px solid var(--sf-border-subtle)`.
`detail-label` has `color: var(--sf-text-muted); font-size: 0.875rem;`.
`detail-value` has `font-weight: 500; font-size: 0.875rem;`.

### 4.2 Back Navigation

The `PageHeader` component accepts a `backLink` prop which renders a back arrow button in the header area. Navigation uses react-router `<Link>` to the back URL.

### 4.3 Action Buttons

**Position:** In the PageHeader `title` slot (right-aligned within the header):
```tsx
<PageHeader title="Detail Warga" subtitle={resident.nama} backLink="/home/warga">
  <Button variant="primary" onClick={handleEdit}>
    <Pencil size={16} /> Edit
  </Button>
</PageHeader>
```

**Button variants:**
- **Edit**: `variant="primary"` (accent color background)
- Additional action buttons would follow same pattern

### 4.4 Loading / Error / Not-Found States

**Loading:**
```tsx
if (loading) return <div className="loading-container">Loading...</div>;
```

**Not found:**
```tsx
if (notFound) return <div className="not-found-container">...Warga tidak ditemukan...</div>;
```

**Error:**
```tsx
{error && <div className="error-message">...Error: {error}...</div>}
```

Uses the same CSS classes as ResidentList (`loading-container`, `error-message`, `not-found-container`).

---

## 5. SpecialResidentCreate.tsx & SpecialResidentEdit.tsx

### 5.1 Create Form Pattern

**Modal wrapping:** Same pattern as HouseholdCreate — `Modal > DialogContent > ModalHeader > ModalBody > form > ModalFooter`.

**Form fields:**
- Modal title: "Tambah Warga Khusus"
- Fields: NIK (required), Nama Lengkap (required), Jenis Kelamin (select), Role (select: ["warga", "kepala_keluarga"]), Status Perkawinan (select), Alamat (textarea), No. Telepon (input), Agama (select)
- Same 2-column grid layout for paired fields
- Same validation with zod + react-hook-form
- Same button pattern in ModalFooter

### 5.2 Edit Form Pattern

**Identical to Create**, with pre-populated data and PUT endpoint. Title changes to "Edit Warga Khusus".

### 5.3 Differences from Regular (Non-Special) Forms

| Aspect | Regular (HouseholdCreate/Edit) | Special Resident Create/Edit |
|--------|-------------------------------|------------------------------|
| Modal title | "Tambah/Edit Kepala Keluarga" | "Tambah/Edit Warga Khusus" |
| Fields | NIK, Nama, Tempat/Tgl Lahir, etc. | NIK, Nama, Jenis Kelamin, Role, Status Perkawinan, Alamat, No. Telepon, Agama |
| Route key | household | special-resident |
| API endpoint | /households | /special-residents |
| Validation schema | householdCreateSchema | specialResidentSchema |

Both follow the **exact same visual pattern**: modal → dialog → 2-col grid form → footer buttons.

---

## 6. Shared Components

All components are custom-built (no external UI library like MUI, Ant Design, or Chakra).

### 6.1 `SearchBox.tsx` — Search Input
- Debounced `<input>` with clear button
- Props: `value`, `onChange`, `onSearch`, `placeholder`, `debounceMs`, `clearable`, `icon`, `fullWidth`
- Renders: label (optional), input with icon, clear × button, search button

### 6.2 `FilterDropdown.tsx` — Filter Dropdown
- Wraps a `<select>` with label
- Props: `label`, `value`, `onChange`, `options: {value, label}[]`, `style`
- Standard dropdown with custom styling

### 6.3 `Pagination.tsx` — Table Pagination Controls
- Shows page range info, prev/next buttons, page size selector
- Props: `page`, `totalPages`, `perPage`, `totalItems`, `onPrevious`, `onNext`, `onPageSizeChange`, `pageSizeOptions`
- Uses `usePersistedPageSize` hook to remember page size per route

### 6.4 `RowActionMenu.tsx` — Per-Row Action Dropdown
- Three-dots kebab menu button
- Props: `items: {label, icon, onClick, danger?}[]`
- Opens dropdown overlay on click
- `danger: true` items render in red

### 6.5 `Modal.tsx` — Generic Modal
- Wraps content in a backdrop overlay
- Props: `open`, `onClose`, `size` ("sm" | "md" | "lg"), children
- Uses DialogContent, ModalHeader, ModalBody, ModalFooter sub-components
- Closes on backdrop click and Escape key

### 6.6 `AppCard.tsx` — Card Container
- Props: `title?`, `action?`, `children`, `style`
- Styled div with border, border-radius, background, padding
- Optional title bar with action slot

### 6.7 `SectionHeader.tsx` — Section Title
- Props: `title`, `action?`
- Renders section title with accent-colored bottom border
- `font-weight: 600`, `font-size: 0.875rem`, `color: var(--sf-text)`

### 6.8 `SummaryCard.tsx` — Metric Card
- Props: `title`, `value`, `icon`, `variant` ("success" | "danger" | "info")
- Card showing a metric with colored icon and value
- Variant controls icon color and accent soft background

### 6.9 `Badge.tsx` — Status Badge
- Props: `variant` ("success" | "danger" | "warning" | "info"), children
- Small pill-shaped element with colored background (`--sf-{variant}-soft`)
- Used for status indicators (active/inactive, gender, etc.)

### 6.10 `DataChoiceModal.tsx` — Selection Card Modal
- Modal with selectable choice cards
- Cards have icon, title, description
- Visual style defined in `styles/data-choice.css` (separate CSS file)
- Cards support selected state (`dc-choice-card--selected`) and disabled state

### 6.11 `SearchableSelect.tsx` — Searchable Dropdown Select
- Combines search input with dropdown
- For selecting from large option lists with search capability
- Custom-styled (not a native `<select>`)

### 6.12 `useToast()` — Toast Notification Hook
- Global toast system for success/error/info messages
- Used for API error feedback and success confirmations

---

## 7. Routing — App.tsx

### All Warga Routes

```tsx
<Route path="/home" element={<AppLayout>...<Sidebar>...</Sidebar></AppLayout>}>
  <Route path="home" element={<Dashboard />} />

  {/* Warga routes — under /home prefix (ShellRoute) */}
  <Route element={<WargaLayout />}>
    <Route path="warga" element={<ResidentList />} />
    <Route path="warga/:id" element={<ResidentDetail />} />
    <Route path="household" element={<HouseholdList />} />
    <Route path="household/create" element={<HouseholdCreate />} />
    <Route path="household/:id" element={<HouseholdDetail />} />
    <Route path="household/:id/edit" element={<HouseholdEdit />} />
    <Route path="special-resident" element={<SpecialResidentList />} />
    <Route path="special-resident/create" element={<SpecialResidentCreate />} />
    <Route path="special-resident/:id" element={<SpecialResidentDetail />} />
    <Route path="special-resident/:id/edit" element={<SpecialResidentEdit />} />
  </Route>

  {/* Other feature routes... */}
</Route>
```

**Route structure notes:**
- All routes nested under `/home` prefix (inside `AppLayout` with sidebar)
- Warga routes use `WargaLayout` wrapper (provides warga-specific auth checks)
- Detail pages: `/warga/:id`, `/household/:id`, `/special-resident/:id`
- Create pages: `/household/create`, `/special-resident/create`
- Edit pages: `/household/:id/edit`, `/special-resident/:id/edit`
- Uses `react-router-dom` v6+ route structure

### WargaLayout — Auth Gate

```tsx
function WargaLayout() {
  const { user, isAuthenticated } = useAuth();
  // Check if user has warga role before rendering content
  if (!isAuthenticated || !hasRole(user, "warga")) {
    return <Navigate to="/home" />;
  }
  return <Outlet />;
}
```

---

## 8. Styling / Theme

### 8.1 CSS Architecture

**Global design tokens:** `styles/tokens.css`
- All colors, spacing, typography, shadows defined as CSS custom properties
- Two themes: `:root` (light) and `[data-theme='dark']` (dark)
- Theme switching via `data-theme` attribute on `<html>` element
- System preference fallback via `@media (prefers-color-scheme: dark)`

**Global styles:** `styles/global.css`
- Base resets, class-based component styles (`.loading-container`, `.error-message`, `.empty-state-container`, `.detail-row`, `.detail-label`, `.detail-value`, `.data-table`, `.search-input`)

**Module-specific CSS:** `styles/data-choice.css`
- Only the DataChoiceModal cards use a separate CSS file
- All other components use inline styles or CSS modules

### 8.2 CSS Class Naming Convention

**BEM-ish pattern with `sf-` prefix for tokens:**
- CSS tokens: `--sf-bg`, `--sf-text`, `--sf-accent`, `--sf-sp-md`, `--sf-radius-sm`
- Component classes: `page-title`, `page-subtitle`, `loading-container`, `error-message`, `empty-state-container`, `data-table`, `search-input`, `detail-row`, `detail-label`, `detail-value`
- DataChoiceModal classes use `dc-` prefix: `dc-choice-container`, `dc-choice-card`, `dc-choice-title`

### 8.3 Inline Styles vs CSS Classes

**Mixed approach:**
- **Inline styles:** Used extensively for layout (flex, grid, gap, position)
- **CSS classes:** Used for component-specific styling (table, search input, badges, detail rows)
- **CSS variables:** Used for all design tokens (colors, spacing, borders, shadows)

**Layout pattern:** Always use CSS variables for spacing (`gap: "var(--sf-sp-md)"`) rather than hard-coded pixel values.

### 8.4 Theme Colors Reference

| Token | Light | Dark | Usage |
|-------|-------|------|-------|
| `--sf-bg` | `#f6f6f7` | `#0d0d0d` | Page background |
| `--sf-surface` | `#ffffff` | `#1a1a1a` | Card/modal background |
| `--sf-text` | `#1a1a1a` | `#e6e6e6` | Primary text |
| `--sf-text-muted` | `#6b6b6b` | `#888888` | Muted text, labels |
| `--sf-border` | `#dedede` | `#2a2a2a` | Borders |
| `--sf-accent` | `#a970ff` | `#a970ff` | Buttons, links, accents |
| `--sf-success` | `#73bf69` | `#73bf69` | Success status |
| `--sf-danger` | `#f2495c` | `#f2495c` | Error/destructive |
| `--sf-info` | `#5794f2` | `#5794f2` | Info |

### 8.5 Accent Soft Tints (for badges, backgrounds)

| Token | Light | Dark |
|-------|-------|------|
| `--sf-accent-soft` | `rgba(169,112,255,0.10)` | `rgba(169,112,255,0.12)` |
| `--sf-success-soft` | `rgba(115,191,105,0.10)` | `rgba(115,191,105,0.15)` |
| `--sf-danger-soft` | `rgba(242,73,92,0.10)` | `rgba(242,73,92,0.15)` |

---

## 9. Auth / User Pattern

### 9.1 `useAuth()` — Auth Context

**Provider:** `AuthContext.tsx`

```tsx
const AuthContext = createContext<{
  user: AuthUser | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshTokens: () => Promise<void>;
}>(...);

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error("useAuth must be used within AuthProvider");
  return context;
}
```

**AuthUser shape:**
```typescript
interface AuthUser {
  id: string;
  email: string;
  name: string;
  role: string;          // "super_admin" | "pengurus" | "bendahara" | "warga"
  rt_id: string | null;  // null for super_admin
  // ... other fields
}
```

### 9.2 `wargaAuth.ts` — Warga Authorization Helper

```typescript
export function requiresWarga(): boolean {
  const { user } = useAuth();
  return user?.role === "warga" || user?.role === "pengurus" || user?.role === "bendahara";
}
```

Used in `WargaLayout` to gate route access based on user role.

### 9.3 How User Data is Accessed in Components

**Pattern 1 — Direct useAuth() call:**
```tsx
const { user, isAuthenticated } = useAuth();
if (!isAuthenticated) return <Navigate to="/login" />;
```

**Pattern 2 — Role checking in layout gates:**
```tsx
function WargaLayout() {
  const { user } = useAuth();
  const isWarga = user?.role === "warga";
  const isPengurus = user?.role === "pengurus";
  return <Outlet />;
}
```

**Pattern 3 — User data for personalization:**
```tsx
<PageHeader title={`Selamat Datang, ${user?.name}`} ... />
```

### 9.4 API Authentication

**`api.ts` — HTTP client:**
```typescript
const api = axios.create({ baseURL: "/api/v1" });

// Interceptor: attach JWT token from localStorage
api.interceptors.request.use((config) => {
  const token = localStorage.getItem("access_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});
```

All API calls go through this interceptor which injects the JWT from localStorage.

---

## Component Reuse Plan for Iuran/KAS

### ✅ REUSE — Direct visual copies

| Component / Pattern | Source | Iuran/KAS Application |
|---------------------|--------|----------------------|
| `PageHeader` | All list/detail pages | List pages, detail pages |
| `SearchBox` | ResidentList | Search in Iuran/KAS list pages |
| `FilterDropdown` | ResidentList | Filters in Iuran/KAS list pages |
| `Pagination` | ResidentList | Pagination in Iuran/KAS list pages |
| `RowActionMenu` | ResidentList | Per-row actions in Iuran/KAS tables |
| `AppCard` | Detail pages | Card containers for Iuran/KAS sections |
| `SectionHeader` | Detail pages | Section titles in Iuran/KAS details |
| `SummaryCard` | Detail pages | Metric cards for Iuran/KAS totals |
| `Badge` | ResidentList | Status badges in Iuran/KAS tables |
| `Modal > DialogContent > ModalHeader > ModalBody > ModalFooter` | HouseholdCreate/Edit | Create/Edit form modals for Iuran/KAS |
| `FormGroup` (2-col grid layout) | HouseholdCreate/Edit | Form fields in Iuran/KAS create/edit |
| `DataChoiceModal` | Household creation | Any "choose type" or multi-option selection |
| `Fab` (FAB) | ResidentList | Add button on Iuran/KAS list pages |
| `SearchableSelect` | SearchableSelect.tsx | Dropdown selects with search in Iuran/KAS forms |
| `useToast()` | All forms | Toast notifications for Iuran/KAS success/error |
| CSS tokens (`--sf-*`) | tokens.css | All colors, spacing, typography |
| `data-choice.css` classes | data-choice.css | DataChoiceModal styling |

### ✅ REUSE — Patterns (not components)

| Pattern | Description |
|---------|-------------|
| List page structure | PageHeader → SearchBox → Filters row → AppCard>Table → Pagination |
| Form in modal pattern | Modal wrapping a form in DialogContent with ModalHeader/Body/Footer |
| 2-column grid form layout | `gridTemplateColumns: "1fr 1fr"` with `gap: "var(--sf-sp-lg)"` |
| Detail page with SectionHeader | PageHeader → AppCard > SectionHeader → detail-row fields |
| Inline validation errors | Zod schema → react-hook-form Controller → error.message below field |
| Loading → Empty → Error states | `loading-container`, `empty-state-container`, `error-message` CSS classes |
| Back navigation via PageHeader `backLink` | Arrow button in header that navigates back |
| FAB positioning | Fixed bottom-right at `24px` |

### ⚠️ CONTEXT-SPECIFIC — Reuse structure, adapt content

| Source | Adaptation Needed |
|--------|-------------------|
| `ResidentList` table columns | Iuran/KAS will have different columns (e.g., periode, nominal, status_bayar) |
| `HouseholdCreate` form fields | Iuran/KAS forms will have different fields (periode, nominal, jenis, etc.) |
| `ResidentDetail` detail fields | Iuran/KAS details will show different information |
| `usePersistedPageSize` hook | Reusable as-is, but page size key will differ per route |

### ❌ DO NOT REUSE — Not applicable to Iuran/KAS

| Component | Why |
|-----------|-----|
| `DataChoiceModal` | Only used for "choose Household/Special Resident" — unlikely needed for Iuran/KAS |
| `WargaImportModal` | Import-specific, only relevant to warga module |
| `HouseholdMove` | Household-specific operation |
| `WargaLayout` | Warga-specific auth gate; Iuran/KAS will need their own layout wrapper |

### 🔑 Key Implementation Notes

1. **NO external UI libraries** — The project uses zero UI component libraries (no MUI, no AntD, no Chakra). All components are custom. Iuran/KAS must follow this pattern.

2. **CSS variables for ALL design tokens** — Never hardcode colors, spacing, or shadows. Use `var(--sf-*)` exclusively.

3. **Inline styles for layout** — Use inline `style={{ display: "flex", gap: "var(--sf-sp-md)" }}` for flex/grid layouts.

4. **CSS classes for component styling** — Use `className="data-table"` for table rows, `className="detail-row"` for detail fields, etc.

5. **Form validation** — Always use react-hook-form + zod resolver pattern. Never custom validation.

6. **Modals** — Always wrap forms in `Modal > DialogContent` with `ModalHeader/Body/Footer`. Never render forms as full-page when they're write operations.

7. **List pages** — Always follow the structure: PageHeader → SearchBox → Filters → AppCard>Table → Pagination.

8. **Theme support** — All components must work in both light and dark themes. Use CSS variables, never hardcode colors.
