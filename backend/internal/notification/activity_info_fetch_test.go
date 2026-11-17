package notification

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"social-finance/internal/testutil"
)

// TestFetchActivityInfo_ValidMapping covers SQL Scan mapping for status, publication_status, ends_at and description/location/organizer NULL handling.
// We insert three fixtures (published, cancelled, completed) with distinct marker values and assert exact field mapping.
func TestFetchActivityInfo_ValidMapping(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "FIM")
	defer cleanupNotifTestRT(t, pool, rtID)

	now := time.Now()

	actPublished := insertActivity(t, pool, rtID, "Kegiatan Mapping 1")
	actCancelled := insertActivity(t, pool, rtID, "Kegiatan Mapping 2",
		map[string]string{"status": "'cancelled'"})
	actCompleted := insertActivity(t, pool, rtID, "Kegiatan Mapping 3",
		map[string]string{"ends_at": "NOW() - INTERVAL '2 hours'"})

	repo := NewRepository(pool).(*pgRepository)

	tests := []struct {
		name              string
		activityID        string
		wantStatus        string
		wantPubStatus     string
		wantCancelEndsAt  bool
		wantCompletedAt   bool
	}{
		{name: "published", activityID: actPublished, wantStatus: "upcoming", wantPubStatus: "published"},
		{name: "cancelled", activityID: actCancelled, wantStatus: "cancelled", wantPubStatus: "published"},
		{name: "completed", activityID: actCompleted, wantStatus: "upcoming", wantPubStatus: "published", wantCompletedAt: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := repo.ActivityInfoFetch(context.Background(), rtID, tt.activityID)
			if err != nil {
				t.Fatalf("ActivityInfoFetch err: %v", err)
			}
			if payload == nil {
				t.Fatal("payload must not be nil")
			}
			if payload.ID != tt.activityID {
				t.Errorf("ID = %q, want %q", payload.ID, tt.activityID)
			}
			if payload.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", payload.Status, tt.wantStatus)
			}
			if payload.PublicationStatus != tt.wantPubStatus {
				t.Errorf("PublicationStatus = %q, want %q", payload.PublicationStatus, tt.wantPubStatus)
			}
			if payload.Title == "" {
				t.Error("Title must not be empty — ensures Scan did not read poster_url into title")
			}
			if payload.Description == nil || *payload.Description != "Deskripsi kegiatan" {
				t.Errorf("Description = %v, want non-nil 'Deskripsi kegiatan'", payload.Description)
			}
			if payload.Location == nil || *payload.Location != "Balai RT" {
				t.Errorf("Location = %v, want 'Balai RT'", payload.Location)
			}
			if payload.Organizer == nil || *payload.Organizer != "Pengurus RT" {
				t.Errorf("Organizer = %v, want 'Pengurus RT'", payload.Organizer)
			}
			if tt.wantCompletedAt {
				if payload.EndsAt == nil {
					t.Fatal("EndsAt must not be nil for completed fixture")
				}
				if payload.EndsAt.After(now) {
					t.Errorf("EndsAt = %s, expected in the past", payload.EndsAt)
				}
			} else {
				if payload.EndsAt == nil {
					t.Fatalf("EndsAt must not be nil (default fixture ends tomorrow)")
				}
			}
			if payload.StartsAt.IsZero() {
				t.Error("StartsAt must not be zero")
			}
		})
	}

	// fetchActivityFull separately must also pass the mapping
	full, err := repo.fetchActivityFull(context.Background(), rtID, actCancelled)
	if err != nil {
		t.Fatalf("fetchActivityFull err: %v", err)
	}
	if full.Status != "cancelled" {
		t.Errorf("fetchActivityFull.Status = %q, want %q", full.Status, "cancelled")
	}
	if full.Title != "Kegiatan Mapping 2" {
		t.Errorf("fetchActivityFull.Title = %q, want %q", full.Title, "Kegiatan Mapping 2")
	}
}

// TestFetchActivityInfo_StatusMappingSweep verifies ListActivityStateCandidates
// correctly picks out cancelled or completed published activities and preserves
// mapping for all three states.
func TestFetchActivityInfo_StatusMappingSweep(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "FIMSWP")
	defer cleanupNotifTestRT(t, pool, rtID)

	now := time.Now()

	// Completed: ends_at in the past
	actCompleted := insertActivity(t, pool, rtID, "Sweep Completed",
		map[string]string{"ends_at": "NOW() - INTERVAL '3 hours'"})
	// Cancelled: status cancelled, still future ends_at
	actCancelled := insertActivity(t, pool, rtID, "Sweep Cancelled",
		map[string]string{"status": "'cancelled'"})
	// Upcoming: not eligible yet
	_ = insertActivity(t, pool, rtID, "Sweep Upcoming")

	repo := NewRepository(pool).(*pgRepository)
	candidates, err := repo.ListActivityStateCandidates(context.Background(), now)
	if err != nil {
		t.Fatalf("ListActivityStateCandidates err: %v", err)
	}

	foundByID := map[string]ActivityPayload{}
	for _, c := range candidates {
		foundByID[c.ID] = c
	}

	if _, ok := foundByID[actCompleted]; !ok {
		t.Error("completed activity (ends_at past) must be included in state candidates")
	}
	if _, ok := foundByID[actCancelled]; !ok {
		t.Error("cancelled published activity must be included in state candidates")
	}

	// Mapping for candidates
	for id, payload := range foundByID {
		if payload.RTID != rtID {
			t.Errorf("payload RTID = %q, want %q", payload.RTID, rtID)
		}
		if payload.Title == "" {
			t.Error("candidate title cannot be empty — ensures correct Scan ordering")
		}
		switch id {
		case actCancelled:
			if payload.Status != "cancelled" {
				t.Errorf("cancelled candidate Status = %q, want %q", payload.Status, "cancelled")
			}
		case actCompleted:
			if payload.Status != "upcoming" {
				t.Errorf("completed candidate Status = %q, want %q", payload.Status, "upcoming")
			}
			if payload.EndsAt == nil {
				t.Fatal("completed candidate EndsAt must not be nil")
			}
		}

		if payload.PublicationStatus != "published" {
			t.Errorf("candidate publication_status = %q, want %q", payload.PublicationStatus, "published")
		}
	}
}

// TestFetchActivityFull_NotFound ensures fetchActivityFull / ActivityInfoFetch return sql.ErrNoRows for missing activity.
func TestFetchActivityFull_NotFound(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "FIMNF")
	defer cleanupNotifTestRT(t, pool, rtID)

	repo := NewRepository(pool).(*pgRepository)
	_, err := repo.fetchActivityFull(context.Background(), rtID, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("fetchActivityFull for missing ID must return an error, not nil")
	}
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	_, err = repo.ActivityInfoFetch(context.Background(), rtID, "00000000-0000-0000-0000-000000000000")
	if err == nil {
		t.Fatal("ActivityInfoFetch for missing ID must return an error, not nil")
	}
	fmt.Printf("ActivityInfoFetch err: %v\n", err)
}

// TestFetchActivityFull_NULLDescriptionAndLocation verifies COALESCE handling of NULL optional columns.
func TestFetchActivityFull_NULLDescriptionAndLocation(t *testing.T) {
	pool := testutil.GetTestPool(t)
	rtID := setupNotifTestRT(t, pool, "FIMNULL")
	defer cleanupNotifTestRT(t, pool, rtID)

	// Explicitly insert with NULLs by clearing default-populated columns after insert.
	actID := insertActivity(t, pool, rtID, "Kegiatan NULL")
	_, err := pool.Raw().ExecContext(context.Background(),
		`UPDATE activities SET description = NULL, location = NULL, organizer = NULL WHERE id = $1`, actID)
	if err != nil {
		t.Fatalf("set NULLs: %v", err)
	}

	repo := NewRepository(pool).(*pgRepository)
	payload, err := repo.fetchActivityFull(context.Background(), rtID, actID)
	if err != nil {
		t.Fatalf("fetchActivityFull err: %v", err)
	}
	if payload.Description != nil {
		t.Errorf("Description = %v, want nil for NULL column", *payload.Description)
	}
	if payload.Location != nil {
		t.Errorf("Location = %v, want nil for NULL column", *payload.Location)
	}
	if payload.Organizer != nil {
		t.Errorf("Organizer = %v, want nil for NULL column", *payload.Organizer)
	}
}
