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


// --- Trigger categories (dispatch mechanism) ---
export type NotificationTriggerCategory = 'by_system' | 'terjadwal' | 'manual_trigger'

const TRIGGER_CATEGORY_BY_SYSTEM_EVENTS = [
  'iuran_bill_created',
  'iuran_payment_submitted',
  'iuran_payment_verified',
  'activity_created',
  'activity_updated',
  'activity_cancelled',
  'activity_completed',
] as const

const TRIGGER_CATEGORY_TERJADWAL_EVENTS = [
  'iuran_payment_reminder',
  'iuran_upcoming_due_3_months',
  'iuran_overdue_3_months',
  'activity_reminder',
] as const

const TRIGGER_CATEGORY_MANUAL_EVENTS = [
  'waste_truck_arrival',
  'manual_push',
  'rw_cash_in',
  'rw_cash_out',
] as const

/**
 * Classify the dispatch mechanism of the event (based on the actual system
 * dispatch: automatic handler / scheduler / explicit officer action).
 * Prefers the trigger_category emitted by the backend when present.
 */
export function isBySystemTriggerEvent(eventType: string): boolean {
  return getNotificationTriggerCategory(eventType) === 'by_system'
}

export function getNotificationTriggerCategory(eventType: string): NotificationTriggerCategory {
  if ((TRIGGER_CATEGORY_BY_SYSTEM_EVENTS as readonly string[]).includes(eventType)) {
    return 'by_system'
  }
  if ((TRIGGER_CATEGORY_TERJADWAL_EVENTS as readonly string[]).includes(eventType)) {
    return 'terjadwal'
  }
  if ((TRIGGER_CATEGORY_MANUAL_EVENTS as readonly string[]).includes(eventType)) {
    return 'manual_trigger'
  }
  // Unknown event: fall back by dispatch metadata via fixed-set fallbacks.
  if (isFixedSystemEvent(eventType)) return 'by_system'
  return 'terjadwal'
}

export function getNotificationTriggerCategoryLabel(category: NotificationTriggerCategory): string {
  switch (category) {
    case 'by_system':
      return 'By System'
    case 'terjadwal':
      return 'Terjadwal'
    case 'manual_trigger':
      return 'Manual Trigger'
  }
}

export function getNotificationTriggerCategoryVariant(
  category: NotificationTriggerCategory,
): 'blue' | 'amber' | 'violet' {
  switch (category) {
    case 'by_system':
      return 'blue'
    case 'terjadwal':
      return 'amber'
    case 'manual_trigger':
      return 'violet'
  }
}

export const TRIGGER_CATEGORY_ORDER: readonly NotificationTriggerCategory[] = [
  'terjadwal',
  'manual_trigger',
  'by_system',
]

/**
 * Place By-System events at the very bottom while preserving the relative
 * order of the rest (per previous ordering requirement for the system list).
 */
export function sortSystemTypesLast<T extends { event_type: string; trigger_category?: string }>(
  types: T[],
): T[] {
  const rest: T[] = []
  const bySystem: T[] = []
  for (const st of types) {
    const category = getNotificationTriggerCategory(st.event_type)
    if (category === 'by_system') {
      bySystem.push(st)
    } else {
      rest.push(st)
    }
  }
  return [...rest, ...bySystem]
}
