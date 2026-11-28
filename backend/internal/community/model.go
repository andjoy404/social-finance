package community

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const TestCommunityCode = "test-community"

var communityCodePattern = regexp.MustCompile(`^[a-z0-9]+([a-z0-9-]*[a-z0-9])?$`)
var hexColorRegex = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// CropRect defines normalized crop coordinates (0.0 to 1.0).
type CropRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// FocalPoint defines normalized focus point coordinates (0.0 to 1.0).
type FocalPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// ImageConfig defines image background properties and crop transformations.
type ImageConfig struct {
	URL        *string     `json:"url,omitempty"`
	Crop       *CropRect   `json:"crop,omitempty"`
	FocalPoint *FocalPoint `json:"focal_point,omitempty"`
	Zoom       float64     `json:"zoom"`
}

// GradientConfig defines the smooth neon gradient properties.
type GradientConfig struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Angle int    `json:"angle"`
	Glow  bool   `json:"glow"`
}

// BannerStyleConfig defines customizable background style for banners.
type BannerStyleConfig struct {
	Version         int             `json:"version"`
	BackgroundType  string          `json:"background_type"` // "color" or "image"
	BackgroundColor string          `json:"background_color,omitempty"`
	Gradient        *GradientConfig `json:"gradient,omitempty"`
	Image           *ImageConfig    `json:"image,omitempty"`
	OverlayOpacity  float64         `json:"overlay_opacity"`
	ShowDetail      bool            `json:"show_detail"`
}

// NormalizeHexColor normalizes #RGB to #RRGGBB in uppercase.
func NormalizeHexColor(hex string) string {
	hex = strings.TrimSpace(strings.ToUpper(hex))
	if !strings.HasPrefix(hex, "#") {
		hex = "#" + hex
	}
	if len(hex) == 4 {
		// #RGB -> #RRGGBB
		r := string(hex[1])
		g := string(hex[2])
		b := string(hex[3])
		hex = "#" + r + r + g + g + b + b
	}
	return hex
}

func isValidHexColor(hex string) bool {
	return hexColorRegex.MatchString(hex)
}

func parseHexColor(hex string) (r, g, b int, err error) {
	norm := NormalizeHexColor(hex)
	if !isValidHexColor(norm) {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %s", hex)
	}
	rgb, err := strconv.ParseUint(norm[1:], 16, 32)
	if err != nil {
		return 0, 0, 0, err
	}
	return int((rgb >> 16) & 0xFF), int((rgb >> 8) & 0xFF), int(rgb & 0xFF), nil
}

// ComputeNeonGradient creates a sleek neon smooth gradient/glow based on primary color.
func ComputeNeonGradient(hexColor string) GradientConfig {
	hexColor = NormalizeHexColor(hexColor)
	r, g, b, err := parseHexColor(hexColor)
	if err != nil {
		return GradientConfig{
			From:  "#A970FF",
			To:    "#1A0B2E",
			Angle: 135,
			Glow:  true,
		}
	}
	// Darker tone blended with dark base (#0F172A)
	toR := int(float64(r)*0.25 + 15*0.75)
	toG := int(float64(g)*0.25 + 23*0.75)
	toB := int(float64(b)*0.25 + 42*0.75)
	if toR > 255 {
		toR = 255
	}
	if toG > 255 {
		toG = 255
	}
	if toB > 255 {
		toB = 255
	}

	toHex := fmt.Sprintf("#%02X%02X%02X", toR, toG, toB)
	return GradientConfig{
		From:  hexColor,
		To:    toHex,
		Angle: 135,
		Glow:  true,
	}
}

// VioletTransparencyGradient provides smooth violet gradient for transparent PNG/WebP images.
func VioletTransparencyGradient() GradientConfig {
	return GradientConfig{
		From:  "#2A124D",
		To:    "#130924",
		Angle: 135,
		Glow:  true,
	}
}

// ValidateAndNormalizeStyleConfig validates and normalizes BannerStyleConfig.
func ValidateAndNormalizeStyleConfig(cfg *BannerStyleConfig) (*BannerStyleConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	norm := *cfg
	if norm.Version <= 0 {
		norm.Version = 1
	}

	if norm.BackgroundType != "color" && norm.BackgroundType != "image" {
		return nil, fmt.Errorf("%w: invalid background_type (must be 'color' or 'image')", ErrValidation)
	}

	if norm.OverlayOpacity < 0 {
		norm.OverlayOpacity = 0
	} else if norm.OverlayOpacity > 1.0 {
		norm.OverlayOpacity = 1.0
	}
	if norm.OverlayOpacity == 0 && cfg.OverlayOpacity == 0 {
		norm.OverlayOpacity = 0.35
	}

	if norm.BackgroundType == "color" {
		if strings.TrimSpace(norm.BackgroundColor) == "" {
			return nil, fmt.Errorf("%w: background_color is required when background_type is color", ErrValidation)
		}
		hex := NormalizeHexColor(norm.BackgroundColor)
		if !isValidHexColor(hex) {
			return nil, fmt.Errorf("%w: invalid background_color hex format", ErrValidation)
		}
		norm.BackgroundColor = hex
		grad := ComputeNeonGradient(norm.BackgroundColor)
		norm.Gradient = &grad
		norm.Image = nil // Mutually exclusive: clear image in color mode
	} else {
		// Image background mode
		if norm.Image == nil || norm.Image.URL == nil || strings.TrimSpace(*norm.Image.URL) == "" {
			return nil, fmt.Errorf("%w: image url is required for image background", ErrValidation)
		}
		cleanURL := strings.TrimSpace(*norm.Image.URL)
		norm.Image.URL = &cleanURL

		if norm.Image.Zoom < 1.0 {
			norm.Image.Zoom = 1.0
		} else if norm.Image.Zoom > 3.0 {
			norm.Image.Zoom = 3.0
		}

		if norm.Image.Crop == nil {
			norm.Image.Crop = &CropRect{X: 0, Y: 0, Width: 1, Height: 1}
		} else {
			c := norm.Image.Crop
			if c.X < 0 || c.Y < 0 || c.Width <= 0 || c.Height <= 0 || c.X+c.Width > 1.0001 || c.Y+c.Height > 1.0001 {
				return nil, fmt.Errorf("%w: crop coordinates must be normalized within [0, 1]", ErrValidation)
			}
		}

		if norm.Image.FocalPoint == nil {
			norm.Image.FocalPoint = &FocalPoint{X: 0.5, Y: 0.5}
		} else {
			fp := norm.Image.FocalPoint
			if fp.X < 0 || fp.X > 1 || fp.Y < 0 || fp.Y > 1 {
				return nil, fmt.Errorf("%w: focal point coordinates must be normalized within [0, 1]", ErrValidation)
			}
		}

		// Under transparent PNG/WebP, show smooth violet gradient
		grad := VioletTransparencyGradient()
		norm.Gradient = &grad
	}

	return &norm, nil
}

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
	ID                string          `json:"id"`
	RTID              string          `json:"rt_id"`
	Title             string          `json:"title"`
	Description       *string         `json:"description,omitempty"`
	PosterURL         *string         `json:"poster_url,omitempty"`
	StartsAt          time.Time       `json:"starts_at"`
	EndsAt            *time.Time      `json:"ends_at,omitempty"`
	Location          *string         `json:"location,omitempty"`
	Organizer         *string         `json:"organizer,omitempty"`
	Status            string          `json:"status"`             // 'upcoming', 'ongoing', 'completed', 'cancelled'
	PublicationStatus string          `json:"publication_status"` // 'draft', 'published'
	ShowOnBanner      bool            `json:"show_on_banner"`
	ArchivedAt        *time.Time      `json:"archived_at,omitempty"`
	StyleConfig       json.RawMessage `json:"style_config,omitempty"`
	CreatedBy         *string         `json:"created_by,omitempty"`
	UpdatedBy         *string         `json:"updated_by,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// CreateActivityRequest defines input for creating an activity.
type CreateActivityRequest struct {
	Title        string             `json:"title"`
	Description  *string            `json:"description"`
	PosterURL    *string            `json:"poster_url"`
	StartsAt     string             `json:"starts_at"` // ISO8601
	EndsAt       *string            `json:"ends_at"`   // ISO8601
	Location     *string            `json:"location"`
	Organizer    *string            `json:"organizer"`
	ShowOnBanner bool               `json:"show_on_banner"`
	PublishNow   bool               `json:"publish_now"`
	StyleConfig  *BannerStyleConfig `json:"style_config,omitempty"`
}

// UpdateActivityRequest defines input for updating an activity.
type UpdateActivityRequest struct {
	Title        string             `json:"title"`
	Description  *string            `json:"description"`
	PosterURL    *string            `json:"poster_url"`
	StartsAt     string             `json:"starts_at"` // ISO8601
	EndsAt       *string            `json:"ends_at"`   // ISO8601
	Location     *string            `json:"location"`
	Organizer    *string            `json:"organizer"`
	ShowOnBanner bool               `json:"show_on_banner"`
	StyleConfig  *BannerStyleConfig `json:"style_config,omitempty"`
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
	StyleConfig     json.RawMessage `json:"style_config,omitempty"`
	CreatedBy       *string         `json:"created_by,omitempty"`
	UpdatedBy       *string         `json:"updated_by,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// CreateAnnouncementRequest defines input for creating an announcement.
type CreateAnnouncementRequest struct {
	Title          string             `json:"title"`
	Body           string             `json:"body"`
	ImageURL       *string            `json:"image_url"`
	DeepLink       *string            `json:"deep_link"`
	TemplateID     *string            `json:"template_id"`
	ScheduledStart *string            `json:"scheduled_start"` // ISO8601
	ScheduledEnd   *string            `json:"scheduled_end"`   // ISO8601
	Priority       int                `json:"priority"`
	DisplayOrder   int                `json:"display_order"`
	PublishNow     bool               `json:"publish_now"`
	StyleConfig    *BannerStyleConfig `json:"style_config,omitempty"`
}

// UpdateAnnouncementRequest defines input for updating an announcement.
type UpdateAnnouncementRequest struct {
	Title          string             `json:"title"`
	Body           string             `json:"body"`
	ImageURL       *string            `json:"image_url"`
	DeepLink       *string            `json:"deep_link"`
	TemplateID     *string            `json:"template_id"`
	ScheduledStart *string            `json:"scheduled_start"` // ISO8601
	ScheduledEnd   *string            `json:"scheduled_end"`   // ISO8601
	Priority       int                `json:"priority"`
	DisplayOrder   int                `json:"display_order"`
	StyleConfig    *BannerStyleConfig `json:"style_config,omitempty"`
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
	StyleConfig     json.RawMessage `json:"style_config,omitempty"`
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
	HouseNumber  *string `json:"house_number,omitempty"`
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
