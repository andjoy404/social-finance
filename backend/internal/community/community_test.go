package community

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/storage"
)

func TestValidateDeepLink(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"empty string", "", false},
		{"whitespace", "   ", false},
		{"valid keuangan", "/keuangan", false},
		{"valid keuangan with subpath", "/keuangan/bills", false},
		{"valid iuran", "/iuran", false},
		{"valid kas", "/kas", false},
		{"valid kegiatan", "/kegiatan", false},
		{"valid kegiatan id", "/kegiatan/123-abc", false},
		{"valid warga", "/warga", false},
		{"valid notifikasi", "/notifikasi", false},
		{"valid profil", "/profil", false},
		{"external https", "https://google.com", true},
		{"external http", "http://malicious.site", true},
		{"protocol-relative", "//evil.com", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"disallowed path", "/admin/super-secret", true},
		{"disallowed root", "/", true},
		{"backslash injection", "/keuangan\\..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateDeepLink(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateDeepLink(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && tt.input != "" && got != strings.TrimSpace(tt.input) {
				t.Errorf("ValidateDeepLink(%q) = %q, want %q", tt.input, got, strings.TrimSpace(tt.input))
			}
		})
	}
}

func TestActivityDynamicStatus(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, loc) // fixed clock, Asia/Jakarta
	startsPast := now.Add(-2 * time.Hour)
	startsFuture := now.Add(2 * time.Hour)
	endsPast := now.Add(-1 * time.Hour)
	endsFuture := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		act  Activity
		want string
	}{
		{
			name: "upcoming when start is in the future",
			act:  Activity{StartsAt: startsFuture, Status: "upcoming"},
			want: "upcoming",
		},
		{
			name: "ongoing when start passed and no end set",
			act:  Activity{StartsAt: startsPast, Status: "upcoming"},
			want: "ongoing",
		},
		{
			name: "ongoing when now exactly equals start (half-open)",
			act:  Activity{StartsAt: now, Status: "upcoming"},
			want: "ongoing",
		},
		{
			name: "ongoing when started and not yet ended",
			act:  Activity{StartsAt: startsPast, EndsAt: &endsFuture, Status: "upcoming"},
			want: "ongoing",
		},
		{
			name: "completed when end passed",
			act:  Activity{StartsAt: startsPast, EndsAt: &endsPast, Status: "upcoming"},
			want: "completed",
		},
		{
			name: "completed when now exactly equals end (half-open)",
			act:  Activity{StartsAt: startsPast, EndsAt: &now, Status: "upcoming"},
			want: "completed",
		},
		{
			name: "cancelled wins over time window",
			act:  Activity{StartsAt: startsFuture, Status: "cancelled"},
			want: "cancelled",
		},
		{
			name: "cancelled wins even for currently-running activity",
			act:  Activity{StartsAt: startsPast, EndsAt: &endsFuture, Status: "cancelled"},
			want: "cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := activityDynamicStatus(&tt.act, now); got != tt.want {
				t.Errorf("activityDynamicStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAnnouncementVisibleForWarga(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatalf("load tz: %v", err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, loc)
	startsPast := now.Add(-2 * time.Hour)
	startsFuture := now.Add(2 * time.Hour)
	endsPast := now.Add(-1 * time.Hour)
	endsFuture := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		ann  Announcement
		want bool
	}{
		{
			name: "active, in window",
			ann:  Announcement{Status: "active", ScheduledStart: &startsPast, ScheduledEnd: &endsFuture},
			want: true,
		},
		{
			name: "active, window not started yet",
			ann:  Announcement{Status: "active", ScheduledStart: &startsFuture, ScheduledEnd: &endsFuture},
			want: false,
		},
		{
			name: "active, exactly at window start (half-open, visible)",
			ann:  Announcement{Status: "active", ScheduledStart: &now, ScheduledEnd: &endsFuture},
			want: true,
		},
		{
			name: "active, exactly at window end (half-open, hidden)",
			ann:  Announcement{Status: "active", ScheduledStart: &startsPast, ScheduledEnd: &now},
			want: false,
		},
		{
			name: "active, window ended",
			ann:  Announcement{Status: "active", ScheduledStart: &startsPast, ScheduledEnd: &endsPast},
			want: false,
		},
		{
			name: "active, open-ended start",
			ann:  Announcement{Status: "active", ScheduledEnd: &endsFuture},
			want: true,
		},
		{
			name: "active, open-ended end",
			ann:  Announcement{Status: "active", ScheduledStart: &startsPast},
			want: true,
		},
		{
			name: "active, no window at all",
			ann:  Announcement{Status: "active"},
			want: true,
		},
		{
			name: "draft is not visible even in window",
			ann:  Announcement{Status: "draft", ScheduledStart: &startsPast, ScheduledEnd: &endsFuture},
			want: false,
		},
		{
			name: "scheduled is not visible before start",
			ann:  Announcement{Status: "scheduled", ScheduledStart: &startsFuture, ScheduledEnd: &endsFuture},
			want: false,
		},
		{
			name: "archived is not visible in window",
			ann:  Announcement{Status: "archived", ScheduledStart: &startsPast, ScheduledEnd: &endsFuture},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := announcementVisibleForWarga(&tt.ann, now); got != tt.want {
				t.Errorf("announcementVisibleForWarga() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseScheduleField(t *testing.T) {
	tests := []struct {
		name    string
		value   *string
		wantErr bool
		wantSet bool
	}{
		{name: "nil", value: nil, wantErr: false},
		{name: "empty string", value: strPtr(""), wantErr: false},
		{name: "whitespace", value: strPtr("   "), wantErr: false},
		{name: "valid RFC3339 with Z timezone", value: strPtr("2026-11-10T12:00:00Z"), wantSet: true},
		{name: "valid RFC3339 with offset", value: strPtr("2026-11-10T12:00:00+07:00"), wantSet: true},
		{name: "missing timezone is rejected", value: strPtr("2026-11-10T12:00:00"), wantErr: true},
		{name: "invalid month", value: strPtr("2026-13-10T12:00:00Z"), wantErr: true},
		{name: "not a date", value: strPtr("tomorrow morning"), wantErr: true},
		{name: "date only is rejected (need explicit time)", value: strPtr("2026-11-10"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScheduleField(tt.value, "scheduled_start")
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseScheduleField() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (got != nil) != tt.wantSet {
				t.Fatalf("parseScheduleField() set = %v, want %v", got != nil, tt.wantSet)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestMockStorageCommunityImage(t *testing.T) {
	mock := storage.NewMockStorage()
	// Minimal valid 1x1 PNG bytes
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG header
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	r := strings.NewReader(string(pngBytes))

	meta, err := mock.UploadCommunityImage(context.Background(), "rt-1", "activity", "poster.png", r, int64(len(pngBytes)))
	if err != nil {
		t.Fatalf("UploadCommunityImage error: %v", err)
	}
	if !strings.HasPrefix(meta.Key, "community/rt-1/activity/") {
		t.Errorf("unexpected key format: %s", meta.Key)
	}

	objReader, err := mock.GetCommunityImage(context.Background(), meta.Key)
	if err != nil {
		t.Fatalf("GetCommunityImage error: %v", err)
	}
	defer objReader.Body.Close()

	if objReader.Size != int64(len(pngBytes)) {
		t.Errorf("expected size %d, got %d", len(pngBytes), objReader.Size)
	}
}

func TestVerifyImageSignatureAt(t *testing.T) {
	secret := []byte("test-secret-key-32-chars-long-ok!!")
	old := auth.SigningSecret
	auth.SigningSecret = secret
	defer func() { auth.SigningSecret = old }()

	key := "community/rt-1/announcement/test.png"
	exp := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	// Valid signature for exp (computed with the same algorithm).
	sig := imageSignature(key, exp.Unix())
	expStr := strconv.FormatInt(exp.Unix(), 10)

	tests := []struct {
		name string
		now  time.Time
		sig  string
		want bool
	}{
		{
			name: "valid signature, now before expiry",
			now:  exp.Add(-1 * time.Minute),
			sig:  sig,
			want: true,
		},
		{
			name: "now exactly equals expiry (rejected)",
			now:  exp,
			sig:  sig,
			want: false,
		},
		{
			name: "now after expiry (rejected)",
			now:  exp.Add(1 * time.Second),
			sig:  sig,
			want: false,
		},
		{
			name: "tampered signature",
			now:  exp.Add(-1 * time.Minute),
			sig:  sig[:len(sig)-2] + "00",
			want: false,
		},
		{
			name: "signature for a different key is rejected",
			now:  exp.Add(-1 * time.Minute),
			sig:  imageSignature("community/rt-1/announcement/other.png", exp.Unix()),
			want: false,
		},
		{
			name: "key/signature mismatch is rejected",
			now:  exp.Add(-1 * time.Minute),
			sig:  imageSignature("community/rt-2/announcement/test.png", exp.Unix()),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifyImageSignatureAt(key, expStr, tt.sig, tt.now)
			if got != tt.want {
				t.Errorf("verifyImageSignatureAt() = %v, want %v", got, tt.want)
			}
		})
	}
}
