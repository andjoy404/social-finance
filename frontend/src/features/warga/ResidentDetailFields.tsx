// ResidentDetailFields — renders the same form-field layout as Ubah / Create
// but all inputs are disabled/read-only. No role/relationship info displayed.
import type { ApiResident } from '@/app/api'
import {
  UserOutlined,
  IdcardOutlined,
  PhoneOutlined,
  MailOutlined,
  CalendarOutlined,
} from '@ant-design/icons'

function formatIntegerOnly(value: string | number | null | undefined): string {
  if (value == null) return '\u2014'
  if (typeof value === 'number') {
    return isNaN(value) ? '\u2014' : String(Math.floor(value))
  }
  const str = String(value).trim()
  if (!str || str === '\u2014' || str === '-') return '\u2014'
  const match = str.replace(/^(rt|rw)\s*/i, '').match(/\d+/)
  if (match) {
    const num = parseInt(match[0], 10)
    return isNaN(num) ? '\u2014' : String(num)
  }
  return '\u2014'
}

const readOnlyStyle: React.CSSProperties = {
  cursor: 'default',
  opacity: 0.7,
}

export function ResidentDetailFields({
  resident,
  showNik,
}: {
  resident: ApiResident
  showNik: boolean
}) {
  return (
    <>
      {/* Nama Lengkap */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rd-name">
          <UserOutlined style={{ marginRight: 4 }} />
          Nama Lengkap
        </label>
        <input
          id="rd-name"
          className="sf-form-input"
          type="text"
          value={resident.full_name ?? ''}
          readOnly
          disabled
          aria-readonly
          style={readOnlyStyle}
        />
      </div>

      {/* NIK + Telepon */}
      <div className="sf-form-grid-2">
        {showNik && (
          <div className="sf-form-field">
            <label className="sf-form-label" htmlFor="rd-nik">
              <IdcardOutlined style={{ marginRight: 4 }} />
              NIK
            </label>
            <input
              id="rd-nik"
              className="sf-form-input"
              type="text"
              value={resident.nik ?? ''}
              readOnly
              disabled
              aria-readonly
              style={readOnlyStyle}
            />
          </div>
        )}
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="rd-phone">
            <PhoneOutlined style={{ marginRight: 4 }} />
            Telepon
          </label>
          <input
            id="rd-phone"
            className="sf-form-input"
            type="text"
            value={resident.phone ?? ''}
            readOnly
            disabled
            aria-readonly
            style={readOnlyStyle}
          />
        </div>
      </div>

      {/* Email */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rd-email">
          <MailOutlined style={{ marginRight: 4 }} />
          Email
        </label>
        <input
          id="rd-email"
          className="sf-form-input"
          type="text"
          value={resident.email ?? ''}
          readOnly
          disabled
          aria-readonly
          style={readOnlyStyle}
        />
      </div>

      {/* Tanggal Mulai */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rd-start-date">
          <CalendarOutlined style={{ marginRight: 4 }} />
          Tanggal Mulai
        </label>
        <input
          id="rd-start-date"
          className="sf-form-input"
          type="text"
          value={resident.start_date ?? ''}
          readOnly
          disabled
          aria-readonly
          style={readOnlyStyle}
        />
      </div>

      {/* RT + RW */}
      <div className="sf-form-grid-2">
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="rd-rt">RT</label>
          <input
            id="rd-rt"
            className="sf-form-input"
            type="text"
            value={
              formatIntegerOnly(resident.rt_number) !== '\u2014'
                ? formatIntegerOnly(resident.rt_number)
                : ''
            }
            readOnly
            disabled
            aria-readonly
            style={readOnlyStyle}
          />
        </div>
        <div className="sf-form-field">
          <label className="sf-form-label" htmlFor="rd-rw">RW</label>
          <input
            id="rd-rw"
            className="sf-form-input"
            type="text"
            value={
              formatIntegerOnly(resident.rw) !== '\u2014'
                ? formatIntegerOnly(resident.rw)
                : ''
            }
            readOnly
            disabled
            aria-readonly
            style={readOnlyStyle}
          />
        </div>
      </div>

      {/* Nama RT */}
      <div className="sf-form-field">
        <label className="sf-form-label" htmlFor="rd-rt-name">Nama RT</label>
        <input
          id="rd-rt-name"
          className="sf-form-input"
          type="text"
          value={resident.rt_name ?? ''}
          readOnly
          disabled
          aria-readonly
          style={readOnlyStyle}
        />
      </div>
    </>
  )
}
