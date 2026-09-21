import { useAuth } from '@/app/AuthContext'
import { formatRupiah } from '@/styles/format'
import { FinancialChart } from '@/components/FinancialChart'
import { PageHeader } from '@/components/PageHeader'
import { DashboardOutlined } from '@ant-design/icons'

import { dashboard, financialData, recentTransactions, iuranData } from '@/mocks/dashboard'

function roleLabel(sys?: string | null, tenant?: string): string {
  if (sys === 'super_admin') return 'Super Admin'
  const labels: Record<string, string> = { pengurus: 'Pengurus', bendahara: 'Bendahara', warga: 'Warga' }
  return labels[tenant ?? ''] ?? tenant ?? ''
}

export function Dashboard() {
  const { user } = useAuth()
  if (!user) return null

  const label = roleLabel(user.systemRole, user.role)
  const roleColors: Record<string, string> = {
    'Super Admin': 'var(--sf-danger)',
    'Pengurus': 'var(--sf-accent)',
    'Bendahara': 'var(--sf-info)',
    'Warga': 'var(--sf-success)',
  }

  return (
    <>
      {/* PageHeader surface */}
      <PageHeader
        surface
        icon={<DashboardOutlined />}
        title={`Selamat datang, ${user.name}`}
        subtitle={user.rt?.name ?? ''}
        actions={
          <span className="sf-badge" style={{ color: roleColors[label] || 'var(--sf-text)', background: 'var(--sf-surface-hover)' }}>
            {label}
          </span>
        }
      />

      {/* Metrics row */}
      <div className="sf-metrics-row">
        <div className="sf-metric-card">
          <div className="sf-metric-label">Saldo Kas</div>
          <div className="sf-metric-value sf-metric-default">
            {formatRupiah(dashboard.balance)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Pemasukan Bulan Ini</div>
          <div className="sf-metric-value sf-metric-income">
            {formatRupiah(dashboard.monthlyIncome)}
          </div>
        </div>

        <div className="sf-metric-card">
          <div className="sf-metric-label">Pengeluaran Bulan Ini</div>
          <div className="sf-metric-value sf-metric-expense">
            {formatRupiah(dashboard.monthlyExpense)}
          </div>
        </div>
      </div>

      {/* Financial chart */}
      <div className="sf-section-card">
        <div className="sf-section-header">
          <div>
            <div className="sf-section-title">Ringkasan Keuangan</div>
            <div className="sf-section-subtitle">6 bulan terakhir</div>
          </div>
        </div>
        <div style={{ height: 240 }}>
          <FinancialChart data={financialData} />
        </div>
      </div>

      {/* Bottom row */}
      <div className="sf-data-grid sf-data-grid-compact">
        {/* Iuran progress */}
        <div className="sf-section-card">
          <div className="sf-section-title">Iuran Bulan Ini</div>
          <div style={{ marginTop: 20 }}>
            <div style={{
              height: 6,
              background: 'var(--sf-border)',
              borderRadius: 999,
              overflow: 'hidden',
              marginBottom: 8,
            }}>
              <div style={{
                width: `${iuranData.percentage}%`,
                height: '100%',
                background: 'var(--sf-accent)',
                borderRadius: 999,
                transition: 'width 0.3s ease',
              }} />
            </div>
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'baseline',
            }}>
              <span style={{
                fontSize: 22,
                fontWeight: 700,
                color: 'var(--sf-accent)',
                lineHeight: 1.2,
              }}>
                {iuranData.paid}/{iuranData.total}
              </span>
              <span style={{
                fontSize: 12,
                color: 'var(--sf-text-muted)',
                fontWeight: 500,
              }}>
                {iuranData.percentage}% terkumpul
              </span>
            </div>
          </div>
        </div>

        {/* Recent transactions */}
        <div className="sf-section-card">
          <div className="sf-section-title">Transaksi Terbaru</div>
          <div style={{ marginTop: 4 }}>
            <table className="sf-table">
              <thead>
                <tr>
                  <th>Keterangan</th>
                  <th>Tanggal</th>
                  <th>Jumlah</th>
                </tr>
              </thead>
              <tbody>
                {recentTransactions.map((t) => (
                  <tr key={t.id}>
                    <td>{t.description}</td>
                    <td>{formatDate(t.date)}</td>
                    <td style={{
                      color: t.type === 'income' ? 'var(--sf-success)' : 'var(--sf-danger)',
                    }}>
                      {t.type === 'income' ? '+' : '\u2212'} {formatRupiah(t.amount)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </>
  )
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' })
}
