package notification

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ActivityInfoFetch returns the notification-relevant projection of an activity.
// pubStatus must be anything — caller is responsible for lifecycle checks.
func (r *pgRepository) ActivityInfoFetch(ctx context.Context, rtID, activityID string) (*ActivityPayload, error) {
	return r.fetchActivityFull(ctx, rtID, activityID)
}

// fetchActivityFull is the canonical full-fidelity projection used by the
// service layer for both lifecycle checks and payload rendering.
func (r *pgRepository) fetchActivityFull(ctx context.Context, rtID, activityID string) (*ActivityPayload, error) {
	var (
		title             string
		desc, loc, org    sql.NullString
		startsAt          time.Time
		endsAt            sql.NullTime
		status, publicationStatus string
		createdAt, updatedAt time.Time
	)
	err := r.pool.Raw().QueryRowContext(ctx, `
		SELECT title, description, starts_at, ends_at, location, organizer, status, publication_status, created_at, updated_at
		FROM activities
		WHERE id = $1 AND rt_id = $2
	`, activityID, rtID).Scan(&title, &desc, &startsAt, &endsAt, &loc, &org, &status, &publicationStatus, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("query activity %s: %w", activityID, err)
	}
	p := buildActivityPayload(
		activityID, rtID, title, desc, startsAt, endsAt, loc, org, status, publicationStatus, createdAt, updatedAt,
	)
	return &p, nil
}

// ListActivityStateCandidates returns published activities that may need
// a lifecycle event (cancelled or completed) notification sweep.
func (r *pgRepository) ListActivityStateCandidates(ctx context.Context, now time.Time) ([]ActivityPayload, error) {
	rows, err := r.pool.Raw().QueryContext(ctx, `
		SELECT id, rt_id, title, description, starts_at, ends_at, location, organizer, status, publication_status, created_at, updated_at
		FROM activities
		WHERE publication_status = 'published'
		  AND (status = 'cancelled' OR (ends_at IS NOT NULL AND ends_at <= $1))
		ORDER BY ends_at NULLS LAST, id
	`, now)
	if err != nil {
		return nil, fmt.Errorf("list activity state candidates: %w", err)
	}
	defer rows.Close()

	var list []ActivityPayload
	for rows.Next() {
		var (
			id, rtID, title     string
			desc, loc, org      sql.NullString
			startsAt            time.Time
			endsAt              sql.NullTime
			status, pubStatus   string
			createdAt, updatedAt time.Time
		)
		if err := rows.Scan(&id, &rtID, &title, &desc, &startsAt, &endsAt, &loc, &org, &status, &pubStatus, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan activity state candidate: %w", err)
		}
		list = append(list,
			buildActivityPayload(id, rtID, title, desc, startsAt, endsAt, loc, org, status, pubStatus, createdAt, updatedAt))
	}
	return list, rows.Err()
}

func buildActivityPayload(
	id, rtID, title string,
	desc sql.NullString,
	startsAt time.Time,
	endsAt sql.NullTime,
	loc, org sql.NullString,
	status, publicationStatus string,
	createdAt, updatedAt time.Time,
) ActivityPayload {
	a := ActivityPayload{
		ID:                id,
		RTID:              rtID,
		Title:             title,
		StartsAt:          startsAt,
		Status:            status,
		PublicationStatus: publicationStatus,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}
	if desc.Valid {
		s := desc.String
		a.Description = &s
	}
	if loc.Valid {
		s := loc.String
		a.Location = &s
	}
	if org.Valid {
		s := org.String
		a.Organizer = &s
	}
	if endsAt.Valid {
		// Copy value so pointer remains valid after NULL conversion path.
		t := endsAt.Time
		a.EndsAt = &t
	}
	return a
}
