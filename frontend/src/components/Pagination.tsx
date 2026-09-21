export interface PaginationProps {
  page: number
  pageSize: number
  total: number
  totalPages: number
  onPageChange: (page: number) => void
  onPageSizeChange?: (pageSize: number) => void
  pageSizeOptions?: number[]
}

export function Pagination({
  page,
  pageSize,
  total,
  totalPages,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [10, 25, 50],
}: PaginationProps) {
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1
  const end = Math.min(page * pageSize, total)

  // Generate page numbers with ellipsis
  const pages: (number | string)[] = []
  for (let i = 1; i <= totalPages; i++) {
    if (i === 1 || i === totalPages || Math.abs(i - page) <= 1) {
      if (pages.length > 0) {
        const last = pages[pages.length - 1]
        if (typeof last === 'number' && i - last > 1) {
          pages.push('...')
        }
      }
      pages.push(i)
    }
  }

  return (
    <div className="sf-pagination-footer" data-testid="pagination">
      <span className="sf-pagination-info">
        {total > 0 ? `${start}–${end} dari ${total}` : '0 dari 0'}
      </span>
      <div className="sf-pagination-right">
        <div className="sf-pagination-controls">
          <button
            type="button"
            className="sf-pagination-btn"
            onClick={() => onPageChange(page - 1)}
            disabled={page <= 1}
            aria-label="Prev"
            title="Sebelumnya"
          >
            ‹ Prev
          </button>
          {pages.map((p, idx) => {
            if (typeof p === 'string') {
              return (
                <span key={`ellipsis-${idx}`} className="sf-pagination-ellipsis">
                  &middot;&middot;&middot;
                </span>
              )
            }
            return (
              <button
                key={p}
                type="button"
                className={`sf-pagination-btn ${p === page ? 'sf-pagination-btn-active' : ''}`}
                onClick={() => onPageChange(p)}
                aria-current={p === page ? 'page' : undefined}
              >
                {p}
              </button>
            )
          })}
          <button
            type="button"
            className="sf-pagination-btn"
            onClick={() => onPageChange(page + 1)}
            disabled={page >= totalPages}
            aria-label="Next"
            title="Berikutnya"
          >
            Next ›
          </button>
        </div>
        {onPageSizeChange && (
          <select
            className="sf-page-size-select"
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
            aria-label="Jumlah per halaman"
          >
            {pageSizeOptions.map((opt) => (
              <option key={opt} value={opt}>
                {opt} / halaman
              </option>
            ))}
          </select>
        )}
      </div>
    </div>
  )
}
