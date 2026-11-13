package community

import (
	"encoding/json"
	"regexp"
	"time"
)

const TestCommunityCode = "test-community"

var communityCodePattern = regexp.MustCompile(`^[a-z0-9]+([a-z0-9-]*[a-z0-9])?$`)

func ValidCode(code string) bool {
	return len(code) >= 1 && len(code) <= 63 && communityCodePattern.MatchString(code)
}

type Community struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// Activity represents a community event/activity in the RT.
type Activity struct {
	ID                string     `json:"id"`
	RTID              string     `json:"rt_id"`
	Title             string     `json:"title"`
	Description       *string    `json:"description,omitempty"`
	PosterURL         *string    `json:"poster_url,omitempty"`
	StartsAt          time.Time  `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at,omitempty"`
	Location          *string    `json:"location,omitempty"`
	Organizer         *string    `json:"organizer,omitempty"`
	Status            string     `json:"status"`             // 'upcoming', 'ongoing', 'completed', 'cancelled'
	PublicationStatus string     `json:"publication_status"` // 'draft', 'published'
	ShowOnBanner      bool       `json:"show_on_banner"`
	CreatedBy         *string    `json:"created_by,omitempty"`
	UpdatedBy         *string    `json:"updated_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// CreateActivityRequest defines input for creating an activity.
type CreateActivityRequest struct {
	Title        string  `json:"title"`
	Description  *string `json:"description"`
	PosterURL    *string `json:"poster_url"`
	StartsAt     string  `json:"starts_at"` // ISO8601
	EndsAt       *string `json:"ends_at"`   // ISO8601
	Location     *string `json:"location"`
	Organizer    *string `json:"organizer"`
	ShowOnBanner bool    `json:"show_on_banner"`
	PublishNow   bool    `json:"publish_now"`
}

// UpdateActivityRequest defines input for updating an activity.
type UpdateActivityRequest struct {
	Title        string  `json:"title"`
	Description  *string `json:"description"`
	PosterURL    *string `json:"poster_url"`
	StartsAt     string  `json:"starts_at"` // ISO8601
	EndsAt       *string `json:"ends_at"`   // ISO8601
	Location     *string `json:"location"`
	Organizer    *string `json:"organizer"`
	ShowOnBanner bool    `json:"show_on_banner"`
}

// Announcement represents a bulletin announcement in the RT.
type Announcement struct {
	ID              string          `json:"id"`
	RTID            string          `json:"rt_id"`
	Title           string          `json:"title"`
	Body            string          `json:"body"`
	ImageURL        *string         `json:"image_url,omitempty"`
	DeepLink        *string         `json:"deep_link,omitempty"`
	TemplateID      *string         `json:"template_id,omitempty"`
	ScheduledStart  *time.Time      `json:"scheduled_start,omitempty"`
	ScheduledEnd    *time.Time      `json:"scheduled_end,omitempty"`
	Priority        int             `json:"priority"`
	DisplayOrder    int             `json:"display_order"`
	Status          string          `json:"status"` // 'draft', 'scheduled', 'active', 'expired', 'archived'
	PublishedAt     *time.Time      `json:"published_at,omitempty"`
	SnapshotContent json.RawMessage `json:"snapshot_content,omitempty"`
	IsPermanent     bool            `json:"is_permanent"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	UpdatedBy       *string         `json:"updated_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// CreateAnnouncementRequest defines input for creating an announcement.
type CreateAnnouncementRequest struct {
	Title          string  `json:"title"`
	Body           string  `json:"body"`
	ImageURL       *string `json:"image_url"`
	DeepLink       *string `json:"deep_link"`
	TemplateID     *string `json:"template_id"`
	ScheduledStart *string `json:"scheduled_start"` // ISO8601
	ScheduledEnd   *string `json:"scheduled_end"`   // ISO8601
	Priority       int     `json:"priority"`
	DisplayOrder   int     `json:"display_order"`
	PublishNow     bool    `json:"publish_now"`
}

// UpdateAnnouncementRequest defines input for updating an announcement.
type UpdateAnnouncementRequest struct {
	Title          string  `json:"title"`
	Body           string  `json:"body"`
	ImageURL       *string `json:"image_url"`
	DeepLink       *string `json:"deep_link"`
	TemplateID     *string `json:"template_id"`
	ScheduledStart *string `json:"scheduled_start"` // ISO8601
	ScheduledEnd   *string `json:"scheduled_end"`   // ISO8601
	Priority       int     `json:"priority"`
	DisplayOrder   int     `json:"display_order"`
}

// BannerTemplate represents reusable banner templates.
type BannerTemplate struct {
	ID              string    `json:"id"`
	RTID            *string   `json:"rt_id,omitempty"`
	Name            string    `json:"name"`
	Category        string    `json:"category"`
	TitleTemplate   string    `json:"title_template"`
	BodyTemplate    string    `json:"body_template"`
	ImageURL        *string   `json:"image_url,omitempty"`
	DefaultPriority int       `json:"default_priority"`
	IsSystem        bool      `json:"is_system"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Banner represents an active banner item on the Beranda.
type Banner struct {
	ID              string          `json:"id"`
	RTID            string          `json:"rt_id"`
	SourceType      string          `json:"source_type"` // 'announcement', 'activity', 'iuran_reminder', 'custom'
	SourceID        *string         `json:"source_id,omitempty"`
	TemplateID      *string         `json:"template_id,omitempty"`
	Title           string          `json:"title"`
	Body            *string         `json:"body,omitempty"`
	ImageURL        *string         `json:"image_url,omitempty"`
	DeepLink        *string         `json:"deep_link,omitempty"`
	StartAt         *time.Time      `json:"start_at,omitempty"`
	EndAt           *time.Time      `json:"end_at,omitempty"`
	Priority        int             `json:"priority"`
	DisplayOrder    int             `json:"display_order"`
	IsPinned        bool            `json:"is_pinned"`
	IsActive        bool            `json:"is_active"`
	SnapshotContent json.RawMessage `json:"snapshot_content,omitempty"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// OrgMember represents a member in the organization structure.
type OrgMember struct {
	UserID       string  `json:"user_id"`
	MembershipID string  `json:"membership_id"`
	FullName     string  `json:"full_name"`
	Jabatan      string  `json:"jabatan"`
	Phone        *string `json:"phone,omitempty"` // only included if viewer has permission
	AvatarURL    *string `json:"avatar_url,omitempty"`
}

// OrgNode represents a position node in the hierarchy.
type OrgNode struct {
	Position       string      `json:"position"`
	PositionLabel  string      `json:"position_label"`
	Level          int         `json:"level"`
	ParentPosition string      `json:"parent_position,omitempty"`
	Members        []OrgMember `json:"members"`
}

// OrganizationStructure represents the full RT organizational hierarchy.
type OrganizationStructure struct {
	RTID     string    `json:"rt_id"`
	RTName   string    `json:"rt_name"`
	RTNumber string    `json:"rt_number"`
	RWNumber string    `json:"rw_number"`
	Nodes    []OrgNode `json:"nodes"`
}

// SecurityOfficer represents an on-duty security officer (for future Security Module integration).
type SecurityOfficer struct {
	UserID      string  `json:"user_id"`
	FullName    string  `json:"full_name"`
	ShiftName   string  `json:"shift_name"`
	ShiftStart  string  `json:"shift_start"`
	ShiftEnd    string  `json:"shift_end"`
	Status      string  `json:"status"` // 'hadir', 'terlambat', etc.
	PhoneNumber     *string `json:"phone_number,omitempty"`
	AvatarPath      *string `json:"avatar_path,omitempty"`
	AvatarThumbPath *string `json:"avatar_thumb_path,omitempty"`
}

// SecurityOnDutyInfo represents the on-duty security status on the Beranda.
type SecurityOnDutyInfo struct {
	IsAvailable bool              `json:"is_available"`
	Message     string            `json:"message"`
	Officers    []SecurityOfficer `json:"officers"`
}

// HomeDashboardData represents the composite response for Beranda.
type HomeDashboardData struct {
	Greeting           string                `json:"greeting"`
	UserFullName       string                `json:"user_full_name"`
	UserRole           string                `json:"user_role"`
	UserJabatan        string                `json:"user_jabatan"`
	RTName             string                `json:"rt_name"`
	RTNumber           string                `json:"rt_number"`
	RWNumber           string                `json:"rw_number"`
	Banners            []Banner              `json:"banners"`
	UpcomingActivities []Activity            `json:"upcoming_activities"`
	Organization       OrganizationStructure `json:"organization"`
	SecurityOnDuty     SecurityOnDutyInfo    `json:"security_on_duty"`
}
