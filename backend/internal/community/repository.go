package community

import (
	"context"
	"database/sql"
	"fmt"

	"social-finance/internal/database"
)

// Repository handles database operations for community features.
type Repository struct {
	pool *database.Pool
}

// NewRepository creates a new community Repository.
func NewRepository(pool *database.Pool) *Repository {
	return &Repository{pool: pool}
}

// querier abstracts *sql.DB and *sql.Tx for transactional repository calls.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// db returns the tx when non-nil, otherwise the pool's raw *sql.DB.
func (r *Repository) db(tx *sql.Tx) querier {
	if tx != nil {
		return tx
	}
	return r.pool.Raw()
}

// BeginTx starts a new database transaction for coupled source+banner operations.
func (r *Repository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.pool.Raw().BeginTx(ctx, nil)
}

// -----------------------------------------------------------------------------
// Activities
// -----------------------------------------------------------------------------

func (r *Repository) ListActivities(ctx context.Context, tx *sql.Tx, rtID, pubStatus, status string, limit, offset int) ([]Activity, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	whereClause := "WHERE rt_id = $1"
	args := []any{rtID}
	argIdx := 2

	if pubStatus != "" {
		whereClause += fmt.Sprintf(" AND publication_status = $%d", argIdx)
		args = append(args, pubStatus)
		argIdx++
	}
	if status != "" {
		// Filter by DYNAMIC execution status (single source of truth):
		// cancelled is stored; upcoming/ongoing/completed derive from starts_at/ends_at.
		whereClause += fmt.Sprintf(` AND CASE
			WHEN status = 'cancelled' THEN 'cancelled'
			WHEN ends_at IS NOT NULL AND ends_at <= NOW() THEN 'completed'
			WHEN starts_at <= NOW() THEN 'ongoing'
			ELSE 'upcoming'
		END = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM activities %s", whereClause)
	var total int
	if err := r.db(tx).QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count activities: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, rt_id, title, description, poster_url, starts_at, ends_at,
		       location, organizer, status, publication_status, show_on_banner,
		       created_by, updated_by, created_at, updated_at
		FROM activities
		%s
		ORDER BY starts_at ASC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db(tx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query activities: %w", err)
	}
	defer rows.Close()

	var list []Activity
	for rows.Next() {
		var a Activity
		var desc, poster, loc, org, cBy, uBy sql.NullString
		var endsAt sql.NullTime
		if err := rows.Scan(
			&a.ID, &a.RTID, &a.Title, &desc, &poster, &a.StartsAt, &endsAt,
			&loc, &org, &a.Status, &a.PublicationStatus, &a.ShowOnBanner,
			&cBy, &uBy, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan activity: %w", err)
		}
		if desc.Valid {
			a.Description = &desc.String
		}
		if poster.Valid {
			a.PosterURL = &poster.String
		}
		if endsAt.Valid {
			a.EndsAt = &endsAt.Time
		}
		if loc.Valid {
			a.Location = &loc.String
		}
		if org.Valid {
			a.Organizer = &org.String
		}
		if cBy.Valid {
			a.CreatedBy = &cBy.String
		}
		if uBy.Valid {
			a.UpdatedBy = &uBy.String
		}
		list = append(list, a)
	}

	return list, total, nil
}

func (r *Repository) GetActivityByID(ctx context.Context, tx *sql.Tx, rtID, id string) (*Activity, error) {
	query := `
		SELECT id, rt_id, title, description, poster_url, starts_at, ends_at,
		       location, organizer, status, publication_status, show_on_banner,
		       created_by, updated_by, created_at, updated_at
		FROM activities
		WHERE rt_id = $1 AND id = $2
	`
	var a Activity
	var desc, poster, loc, org, cBy, uBy sql.NullString
	var endsAt sql.NullTime
	err := r.db(tx).QueryRowContext(ctx, query, rtID, id).Scan(
		&a.ID, &a.RTID, &a.Title, &desc, &poster, &a.StartsAt, &endsAt,
		&loc, &org, &a.Status, &a.PublicationStatus, &a.ShowOnBanner,
		&cBy, &uBy, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get activity by id: %w", err)
	}
	if desc.Valid {
		a.Description = &desc.String
	}
	if poster.Valid {
		a.PosterURL = &poster.String
	}
	if endsAt.Valid {
		a.EndsAt = &endsAt.Time
	}
	if loc.Valid {
		a.Location = &loc.String
	}
	if org.Valid {
		a.Organizer = &org.String
	}
	if cBy.Valid {
		a.CreatedBy = &cBy.String
	}
	if uBy.Valid {
		a.UpdatedBy = &uBy.String
	}
	return &a, nil
}

func (r *Repository) CreateActivity(ctx context.Context, tx *sql.Tx, a *Activity) error {
	query := `
		INSERT INTO activities (
			rt_id, title, description, poster_url, starts_at, ends_at,
			location, organizer, status, publication_status, show_on_banner,
			created_by, updated_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`
	return r.db(tx).QueryRowContext(ctx, query,
		a.RTID, a.Title, a.Description, a.PosterURL, a.StartsAt, a.EndsAt,
		a.Location, a.Organizer, a.Status, a.PublicationStatus, a.ShowOnBanner,
		a.CreatedBy,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

func (r *Repository) UpdateActivity(ctx context.Context, tx *sql.Tx, a *Activity) error {
	query := `
		UPDATE activities
		SET title = $3, description = $4, poster_url = $5, starts_at = $6, ends_at = $7,
		    location = $8, organizer = $9, show_on_banner = $10, updated_by = $11, updated_at = NOW()
		WHERE rt_id = $1 AND id = $2
		RETURNING updated_at
	`
	return r.db(tx).QueryRowContext(ctx, query,
		a.RTID, a.ID, a.Title, a.Description, a.PosterURL, a.StartsAt, a.EndsAt,
		a.Location, a.Organizer, a.ShowOnBanner, a.UpdatedBy,
	).Scan(&a.UpdatedAt)
}

func (r *Repository) PublishActivity(ctx context.Context, tx *sql.Tx, rtID, id, userID string) error {
	query := `
		UPDATE activities
		SET publication_status = 'published', updated_by = $3, updated_at = NOW()
		WHERE rt_id = $1 AND id = $2
	`
	res, err := r.db(tx).ExecContext(ctx, query, rtID, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) CancelActivity(ctx context.Context, tx *sql.Tx, rtID, id, userID string) error {
	query := `
		UPDATE activities
		SET status = 'cancelled', updated_by = $3, updated_at = NOW()
		WHERE rt_id = $1 AND id = $2
	`
	res, err := r.db(tx).ExecContext(ctx, query, rtID, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteActivity(ctx context.Context, tx *sql.Tx, rtID, id string) error {
	query := "DELETE FROM activities WHERE rt_id = $1 AND id = $2"
	res, err := r.db(tx).ExecContext(ctx, query, rtID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// -----------------------------------------------------------------------------
// Announcements
// -----------------------------------------------------------------------------

func (r *Repository) ListAnnouncements(ctx context.Context, tx *sql.Tx, rtID, status string, forWarga bool, limit, offset int) ([]Announcement, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	whereClause := "WHERE rt_id = $1"
	args := []any{rtID}
	argIdx := 2

	if forWarga {
		if status == "history" || status == "archived" {
			// Warga history: any non-draft announcement that is archived, expired, or past its scheduled_end
			whereClause += " AND status != 'draft' AND (status IN ('archived', 'expired') OR (scheduled_end IS NOT NULL AND scheduled_end <= NOW()))"
		} else if status == "all" {
			// All non-draft announcements for warga
			whereClause += " AND status != 'draft'"
		} else {
			// Active announcements inside display window
			whereClause += " AND status = 'active'"
			whereClause += " AND (scheduled_start IS NULL OR scheduled_start <= NOW())"
			whereClause += " AND (scheduled_end IS NULL OR scheduled_end > NOW())"
		}
	} else {
		if status != "" && status != "all" {
			whereClause += fmt.Sprintf(" AND status = $%d", argIdx)
			args = append(args, status)
			argIdx++
		}
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM announcements %s", whereClause)
	var total int
	if err := r.db(tx).QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count announcements: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT id, rt_id, title, body, image_url, deep_link, template_id,
		       scheduled_start, scheduled_end, priority, display_order, status,
		       published_at, snapshot_content, created_by, updated_by, created_at, updated_at
		FROM announcements
		%s
		ORDER BY priority DESC, created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db(tx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query announcements: %w", err)
	}
	defer rows.Close()

	var list []Announcement
	for rows.Next() {
		var an Announcement
		var img, link, tmplID, cBy, uBy sql.NullString
		var schedStart, schedEnd, pubAt sql.NullTime
		var snapshotBytes []byte
		if err := rows.Scan(
			&an.ID, &an.RTID, &an.Title, &an.Body, &img, &link, &tmplID,
			&schedStart, &schedEnd, &an.Priority, &an.DisplayOrder, &an.Status,
			&pubAt, &snapshotBytes, &cBy, &uBy, &an.CreatedAt, &an.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan announcement: %w", err)
		}
		if img.Valid {
			an.ImageURL = &img.String
		}
		if link.Valid {
			an.DeepLink = &link.String
		}
		if tmplID.Valid {
			an.TemplateID = &tmplID.String
		}
		if schedStart.Valid {
			an.ScheduledStart = &schedStart.Time
		}
		if schedEnd.Valid {
			an.ScheduledEnd = &schedEnd.Time
		}
		if pubAt.Valid {
			an.PublishedAt = &pubAt.Time
		}
		if cBy.Valid {
			an.CreatedBy = &cBy.String
		}
		if uBy.Valid {
			an.UpdatedBy = &uBy.String
		}
		if len(snapshotBytes) > 0 {
			an.SnapshotContent = snapshotBytes
		}
		list = append(list, an)
	}

	return list, total, nil
}

func (r *Repository) GetAnnouncementByID(ctx context.Context, tx *sql.Tx, rtID, id string) (*Announcement, error) {
	query := `
		SELECT id, rt_id, title, body, image_url, deep_link, template_id,
		       scheduled_start, scheduled_end, priority, display_order, status,
		       published_at, snapshot_content, created_by, updated_by, created_at, updated_at
		FROM announcements
		WHERE rt_id = $1 AND id = $2
	`
	var an Announcement
	var img, link, tmplID, cBy, uBy sql.NullString
	var schedStart, schedEnd, pubAt sql.NullTime
	var snapshotBytes []byte
	err := r.db(tx).QueryRowContext(ctx, query, rtID, id).Scan(
		&an.ID, &an.RTID, &an.Title, &an.Body, &img, &link, &tmplID,
		&schedStart, &schedEnd, &an.Priority, &an.DisplayOrder, &an.Status,
		&pubAt, &snapshotBytes, &cBy, &uBy, &an.CreatedAt, &an.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get announcement by id: %w", err)
	}
	if img.Valid {
		an.ImageURL = &img.String
	}
	if link.Valid {
		an.DeepLink = &link.String
	}
	if tmplID.Valid {
		an.TemplateID = &tmplID.String
	}
	if schedStart.Valid {
		an.ScheduledStart = &schedStart.Time
	}
	if schedEnd.Valid {
		an.ScheduledEnd = &schedEnd.Time
	}
	if pubAt.Valid {
		an.PublishedAt = &pubAt.Time
	}
	if cBy.Valid {
		an.CreatedBy = &cBy.String
	}
	if uBy.Valid {
		an.UpdatedBy = &uBy.String
	}
	if len(snapshotBytes) > 0 {
		an.SnapshotContent = snapshotBytes
	}
	return &an, nil
}

func (r *Repository) CreateAnnouncement(ctx context.Context, tx *sql.Tx, an *Announcement) error {
	snapshotJSON := an.SnapshotContent
	if len(snapshotJSON) == 0 {
		snapshotJSON = []byte("{}")
	}

	query := `
		INSERT INTO announcements (
			rt_id, title, body, image_url, deep_link, template_id,
			scheduled_start, scheduled_end, priority, display_order, status,
			published_at, snapshot_content, created_by, updated_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at
	`
	return r.db(tx).QueryRowContext(ctx, query,
		an.RTID, an.Title, an.Body, an.ImageURL, an.DeepLink, an.TemplateID,
		an.ScheduledStart, an.ScheduledEnd, an.Priority, an.DisplayOrder, an.Status,
		an.PublishedAt, snapshotJSON, an.CreatedBy,
	).Scan(&an.ID, &an.CreatedAt, &an.UpdatedAt)
}

func (r *Repository) UpdateAnnouncement(ctx context.Context, tx *sql.Tx, an *Announcement) error {
	query := `
		UPDATE announcements
		SET title = $3, body = $4, image_url = $5, deep_link = $6, template_id = $7,
		    scheduled_start = $8, scheduled_end = $9, priority = $10, display_order = $11,
		    updated_by = $12, updated_at = NOW()
		WHERE rt_id = $1 AND id = $2
		RETURNING updated_at
	`
	return r.db(tx).QueryRowContext(ctx, query,
		an.RTID, an.ID, an.Title, an.Body, an.ImageURL, an.DeepLink, an.TemplateID,
		an.ScheduledStart, an.ScheduledEnd, an.Priority, an.DisplayOrder, an.UpdatedBy,
	).Scan(&an.UpdatedAt)
}

func (r *Repository) PublishAnnouncement(ctx context.Context, tx *sql.Tx, rtID, id, userID, status string, snapshotContent []byte) error {
	if len(snapshotContent) == 0 {
		snapshotContent = []byte("{}")
	}
	// status is 'active' (window already open / no schedule) or 'scheduled'
	// (scheduled_start in the future). Display window is still enforced at read time.
	query := `
		UPDATE announcements
		SET status = $5, published_at = NOW(), snapshot_content = $4, updated_by = $3, updated_at = NOW()
		WHERE rt_id = $1 AND id = $2
	`
	res, err := r.db(tx).ExecContext(ctx, query, rtID, id, userID, snapshotContent, status)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) ArchiveAnnouncement(ctx context.Context, tx *sql.Tx, rtID, id, userID string) error {
	query := `
		UPDATE announcements
		SET status = 'archived', updated_by = $3, updated_at = NOW()
		WHERE rt_id = $1 AND id = $2
	`
	res, err := r.db(tx).ExecContext(ctx, query, rtID, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteAnnouncement(ctx context.Context, tx *sql.Tx, rtID, id string) error {
	query := "DELETE FROM announcements WHERE rt_id = $1 AND id = $2"
	res, err := r.db(tx).ExecContext(ctx, query, rtID, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// -----------------------------------------------------------------------------
// Banner Templates
// -----------------------------------------------------------------------------

func (r *Repository) ListBannerTemplates(ctx context.Context, tx *sql.Tx, rtID string) ([]BannerTemplate, error) {
	query := `
		SELECT id, rt_id, name, category, title_template, body_template, image_url,
		       default_priority, is_system, is_active, created_at, updated_at
		FROM banner_templates
		WHERE (rt_id = $1 OR rt_id IS NULL) AND is_active = true
		ORDER BY is_system DESC, default_priority DESC, name ASC
	`
	rows, err := r.db(tx).QueryContext(ctx, query, rtID)
	if err != nil {
		return nil, fmt.Errorf("list banner templates: %w", err)
	}
	defer rows.Close()

	var list []BannerTemplate
	for rows.Next() {
		var bt BannerTemplate
		var rtIDVal, img sql.NullString
		if err := rows.Scan(
			&bt.ID, &rtIDVal, &bt.Name, &bt.Category, &bt.TitleTemplate, &bt.BodyTemplate, &img,
			&bt.DefaultPriority, &bt.IsSystem, &bt.IsActive, &bt.CreatedAt, &bt.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan banner template: %w", err)
		}
		if rtIDVal.Valid {
			bt.RTID = &rtIDVal.String
		}
		if img.Valid {
			bt.ImageURL = &img.String
		}
		list = append(list, bt)
	}
	return list, nil
}

func (r *Repository) GetBannerTemplateByID(ctx context.Context, tx *sql.Tx, rtID, id string) (*BannerTemplate, error) {
	query := `
		SELECT id, rt_id, name, category, title_template, body_template, image_url,
		       default_priority, is_system, is_active, created_at, updated_at
		FROM banner_templates
		WHERE id = $2 AND (rt_id = $1 OR rt_id IS NULL) AND is_active = true
	`
	var bt BannerTemplate
	var rtIDVal, img sql.NullString
	err := r.db(tx).QueryRowContext(ctx, query, rtID, id).Scan(
		&bt.ID, &rtIDVal, &bt.Name, &bt.Category, &bt.TitleTemplate, &bt.BodyTemplate, &img,
		&bt.DefaultPriority, &bt.IsSystem, &bt.IsActive, &bt.CreatedAt, &bt.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get banner template: %w", err)
	}
	if rtIDVal.Valid {
		bt.RTID = &rtIDVal.String
	}
	if img.Valid {
		bt.ImageURL = &img.String
	}
	return &bt, nil
}

// -----------------------------------------------------------------------------
// Banners
// -----------------------------------------------------------------------------

func (r *Repository) ListActiveBanners(ctx context.Context, tx *sql.Tx, rtID string) ([]Banner, error) {
	query := `
		SELECT b.id, b.rt_id, b.source_type, b.source_id, b.template_id, b.title, b.body,
		       COALESCE(b.image_url, bt.image_url) AS image_url,
		       b.deep_link, b.start_at, b.end_at, b.priority, b.display_order, b.is_pinned,
		       b.is_active, b.snapshot_content, b.created_by, b.created_at, b.updated_at
		FROM banners b
		LEFT JOIN banner_templates bt ON bt.id = b.template_id
		WHERE b.rt_id = $1 AND b.is_active = true
		  AND (b.start_at IS NULL OR b.start_at <= NOW())
		  AND (b.end_at IS NULL OR b.end_at > NOW())
		ORDER BY b.is_pinned DESC, b.priority DESC, b.display_order ASC, b.created_at DESC
	`
	rows, err := r.db(tx).QueryContext(ctx, query, rtID)
	if err != nil {
		return nil, fmt.Errorf("list active banners: %w", err)
	}
	defer rows.Close()

	var list []Banner
	for rows.Next() {
		var b Banner
		var srcID, tmplID, body, img, link, cBy sql.NullString
		var startAt, endAt sql.NullTime
		var snapshotBytes []byte
		if err := rows.Scan(
			&b.ID, &b.RTID, &b.SourceType, &srcID, &tmplID, &b.Title, &body, &img, &link,
			&startAt, &endAt, &b.Priority, &b.DisplayOrder, &b.IsPinned, &b.IsActive, &snapshotBytes,
			&cBy, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan banner: %w", err)
		}
		if srcID.Valid {
			b.SourceID = &srcID.String
		}
		if tmplID.Valid {
			b.TemplateID = &tmplID.String
		}
		if body.Valid {
			b.Body = &body.String
		}
		if img.Valid {
			b.ImageURL = &img.String
		}
		if link.Valid {
			b.DeepLink = &link.String
		}
		if startAt.Valid {
			b.StartAt = &startAt.Time
		}
		if endAt.Valid {
			b.EndAt = &endAt.Time
		}
		if cBy.Valid {
			b.CreatedBy = &cBy.String
		}
		if len(snapshotBytes) > 0 {
			b.SnapshotContent = snapshotBytes
		}
		list = append(list, b)
	}
	return list, nil
}

func (r *Repository) UpsertBanner(ctx context.Context, tx *sql.Tx, b *Banner) error {
	snapshotJSON := b.SnapshotContent
	if len(snapshotJSON) == 0 {
		snapshotJSON = []byte("{}")
	}

	// Atomic upsert backed by the partial unique index
	// banners_one_per_source_idx (migration 069). Re-publishing updates the
	// existing banner row (including re-activation) instead of duplicating it.
	query := `
		INSERT INTO banners (
			rt_id, source_type, source_id, template_id, title, body, image_url,
			deep_link, start_at, end_at, priority, display_order, is_pinned, is_active,
			snapshot_content, created_by, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW()
		)
		ON CONFLICT (rt_id, source_type, source_id) WHERE source_id IS NOT NULL AND is_active = true DO UPDATE
		SET template_id = EXCLUDED.template_id, title = EXCLUDED.title, body = EXCLUDED.body,
		    image_url = EXCLUDED.image_url, deep_link = EXCLUDED.deep_link,
		    start_at = EXCLUDED.start_at, end_at = EXCLUDED.end_at,
		    priority = EXCLUDED.priority, display_order = EXCLUDED.display_order,
		    is_pinned = EXCLUDED.is_pinned, is_active = true,
		    snapshot_content = EXCLUDED.snapshot_content,
		    updated_at = NOW()
		RETURNING id, created_at, updated_at
	`
	return r.db(tx).QueryRowContext(ctx, query,
		b.RTID, b.SourceType, b.SourceID, b.TemplateID, b.Title, b.Body, b.ImageURL,
		b.DeepLink, b.StartAt, b.EndAt, b.Priority, b.DisplayOrder, b.IsPinned, b.IsActive,
		snapshotJSON, b.CreatedBy,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
}

func (r *Repository) DeleteBannerBySource(ctx context.Context, tx *sql.Tx, rtID, sourceType, sourceID string) error {
	query := "DELETE FROM banners WHERE rt_id = $1 AND source_type = $2 AND source_id = $3"
	_, err := r.db(tx).ExecContext(ctx, query, rtID, sourceType, sourceID)
	return err
}

// DeactivateBannerBySource hides banners for a source without deleting them,
// preserving publish history. Used when a published source is cancelled or
// archived: the banner leaves the active set but the row (and its snapshot)
// remains for audit.
func (r *Repository) DeactivateBannerBySource(ctx context.Context, tx *sql.Tx, rtID, sourceType, sourceID string) error {
	query := "UPDATE banners SET is_active = false, updated_at = NOW() WHERE rt_id = $1 AND source_type = $2 AND source_id = $3"
	_, err := r.db(tx).ExecContext(ctx, query, rtID, sourceType, sourceID)
	return err
}

// ImageAccessAllowed reports whether imageKey may be served to a caller in
// rtID. canManageActivities/canManageAnnouncements grant draft-content
// visibility; without them, only published/active in-window references count.
// Unreferenced keys are never allowed. Stored image references are accepted
// in both "community/<key>" and "/api/v1/community/images/<key>" form, since
// the upload endpoint stores the full API path.
func (r *Repository) ImageAccessAllowed(ctx context.Context, tx *sql.Tx, rtID, imageKey string, canManageActivities, canManageAnnouncements bool) (bool, error) {
	urlForm := "/api/v1/community/images/" + imageKey
	var allowed bool
	err := r.db(tx).QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM activities
			WHERE rt_id = $1 AND (poster_url = $2 OR poster_url = $3)
			  AND ($4::bool OR publication_status = 'published')
			UNION
			SELECT 1 FROM announcements
			WHERE rt_id = $1 AND (image_url = $2 OR image_url = $3)
			  AND ($5::bool OR
			        (status='active' AND (scheduled_start IS NULL OR scheduled_start <= NOW()) AND (scheduled_end IS NULL OR scheduled_end > NOW()))
			     OR status IN ('archived', 'expired'))
			UNION
			SELECT 1 FROM banners
			WHERE rt_id = $1 AND (image_url = $2 OR image_url = $3)
			  AND ($4::bool OR $5::bool OR (is_active = true
			    AND (start_at IS NULL OR start_at <= NOW())
			    AND (end_at IS NULL OR end_at > NOW())))
			UNION
			SELECT 1 FROM banner_templates
			WHERE (rt_id = $1 OR rt_id IS NULL) AND (image_url = $2 OR image_url = $3)
		)
	`, rtID, imageKey, urlForm, canManageActivities, canManageAnnouncements).Scan(&allowed)
	return allowed, err
}

// -----------------------------------------------------------------------------
// Organization Structure & RT Info
// -----------------------------------------------------------------------------

func (r *Repository) GetOrganizationStructure(ctx context.Context, tx *sql.Tx, rtID string, canViewContact bool) (*OrganizationStructure, error) {
	// 1. Fetch RT metadata
	var rtName, rtNum string
	var rwNum int
	err := r.db(tx).QueryRowContext(ctx, `
		SELECT name, rt, rw
		FROM rts
		WHERE id = $1
	`, rtID).Scan(&rtName, &rtNum, &rwNum)
	if err != nil {
		return nil, fmt.Errorf("get rt info: %w", err)
	}

	// 2. Query active positions and members
	query := `
		SELECT m.id, u.id, u.full_name, m.jabatan,
		       CASE WHEN $2 = true THEN u.phone ELSE NULL END AS phone,
		       CASE WHEN $2 = true THEN u.email ELSE NULL END AS email
		FROM user_rt_memberships m
		JOIN users u ON m.user_id = u.id
		WHERE m.rt_id = $1 AND m.is_active = true AND u.is_active = true AND m.jabatan IS NOT NULL
		ORDER BY m.created_at ASC
	`
	rows, err := r.db(tx).QueryContext(ctx, query, rtID, canViewContact)
	if err != nil {
		return nil, fmt.Errorf("get org members: %w", err)
	}
	defer rows.Close()

	membersByJabatan := make(map[string][]OrgMember)
	for rows.Next() {
		var memID, userID, fullName, jab string
		var phone, email sql.NullString
		if err := rows.Scan(&memID, &userID, &fullName, &jab, &phone, &email); err != nil {
			return nil, fmt.Errorf("scan org member: %w", err)
		}
		om := OrgMember{
			UserID:       userID,
			MembershipID: memID,
			FullName:     fullName,
			Jabatan:      jab,
		}
		if phone.Valid && phone.String != "" {
			om.Phone = &phone.String
		}
		membersByJabatan[jab] = append(membersByJabatan[jab], om)
	}

	// 3. Assemble standard hierarchy nodes
	nodesConfig := []struct {
		Position      string
		PositionLabel string
		Level         int
		Parent        string
	}{
		{Position: "ketua", PositionLabel: "Ketua RT", Level: 1, Parent: ""},
		{Position: "wakil_ketua", PositionLabel: "Wakil Ketua", Level: 2, Parent: "ketua"},
		{Position: "sekretaris", PositionLabel: "Sekretaris", Level: 2, Parent: "ketua"},
		{Position: "bendahara", PositionLabel: "Bendahara", Level: 3, Parent: "sekretaris"},
		{Position: "keamanan", PositionLabel: "Keamanan", Level: 4, Parent: "ketua"},
		{Position: "kebersihan", PositionLabel: "Kebersihan", Level: 4, Parent: "ketua"},
		{Position: "sosial", PositionLabel: "Seksi Sosial", Level: 4, Parent: "ketua"},
		{Position: "operasional", PositionLabel: "Operasional", Level: 4, Parent: "ketua"},
	}

	var nodes []OrgNode
	for _, cfg := range nodesConfig {
		members := membersByJabatan[cfg.Position]
		if members == nil {
			members = []OrgMember{}
		}
		nodes = append(nodes, OrgNode{
			Position:       cfg.Position,
			PositionLabel:  cfg.PositionLabel,
			Level:          cfg.Level,
			ParentPosition: cfg.Parent,
			Members:        members,
		})
	}

	return &OrganizationStructure{
		RTID:     rtID,
		RTName:   rtName,
		RTNumber: rtNum,
		RWNumber: fmt.Sprintf("%03d", rwNum),
		Nodes:    nodes,
	}, nil
}

// -----------------------------------------------------------------------------
// Billing / Iuran Reminder Check
// -----------------------------------------------------------------------------

func (r *Repository) HasUnpaidObligatoryBills(ctx context.Context, tx *sql.Tx, rtID string) (bool, error) {
	var hasUnpaid bool
	err := r.db(tx).QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM bills b
			JOIN household_occupancies ho ON b.household_occupancy_id = ho.id
			JOIN households h ON ho.household_id = h.id AND h.is_active = true
			WHERE b.rt_id = $1 AND b.is_active = true AND b.is_obligatory = true
			  AND b.status IN ('unpaid', 'partial')
			  AND ho.end_date IS NULL
		)
	`, rtID).Scan(&hasUnpaid)
	return hasUnpaid, err
}
