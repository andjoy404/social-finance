package notification

import (
	"fmt"
	"testing"
)

func TestCountSystemTypes(t *testing.T) {
	allTypes := make([]SystemNotificationType, 0, len(SystemNotificationTypes)+len(RWSystemNotificationTypes))
	allTypes = append(allTypes, SystemNotificationTypes...)
	allTypes = append(allTypes, RWSystemNotificationTypes...)
	
	fmt.Printf("\n=== ALL SYSTEM TYPES (%d) ===\n", len(allTypes))
	for i, t := range allTypes {
		fmt.Printf("%2d. %-35s trigger_category=%s\n", i+1, t.EventType, TriggerCategoryForEvent(t.EventType))
	}
	
	if len(allTypes) != 19 {
		t.Errorf("expected 19 system types, got %d", len(allTypes))
	}
}
