package notification

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"social-finance/internal/testutil"
)

func TestSystemTypesTriggerCategoryComplete(t *testing.T) {
	for _, tp := range SystemNotificationTypes {
		t.Run(tp.EventType, func(t *testing.T) {
			require.NotEmpty(t, tp.TriggerCategory, "event %s must have a trigger_category", tp.EventType)
			require.Equal(t, TriggerCategoryForEvent(tp.EventType), tp.TriggerCategory,
				"event %s trigger_category must match TriggerCategoryForEvent", tp.EventType)
			require.Contains(t, []string{TriggerCategoryBySystem, TriggerCategoryTerjadwal, TriggerCategoryManualTrigger}, tp.TriggerCategory,
				"event %s has unknown trigger_category %q", tp.EventType, tp.TriggerCategory)
		})
	}
}

func TestTriggerCategoryClassification(t *testing.T) {
	bySystem := map[string]string{
		EventBillCreated:       "Tagihan Iuran Baru",
		EventPaymentSubmitted:  "Pembayaran Iuran Masuk",
		EventPaymentVerified:   "Verifikasi Pembayaran Iuran",
		EventActivityCreated:   "Kegiatan Baru Diterbitkan",
		EventActivityUpdated:   "Kegiatan Diperbarui",
		EventActivityCancelled: "Kegiatan Dibatalkan",
		EventActivityCompleted: "Kegiatan Selesai",
	}
	terjadwal := map[string]string{
		EventPaymentReminder:    "Pengingat Pembayaran Iuran",
		EventUpcomingDue3Months: "Pengingat Pembayaran Iuran (3 Bulan Sebelum Jatuh Tempo)",
		EventOverdue3Months:     "Pengingat Tunggakan Iuran (3 Bulan+)",
		EventActivityReminder:   "Pengingat Kegiatan (H-<D>)",
	}
	manualTrigger := map[string]string{
		EventWasteTruckArrival: "Kedatangan Mobil Pengangkut Sampah",
		EventManualPush:        "Pengumuman Warga Manual",
	}

	for eventType, name := range bySystem {
		t.Run("by_system/"+name, func(t *testing.T) {
			require.Equal(t, TriggerCategoryBySystem, TriggerCategoryForEvent(eventType))
			require.True(t, IsFixedSystemEvent(eventType))
			st := map[string]SystemNotificationType{}
			for _, tp := range SystemNotificationTypes {
				st[tp.EventType] = tp
			}
			require.Equal(t, TriggerCategoryBySystem, st[eventType].TriggerCategory)
		})
	}
	for eventType, name := range terjadwal {
		t.Run("terjadwal/"+name, func(t *testing.T) {
			require.Equal(t, TriggerCategoryTerjadwal, TriggerCategoryForEvent(eventType))
			require.False(t, IsFixedSystemEvent(eventType))
			st := map[string]SystemNotificationType{}
			for _, tp := range SystemNotificationTypes {
				st[tp.EventType] = tp
			}
			require.Equal(t, TriggerCategoryTerjadwal, st[eventType].TriggerCategory)
		})
	}
	for eventType, name := range manualTrigger {
		t.Run("manual_trigger/"+name, func(t *testing.T) {
			require.Equal(t, TriggerCategoryManualTrigger, TriggerCategoryForEvent(eventType))
			require.False(t, IsFixedSystemEvent(eventType))
			st := map[string]SystemNotificationType{}
			for _, tp := range SystemNotificationTypes {
				st[tp.EventType] = tp
			}
			require.Equal(t, TriggerCategoryManualTrigger, st[eventType].TriggerCategory)
		})
	}
}

func TestGetSystemTypesOrderingBySystemLast(t *testing.T) {
	pool := testutil.GetTestPool(t)
	defer pool.Close()

	svc := NewService(NewRepository(pool), NewMockPushProvider())
	types := svc.GetSystemTypes()

	firstBySystem := -1
	for i, tp := range types {
		if tp.TriggerCategory == TriggerCategoryBySystem {
			if firstBySystem == -1 {
				firstBySystem = i
			}
			continue
		}
		if firstBySystem != -1 {
			t.Fatalf("non-By-System event %q (index %d) appears after By-System event at index %d; By-System events must be placed last", tp.EventType, i, firstBySystem)
		}
	}
	require.NotEqual(t, -1, firstBySystem, "expected at least one by_system event in the list")

	// Relative order of non-By-System entries must match the source slice order.
	srcNon := []string{}
	for _, tp := range SystemNotificationTypes {
		if tp.TriggerCategory != TriggerCategoryBySystem {
			srcNon = append(srcNon, tp.EventType)
		}
	}
	outNon := []string{}
	for _, tp := range types {
		if tp.TriggerCategory != TriggerCategoryBySystem {
			outNon = append(outNon, tp.EventType)
		}
	}
	for i := range srcNon {
		require.Equal(t, srcNon[i], outNon[i], "non-By-System relative order must be preserved at index %d", i)
	}
}

func TestBySystemEventsCannotBeCreatedScheduledToggledOrDeleted(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "bysys")
	defer cleanupNotifTestRT(t, pool, rtID)
	ctx := context.Background()

	svc := NewService(NewRepository(pool), NewMockPushProvider())

	bySystem := TriggerCategoryBySystemEvents()
	require.NotEmpty(t, bySystem)

	for _, eventType := range bySystem {
		t.Run("create_template_blocked__"+eventType, func(t *testing.T) {
			_, err := svc.CreateTemplate(ctx, rtID, CreateTemplateInput{
				Name:    "XSS " + eventType,
				EventType: eventType,
				Title:     "Title",
				Body:      "Body",
			})
			require.Error(t, err)
		})
	}

	// Create templates directly in DB as fixtures for toggle/delete/schedule assertions.
	fixtureEvent := TriggerCategoryBySystemEvents()[0]
	templateID := insertActivityTemplate(t, pool, rtID, fixtureEvent,
		"By System Template", "By System Title", "By System Body")

	t.Run("toggle_blocked_by_system", func(t *testing.T) {
		_, err := svc.ToggleTemplateActive(ctx, rtID, templateID)
		require.Error(t, err)
	})

	t.Run("delete_blocked_by_system", func(t *testing.T) {
		err := svc.DeleteTemplate(ctx, rtID, templateID)
		require.Error(t, err)
	})

	t.Run("create_schedule_blocked_by_system", func(t *testing.T) {
		_, err := svc.CreateSchedule(ctx, rtID, "user-x", CreateScheduleInput{
			EventType:   fixtureEvent,
			TemplateID:  templateID,
			Name:        "Schedule " + fixtureEvent,
			SendDays:    []int{1},
			SendHour:    8,
			SendMinute:  0,
			Timezone:    "Asia/Jakarta",
		})
		require.Error(t, err)
	})

	t.Run("toggle_schedule_blocked_by_system", func(t *testing.T) {
		// Insert a schedule row directly for a by-system event to simulate
		// legacy/existing data, then ensure pengurus cannot toggle it.
		bySysTemplate := insertActivityTemplate(t, pool, rtID, EventActivityCreated, "X", "X", "X")
		activityID := insertActivity(t, pool, rtID, "By Sys Activity")
		bySysSchedule := createActivitySchedule(t, pool, rtID, bySysTemplate, "S", activityID, time.Now().Add(time.Hour))
		_, err := svc.ToggleScheduleActive(ctx, rtID, bySysSchedule, "user-x")
		require.Error(t, err)
	})

	t.Run("terjadwal_and_manual_still_configurable", func(t *testing.T) {
		_, err := svc.CreateTemplate(ctx, rtID, CreateTemplateInput{
			Name:      "Boleh Edit",
			EventType: EventActivityReminder,
			Title:     "Title",
			Body:      "Body {{nama_kegiatan}}",
		})
		require.NoError(t, err)

		_, err = svc.CreateTemplate(ctx, rtID, CreateTemplateInput{
			Name:      "Boleh Edit 2",
			EventType: EventWasteTruckArrival,
			Title:     "Title",
			Body:      "Body",
		})
		require.NoError(t, err)
	})
}
