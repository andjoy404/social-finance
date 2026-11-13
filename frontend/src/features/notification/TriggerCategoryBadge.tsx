import { Badge } from '@/components/Badge'
import {
  getNotificationTriggerCategory,
  getNotificationTriggerCategoryLabel,
  getNotificationTriggerCategoryVariant,
  type NotificationTriggerCategory,
} from './notificationSeverity'

export function TriggerCategoryBadge({
  eventType,
  category,
}: {
  eventType?: string
  category?: NotificationTriggerCategory
}) {
  const resolved: NotificationTriggerCategory =
    category ?? (eventType ? getNotificationTriggerCategory(eventType) : 'terjadwal')
  return (
    <Badge variant={getNotificationTriggerCategoryVariant(resolved)}>
      {getNotificationTriggerCategoryLabel(resolved)}
    </Badge>
  )
}
