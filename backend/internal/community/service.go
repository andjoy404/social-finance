package community

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"social-finance/internal/storage"
)

var (
	ErrValidation       = errors.New("validation error")
	ErrNotFound         = errors.New("resource not found")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrInvalidDeepLink  = errors.New("invalid deep link: must be an internal allowed path")
	ErrTemplateNotFound = errors.New("template not found")
)

var allowedDeepLinkPrefixes = []string{
	"/keuangan",
	"/iuran",
	"/kas",
	"/kegiatan",
	"/warga",
	"/notifikasi",
	"/pengumuman",
	"/profil",
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// OnDutyReaderFunc injects live on-duty security officers from the security module without cyclic imports.
type OnDutyReaderFunc func(ctx context.Context, rtID string) ([]SecurityOfficer, error)

// ActivityNotifier is implemented by the notification module to fan activity
// lifecycle events (created/updated/reminder/cancelled/completed) and to cancel
// pending activity reminders when the activity is cancelled or completes.
type ActivityNotifier interface {
	NotifyActivityEvent(ctx context.Context, p ActivityNotificationPayload, eventType string) error
	CancelActivityPendingSchedules(ctx context.Context, rtID, activityID string) error
}

// ActivityNotificationPayload is the community package's view of an activity for notifying.
type ActivityNotificationPayload struct {
	ID                string
	RTID              string
	Title             string
	Description       *string
	Location          *string
	Organizer         *string
	StartsAt          time.Time
	EndsAt            *time.Time
	Status            string
	PublicationStatus string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Service provides business logic for community features.
type Service struct {
	repo             *Repository
	storage          storage.Service
	onDutyReader     OnDutyReaderFunc
	activityNotifier ActivityNotifier
}

// NewService creates a new community Service.
func NewService(repo *Repository, store storage.Service) *Service {
	return &Service{
		repo:    repo,
		storage: store,
	}
}

// WithOnDutyReader attaches a security module reader for live on-duty officer status on Beranda.
func (s *Service) WithOnDutyReader(fn OnDutyReaderFunc) {
	s.onDutyReader = fn
}

// WithActivityNotifier attaches the notification module notifier for activity lifecycle events.
func (s *Service) WithActivityNotifier(n ActivityNotifier) {
	s.activityNotifier = n
}

// activityNotificationPayload maps a domain Activity into the notifier payload.
func activityNotificationPayload(a *Activity) ActivityNotificationPayload {
	out := ActivityNotificationPayload{
		ID:                a.ID,
		RTID:              a.RTID,
		Title:             a.Title,
		Description:       a.Description,
		Location:          a.Location,
		Organizer:         a.Organizer,
		StartsAt:          a.StartsAt,
		Status:            a.Status,
		PublicationStatus: a.PublicationStatus,
		CreatedAt:         a.CreatedAt,
		UpdatedAt:         a.UpdatedAt,
	}
	if a.EndsAt != nil {
		out.EndsAt = a.EndsAt
	}
	return out
}

// notifyActivityEvent reports an activity lifecycle event to the notification module.
// Best-effort: delivery failure must not roll back the activity transaction.
func (s *Service) notifyActivityEvent(ctx context.Context, a *Activity, eventType string) {
	if s.activityNotifier == nil || a == nil {
		return
	}
	if err := s.activityNotifier.NotifyActivityEvent(ctx, activityNotificationPayload(a), eventType); err != nil {
		slog.Warn("activity event notification failed", "activity_id", a.ID, "event_type", eventType, "error", err)
	}
}

// cancelActivitySchedules requests cancellation of pending one-shot schedules for an activity.
func (s *Service) cancelActivitySchedules(ctx context.Context, a *Activity) {
	if s.activityNotifier == nil || a == nil {
		return
	}
	if err := s.activityNotifier.CancelActivityPendingSchedules(ctx, a.RTID, a.ID); err != nil {
		slog.Warn("activity schedule cancellation failed", "activity_id", a.ID, "error", err)
	}
}

// ValidateDeepLink ensures that a deep link is safe and internal.
func ValidateDeepLink(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	// Must start with '/'
	if !strings.HasPrefix(trimmed, "/") {
		return "", ErrInvalidDeepLink
	}

	// Disallow protocol, javascript:, backslashes, data:, etc.
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "://") || strings.HasPrefix(lower, "javascript:") || strings.Contains(trimmed, "\\") {
		return "", ErrInvalidDeepLink
	}

	u, err := url.Parse(trimmed)
	if err != nil || u.Host != "" || u.Scheme != "" {
		return "", ErrInvalidDeepLink
	}

	// Check against allowlist prefixes
	path := u.Path
	matched := false
	for _, prefix := range allowedDeepLinkPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			matched = true
			break
		}
	}
	if !matched {
		return "", fmt.Errorf("%w: path %s is not permitted", ErrInvalidDeepLink, path)
	}

	return trimmed, nil
}

// -----------------------------------------------------------------------------
// Activities Service
// -----------------------------------------------------------------------------

func (s *Service) ListActivities(ctx context.Context, rtID, pubStatus, status string, isOfficer bool, limit, offset int) ([]Activity, int, error) {
	// If viewer is ordinary warga, force pubStatus to 'published'
	if !isOfficer {
		pubStatus = "published"
	}
	activities, total, err := s.repo.ListActivities(ctx, nil, rtID, pubStatus, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	// Compute dynamic status based on time
	now := time.Now()
	for i := range activities {
		activities[i].Status = activityDynamicStatus(&activities[i], now)
	}

	return activities, total, nil
}

// activityDynamicStatus computes the PUBLIC execution status of an activity
// at time now. Cancellation is explicit and takes priority; everything else
// derives from starts_at/ends_at:
//
//	upcoming  — starts_at in the future
//	ongoing   — starts_at <= now < ends_at (or open-ended)
//	completed — ends_at <= now (or starts_at <= now and no ends_at and ...
//	            handled by ongoing), i.e. ended in the past
//	cancelled — explicit stored cancel
func activityDynamicStatus(a *Activity, now time.Time) string {
	if a.Status == "cancelled" {
		return "cancelled"
	}
	if a.EndsAt != nil && !a.EndsAt.After(now) {
		return "completed"
	}
	if a.StartsAt.After(now) {
		return "upcoming"
	}
	return "ongoing"
}

func (s *Service) GetActivityByID(ctx context.Context, rtID, id string, isOfficer bool) (*Activity, error) {
	a, err := s.repo.GetActivityByID(ctx, nil, rtID, id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrNotFound
	}
	if !isOfficer && a.PublicationStatus != "published" {
		return nil, ErrNotFound
	}

	a.Status = activityDynamicStatus(a, time.Now())

	return a, nil
}

func (s *Service) CreateActivity(ctx context.Context, rtID, userID string, req CreateActivityRequest) (*Activity, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		// Try date format YYYY-MM-DD
		t, errDate := time.Parse("2006-01-02", req.StartsAt)
		if errDate != nil {
			return nil, fmt.Errorf("%w: invalid starts_at format (RFC3339 required)", ErrValidation)
		}
		startsAt = t
	}

	var endsAt *time.Time
	if req.EndsAt != nil && strings.TrimSpace(*req.EndsAt) != "" {
		t, err := time.Parse(time.RFC3339, *req.EndsAt)
		if err != nil {
			tDate, errDate := time.Parse("2006-01-02", *req.EndsAt)
			if errDate != nil {
				return nil, fmt.Errorf("%w: invalid ends_at format", ErrValidation)
			}
			t = tDate
		}
		if t.Before(startsAt) {
			return nil, fmt.Errorf("%w: ends_at cannot be before starts_at", ErrValidation)
		}
		endsAt = &t
	}

	pubStatus := "draft"
	if req.PublishNow {
		pubStatus = "published"
	}

	a := Activity{
		RTID:              rtID,
		Title:             strings.TrimSpace(req.Title),
		Description:       req.Description,
		PosterURL:         req.PosterURL,
		StartsAt:          startsAt,
		EndsAt:            endsAt,
		Location:          req.Location,
		Organizer:         req.Organizer,
		Status:            "upcoming",
		PublicationStatus: pubStatus,
		ShowOnBanner:      req.ShowOnBanner,
		CreatedBy:         &userID,
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.repo.CreateActivity(ctx, tx, &a); err != nil {
		return nil, err
	}

	// If published with show_on_banner, sync banner in the same transaction.
	if a.PublicationStatus == "published" && a.ShowOnBanner {
		if err := s.syncActivityBanner(ctx, tx, &a); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if a.PublicationStatus == "published" {
		s.notifyActivityEvent(ctx, &a, "activity_created")
	}
	return &a, nil
}

func (s *Service) UpdateActivity(ctx context.Context, rtID, id, userID string, req UpdateActivityRequest) (*Activity, error) {
	existing, err := s.repo.GetActivityByID(ctx, nil, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrNotFound
	}

	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		t, errDate := time.Parse("2006-01-02", req.StartsAt)
		if errDate != nil {
			return nil, fmt.Errorf("%w: invalid starts_at format", ErrValidation)
		}
		startsAt = t
	}

	var endsAt *time.Time
	if req.EndsAt != nil && strings.TrimSpace(*req.EndsAt) != "" {
		t, err := time.Parse(time.RFC3339, *req.EndsAt)
		if err != nil {
			tDate, errDate := time.Parse("2006-01-02", *req.EndsAt)
			if errDate != nil {
				return nil, fmt.Errorf("%w: invalid ends_at format", ErrValidation)
			}
			t = tDate
		}
		if t.Before(startsAt) {
			return nil, fmt.Errorf("%w: ends_at cannot be before starts_at", ErrValidation)
		}
		endsAt = &t
	}

	existing.Title = strings.TrimSpace(req.Title)
	existing.Description = req.Description
	existing.PosterURL = req.PosterURL
	existing.StartsAt = startsAt
	existing.EndsAt = endsAt
	existing.Location = req.Location
	existing.Organizer = req.Organizer
	existing.ShowOnBanner = req.ShowOnBanner
	existing.UpdatedBy = &userID

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.repo.UpdateActivity(ctx, tx, existing); err != nil {
		return nil, err
	}

	// NOTE: the published banner is an immutable snapshot taken at publish
	// time. Editing a published activity must NOT mutate the live banner;
	// the banner only changes via an explicit republish action.
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if existing.PublicationStatus == "published" {
		s.notifyActivityEvent(ctx, existing, "activity_updated")
	}
	return existing, nil
}

func (s *Service) PublishActivity(ctx context.Context, rtID, id, userID string) error {
	a, err := s.repo.GetActivityByID(ctx, nil, rtID, id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrNotFound
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.PublishActivity(ctx, tx, rtID, id, userID); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}

	a.PublicationStatus = "published"
	if a.ShowOnBanner {
		if err := s.syncActivityBanner(ctx, tx, a); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	s.notifyActivityEvent(ctx, a, "activity_created")
	return nil
}

func (s *Service) CancelActivity(ctx context.Context, rtID, id, userID string) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.CancelActivity(ctx, tx, rtID, id, userID); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	// Fetch payload BEFORE commit so title/details are available for the notification.
	after, err := s.repo.GetActivityByID(ctx, tx, rtID, id)
	if err != nil {
		after = nil
	}

	// Hide the banner but keep the row (publish history) intact.
	if err := s.repo.DeactivateBannerBySource(ctx, tx, rtID, "activity", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if after != nil {
		after.Status = "cancelled"
		s.notifyActivityEvent(ctx, after, "activity_cancelled")
		s.cancelActivitySchedules(ctx, after)
	}
	return nil
}

func (s *Service) DeleteActivity(ctx context.Context, rtID, id string) error {
	existing, err := s.repo.GetActivityByID(ctx, nil, rtID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	// Only drafts may be hard-deleted. Published activities must be
	// cancelled instead so published history is preserved.
	if existing.PublicationStatus != "draft" {
		return fmt.Errorf("%w: only draft activities can be deleted; cancel published activities instead", ErrValidation)
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.DeleteBannerBySource(ctx, tx, rtID, "activity", id); err != nil {
		return err
	}
	if err := s.repo.DeleteActivity(ctx, tx, rtID, id); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	return tx.Commit()
}

func (s *Service) syncActivityBanner(ctx context.Context, tx *sql.Tx, a *Activity) error {
	snapshot, _ := json.Marshal(map[string]any{
		"activity_id": a.ID,
		"title":       a.Title,
		"description": a.Description,
		"starts_at":   a.StartsAt,
		"ends_at":     a.EndsAt,
		"location":    a.Location,
		"organizer":   a.Organizer,
		"poster_url":  a.PosterURL,
	})

	link := fmt.Sprintf("/kegiatan/%s", a.ID)
	b := Banner{
		RTID:            a.RTID,
		SourceType:      "activity",
		SourceID:        &a.ID,
		Title:           a.Title,
		Body:            a.Description,
		ImageURL:        a.PosterURL,
		DeepLink:        &link,
		StartAt:         &a.StartsAt,
		EndAt:           a.EndsAt,
		Priority:        4,
		DisplayOrder:    0,
		IsPinned:        false,
		IsActive:        true,
		SnapshotContent: snapshot,
		CreatedBy:       a.CreatedBy,
	}
	return s.repo.UpsertBanner(ctx, tx, &b)
}

// -----------------------------------------------------------------------------
// Announcements Service
// -----------------------------------------------------------------------------

func (s *Service) ListAnnouncements(ctx context.Context, rtID, status string, canManage bool, limit, offset int) ([]Announcement, int, error) {
	forWarga := false
	if !canManage {
		forWarga = true
		if status == "history" || status == "archived" {
			status = "history"
		} else if status == "all" {
			status = "all"
		} else {
			status = "active"
		}
	}
	return s.repo.ListAnnouncements(ctx, nil, rtID, status, forWarga, limit, offset)
}

func (s *Service) GetAnnouncementByID(ctx context.Context, rtID, id string, canManage bool) (*Announcement, error) {
	an, err := s.repo.GetAnnouncementByID(ctx, nil, rtID, id)
	if err != nil {
		return nil, err
	}
	if an == nil {
		return nil, ErrNotFound
	}
	if !canManage && an.Status == "draft" {
		return nil, ErrNotFound
	}
	return an, nil
}

// announcementVisibleForWarga reports whether an announcement is inside its
// display window for warga. Visibility window: scheduled_start <= now <
// scheduled_end (open-ended when NULL). At exactly scheduled_end the
// announcement is already hidden.
func announcementVisibleForWarga(an *Announcement, now time.Time) bool {
	if an.Status != "active" {
		return false
	}
	if an.ScheduledStart != nil && an.ScheduledStart.After(now) {
		return false
	}
	if an.ScheduledEnd != nil && !an.ScheduledEnd.After(now) {
		return false
	}
	return true
}

// parseScheduleField parses an optional RFC3339 schedule field strictly.
// Empty means unset; invalid values are a validation error (never silently
// dropped to nil). RFC3339 with an explicit timezone is required.
func parseScheduleField(raw *string, field string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(*raw))
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be RFC3339 with timezone", ErrValidation, field)
	}
	return &t, nil
}

// validateTemplate ensures a referenced banner template exists, is active,
// and belongs to the caller's RT (or is a global system template).
func (s *Service) validateTemplate(ctx context.Context, tx *sql.Tx, rtID string, templateID *string) error {
	if templateID == nil || strings.TrimSpace(*templateID) == "" {
		return nil
	}
	id := strings.TrimSpace(*templateID)
	if !uuidPattern.MatchString(id) {
		return fmt.Errorf("%w: invalid template_id", ErrValidation)
	}
	tmpl, err := s.repo.GetBannerTemplateByID(ctx, tx, rtID, id)
	if err != nil {
		return err
	}
	if tmpl == nil {
		return ErrTemplateNotFound
	}
	return nil
}

func (s *Service) CreateAnnouncement(ctx context.Context, rtID, userID string, req CreateAnnouncementRequest) (*Announcement, error) {
	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if strings.TrimSpace(req.Body) == "" {
		return nil, fmt.Errorf("%w: body is required", ErrValidation)
	}

	var deepLink *string
	if req.DeepLink != nil && *req.DeepLink != "" {
		validated, err := ValidateDeepLink(*req.DeepLink)
		if err != nil {
			return nil, err
		}
		deepLink = &validated
	}

	var schedStart, schedEnd *time.Time
	schedStart, err := parseScheduleField(req.ScheduledStart, "scheduled_start")
	if err != nil {
		return nil, err
	}
	schedEnd, err = parseScheduleField(req.ScheduledEnd, "scheduled_end")
	if err != nil {
		return nil, err
	}
	if schedStart != nil && schedEnd != nil && !schedEnd.After(*schedStart) {
		return nil, fmt.Errorf("%w: scheduled_end must be after scheduled_start", ErrValidation)
	}

	priority := req.Priority
	if priority <= 0 {
		priority = 1
	}

	status := "draft"
	var pubAt *time.Time
	var snapshotBytes []byte
	if req.PublishNow {
		now := time.Now()
		pubAt = &now
		// A publish action with a future schedule start lands in
		// 'scheduled', never 'active': the display window is enforced at
		// read time, and the stored state must not claim active early.
		status = "active"
		if schedStart != nil && schedStart.After(now) {
			status = "scheduled"
		}
		snapshotBytes, _ = json.Marshal(map[string]any{
			"title":     req.Title,
			"body":      req.Body,
			"image_url": req.ImageURL,
			"deep_link": deepLink,
		})
	}

	an := Announcement{
		RTID:            rtID,
		Title:           strings.TrimSpace(req.Title),
		Body:            strings.TrimSpace(req.Body),
		ImageURL:        req.ImageURL,
		DeepLink:        deepLink,
		TemplateID:      req.TemplateID,
		ScheduledStart:  schedStart,
		ScheduledEnd:    schedEnd,
		Priority:        priority,
		DisplayOrder:    req.DisplayOrder,
		Status:          status,
		PublishedAt:     pubAt,
		SnapshotContent: snapshotBytes,
		CreatedBy:       &userID,
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.validateTemplate(ctx, tx, rtID, req.TemplateID); err != nil {
		return nil, err
	}

	if (an.ImageURL == nil || *an.ImageURL == "") && req.TemplateID != nil && *req.TemplateID != "" {
		if tmpl, err := s.repo.GetBannerTemplateByID(ctx, tx, rtID, *req.TemplateID); err == nil && tmpl != nil && tmpl.ImageURL != nil && *tmpl.ImageURL != "" {
			an.ImageURL = tmpl.ImageURL
		}
	}

	if err := s.repo.CreateAnnouncement(ctx, tx, &an); err != nil {
		return nil, err
	}

	if an.Status == "active" || an.Status == "scheduled" {
		if err := s.syncAnnouncementBanner(ctx, tx, &an); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &an, nil
}

func (s *Service) UpdateAnnouncement(ctx context.Context, rtID, id, userID string, req UpdateAnnouncementRequest) (*Announcement, error) {
	existing, err := s.repo.GetAnnouncementByID(ctx, nil, rtID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrNotFound
	}

	if strings.TrimSpace(req.Title) == "" {
		return nil, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if strings.TrimSpace(req.Body) == "" {
		return nil, fmt.Errorf("%w: body is required", ErrValidation)
	}

	var deepLink *string
	if req.DeepLink != nil && *req.DeepLink != "" {
		validated, err := ValidateDeepLink(*req.DeepLink)
		if err != nil {
			return nil, err
		}
		deepLink = &validated
	}

	schedStart, err := parseScheduleField(req.ScheduledStart, "scheduled_start")
	if err != nil {
		return nil, err
	}
	schedEnd, err := parseScheduleField(req.ScheduledEnd, "scheduled_end")
	if err != nil {
		return nil, err
	}
	if schedStart != nil && schedEnd != nil && !schedEnd.After(*schedStart) {
		return nil, fmt.Errorf("%w: scheduled_end must be after scheduled_start", ErrValidation)
	}

	priority := req.Priority
	if priority <= 0 {
		priority = 1
	}

	existing.Title = strings.TrimSpace(req.Title)
	existing.Body = strings.TrimSpace(req.Body)
	existing.ImageURL = req.ImageURL
	existing.DeepLink = deepLink
	existing.TemplateID = req.TemplateID
	existing.ScheduledStart = schedStart
	existing.ScheduledEnd = schedEnd
	existing.Priority = priority
	existing.DisplayOrder = req.DisplayOrder
	existing.UpdatedBy = &userID

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.validateTemplate(ctx, tx, rtID, req.TemplateID); err != nil {
		return nil, err
	}

	if (existing.ImageURL == nil || *existing.ImageURL == "") && req.TemplateID != nil && *req.TemplateID != "" {
		if tmpl, err := s.repo.GetBannerTemplateByID(ctx, tx, rtID, *req.TemplateID); err == nil && tmpl != nil && tmpl.ImageURL != nil && *tmpl.ImageURL != "" {
			existing.ImageURL = tmpl.ImageURL
		}
	}

	if err := s.repo.UpdateAnnouncement(ctx, tx, existing); err != nil {
		return nil, err
	}

	// NOTE: the published banner is an immutable snapshot taken at publish
	// time. Editing an active announcement must NOT mutate the live banner
	// (nor its own frozen snapshot); changes only take effect through an
	// explicit republish action that rewrites both snapshots atomically.
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return existing, nil
}

func (s *Service) PublishAnnouncement(ctx context.Context, rtID, id, userID string) error {
	an, err := s.repo.GetAnnouncementByID(ctx, nil, rtID, id)
	if err != nil {
		return err
	}
	if an == nil {
		return ErrNotFound
	}

	snapshot, _ := json.Marshal(map[string]any{
		"announcement_id": an.ID,
		"title":           an.Title,
		"body":            an.Body,
		"image_url":       an.ImageURL,
		"deep_link":       an.DeepLink,
		"published_at":    time.Now().Format(time.RFC3339),
	})

	// Explicit publish action: future schedule start lands in 'scheduled',
	// otherwise 'active'. Both snapshots (announcement + banner) are
	// rewritten atomically in the same transaction.
	status := "active"
	if an.ScheduledStart != nil && an.ScheduledStart.After(time.Now()) {
		status = "scheduled"
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.PublishAnnouncement(ctx, tx, rtID, id, userID, status, snapshot); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}

	an.Status = status
	an.SnapshotContent = snapshot
	if err := s.syncAnnouncementBanner(ctx, tx, an); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ArchiveAnnouncement(ctx context.Context, rtID, id, userID string) error {
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.ArchiveAnnouncement(ctx, tx, rtID, id, userID); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	// Hide the banner but keep the row (publish history) intact.
	// Permanent rows publish under 'iuran_reminder' — deactivate both slots.
	if perm, errP := s.repo.IsPermanentAnnouncement(ctx, tx, rtID, id); errP == nil && perm {
		_ = s.repo.DeactivateBannerBySource(ctx, tx, rtID, "iuran_reminder", id)
	} else if err := s.repo.DeactivateBannerBySource(ctx, tx, rtID, "announcement", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) DeleteAnnouncement(ctx context.Context, rtID, id string) error {
	existing, err := s.repo.GetAnnouncementByID(ctx, nil, rtID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrNotFound
	}
	// The permanent system default announcement is not deletable — it is part
	// of every RT's managed experience and can only be toggled active/archived.
	if existing.IsPermanent {
		return fmt.Errorf("%w: pengumuman sistem default bersifat permanen dan tidak dapat dihapus", ErrValidation)
	}
	// Only drafts may be hard-deleted. Active/scheduled announcements must be
	// archived instead so published history is preserved. Archived content
	// likewise requires an explicit publish action to come back (never
	// automatically reactivated).
	if existing.Status != "draft" {
		return fmt.Errorf("%w: only draft announcements can be deleted; archive published announcements instead", ErrValidation)
	}

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.DeleteBannerBySource(ctx, tx, rtID, "announcement", id); err != nil {
		return err
	}
	if err := s.repo.DeleteAnnouncement(ctx, tx, rtID, id); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	return tx.Commit()
}

func (s *Service) syncAnnouncementBanner(ctx context.Context, tx *sql.Tx, an *Announcement) error {
	imgURL := an.ImageURL
	if (imgURL == nil || *imgURL == "") && an.TemplateID != nil && *an.TemplateID != "" {
		if tmpl, err := s.repo.GetBannerTemplateByID(ctx, tx, an.RTID, *an.TemplateID); err == nil && tmpl != nil && tmpl.ImageURL != nil {
			imgURL = tmpl.ImageURL
		}
	}

	sourceType := "announcement"
	if an.IsPermanent {
		// Permanent reminder flows through the iuran_reminder slot so it shares
		// dedup semantics with banners generated for unpaid obligatory bills.
		sourceType = "iuran_reminder"
	}

	b := Banner{
		RTID:            an.RTID,
		SourceType:      sourceType,
		SourceID:        &an.ID,
		TemplateID:      an.TemplateID,
		Title:           an.Title,
		Body:            &an.Body,
		ImageURL:        imgURL,
		DeepLink:        an.DeepLink,
		StartAt:         an.ScheduledStart,
		EndAt:           an.ScheduledEnd,
		Priority:        an.Priority,
		DisplayOrder:    an.DisplayOrder,
		IsPinned:        an.Priority >= 10,
		IsActive:        true,
		SnapshotContent: an.SnapshotContent,
		CreatedBy:       an.CreatedBy,
	}
	return s.repo.UpsertBanner(ctx, tx, &b)
}

// -----------------------------------------------------------------------------
// Banner Templates Service
// -----------------------------------------------------------------------------

func (s *Service) ListBannerTemplates(ctx context.Context, rtID string) ([]BannerTemplate, error) {
	return s.repo.ListBannerTemplates(ctx, nil, rtID)
}

func (s *Service) GetBannerTemplateByID(ctx context.Context, rtID, id string) (*BannerTemplate, error) {
	bt, err := s.repo.GetBannerTemplateByID(ctx, nil, rtID, id)
	if err != nil {
		return nil, err
	}
	if bt == nil {
		return nil, ErrNotFound
	}
	return bt, nil
}

// -----------------------------------------------------------------------------
// Banners & Home Dashboard Service
// -----------------------------------------------------------------------------

func (s *Service) ListActiveBanners(ctx context.Context, rtID string) ([]Banner, error) {
	banners, err := s.repo.ListActiveBanners(ctx, nil, rtID)
	if err != nil {
		return nil, err
	}

	// Automatic IPL/Iuran Reminder Banner check.
	// The reminder is RT-broadcast and deliberately generic: it contains no
	// names, household counts, amounts, or any per-resident financial data.
	// It is merged into the banner list with the same ordering as DB banners
	// (pinned, then priority) instead of being prepended, so it stays
	// findable without permanently shadowing activity/announcement banners.
	//
	// When a permanent announcement exists for the RT (published or archived),
	// skip synthesized banner injection — the permanent announcement owns the
	// iuran_reminder slot through the standard publish flow.
	hasPermanent, err := s.repo.PermanentAnnouncementExists(ctx, nil, rtID)
	if err != nil {
		hasPermanent = false
	}

	hasUnpaid, err := s.repo.HasUnpaidObligatoryBills(ctx, nil, rtID)
	if err == nil && hasUnpaid && !hasPermanent {
		hasReminder := false
		for _, b := range banners {
			if b.SourceType == "iuran_reminder" {
				hasReminder = true
				break
			}
		}

		if !hasReminder {
			reminderBody := "Jangan lupa melakukan pembayaran Iuran/IPL sesuai periode berjalan melalui menu Keuangan."
			reminderLink := "/keuangan"
			reminderBanner := Banner{
				ID:           "system-iuran-reminder",
				RTID:         rtID,
				SourceType:   "iuran_reminder",
				Title:        "Pengingat Pembayaran Iuran Warga",
				Body:         &reminderBody,
				DeepLink:     &reminderLink,
				Priority:     3,
				DisplayOrder: 0,
				IsPinned:     false,
				IsActive:     true,
			}
			banners = append([]Banner{reminderBanner}, banners...)
			sort.SliceStable(banners, func(i, j int) bool {
				if banners[i].IsPinned != banners[j].IsPinned {
					return banners[i].IsPinned
				}
				if banners[i].Priority != banners[j].Priority {
					return banners[i].Priority > banners[j].Priority
				}
				if banners[i].DisplayOrder != banners[j].DisplayOrder {
					return banners[i].DisplayOrder < banners[j].DisplayOrder
				}
				return banners[i].CreatedAt.After(banners[j].CreatedAt)
			})
		}
	}

	return banners, nil
}

func (s *Service) GetHomeDashboard(ctx context.Context, rtID, userFullName, userRole, userJabatan string, canViewContact bool) (*HomeDashboardData, error) {
	// 1. Fetch active banners (includes safe Iuran reminder)
	banners, err := s.ListActiveBanners(ctx, rtID)
	if err != nil {
		return nil, fmt.Errorf("fetch banners: %w", err)
	}

	// 2. Fetch upcoming activities (limit 5)
	activities, _, err := s.ListActivities(ctx, rtID, "published", "", false, 5, 0)
	if err != nil {
		activities = []Activity{}
	}

	// Filter out cancelled or completed
	var upcoming []Activity
	for _, a := range activities {
		if a.Status == "upcoming" || a.Status == "ongoing" {
			upcoming = append(upcoming, a)
		}
	}

	// 3. Fetch organization structure
	org, err := s.repo.GetOrganizationStructure(ctx, nil, rtID, canViewContact)
	if err != nil {
		return nil, fmt.Errorf("fetch org structure: %w", err)
	}

	// 4. Time-based greeting
	loc, _ := time.LoadLocation("Asia/Jakarta")
	if loc == nil {
		loc = time.Local
	}
	hour := time.Now().In(loc).Hour()
	var greeting string
	switch {
	case hour >= 4 && hour < 11:
		greeting = "Selamat Pagi"
	case hour >= 11 && hour < 15:
		greeting = "Selamat Siang"
	case hour >= 15 && hour < 18:
		greeting = "Selamat Sore"
	default:
		greeting = "Selamat Malam"
	}

	// 5. Security on-duty info (officers actually on duty now; backed by Security Attendance module)
	securityInfo := SecurityOnDutyInfo{
		IsAvailable: false,
		Message:     "Tidak ada petugas keamanan yang sedang bertugas.",
		Officers:    []SecurityOfficer{},
	}
	if s.onDutyReader != nil {
		if officers, err := s.onDutyReader(ctx, rtID); err == nil && len(officers) > 0 {
			// Officers who are actually on duty are shown as an emergency
			// hotline: their phone number is exposed to every authenticated
			// member of the RT. Callers need not hold
			// organization.contact.read (that permission only governs the
			// personal data of the organization structure, not the duty hotline).
			securityInfo.IsAvailable = true
			securityInfo.Message = "Petugas keamanan sedang bertugas."
			securityInfo.Officers = officers
		}
	}

	return &HomeDashboardData{
		Greeting:           greeting,
		UserFullName:       userFullName,
		UserRole:           userRole,
		UserJabatan:        userJabatan,
		RTName:             org.RTName,
		RTNumber:           org.RTNumber,
		RWNumber:           org.RWNumber,
		Banners:            banners,
		UpcomingActivities: upcoming,
		Organization:       *org,
		SecurityOnDuty:     securityInfo,
	}, nil
}
