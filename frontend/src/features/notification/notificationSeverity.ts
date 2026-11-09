export const FIXED_SYSTEM_EVENTS = [
  'iuran_bill_created',
  'iuran_payment_submitted',
  'iuran_payment_verified',
] as const

export function isFixedSystemEvent(eventType: string): boolean {
  return (FIXED_SYSTEM_EVENTS as readonly string[]).includes(eventType)
}

export type NotificationSeverity = 'informational' | 'normal' | 'warning' | 'urgent'

export function getNotificationSeverity(eventType: string): NotificationSeverity {
  switch (eventType) {
    case 'payment_rejected':
    case 'iuran_overdue_3_months':
      return 'urgent'
    case 'iuran_payment_reminder':
    case 'iuran_upcoming_due_3_months':
    case 'iuran_payment_submitted':
    case 'rw_cash_out':
      return 'warning'
    case 'payment_verified':
    case 'iuran_payment_verified':
    case 'rw_cash_in':
      return 'normal'
    case 'iuran_bill_created':
    case 'manual_push':
    case 'waste_truck_arrival':
    default:
      return 'informational'
  }
}

export function getNotificationBadgeVariant(eventType: string): 'blue' | 'green' | 'amber' | 'red' {
  const severity = getNotificationSeverity(eventType)
  switch (severity) {
    case 'urgent':
      return 'red'
    case 'warning':
      return 'amber'
    case 'normal':
      return 'green'
    case 'informational':
    default:
      return 'blue'
  }
}

