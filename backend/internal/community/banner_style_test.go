package community

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"social-finance/internal/testutil"
)

func TestBannerStyleConfig_ValidationAndNormalization(t *testing.T) {
	// 1. Valid custom color
	t.Run("valid custom color", func(t *testing.T) {
		cfg := &BannerStyleConfig{
			BackgroundType:  "color",
			BackgroundColor: "#3b82f6",
		}
		norm, err := ValidateAndNormalizeStyleConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if norm.BackgroundType != "color" {
			t.Errorf("expected background_type color, got %s", norm.BackgroundType)
		}
		if norm.BackgroundColor != "#3B82F6" {
			t.Errorf("expected normalized uppercase #3B82F6, got %s", norm.BackgroundColor)
		}
		if norm.Gradient == nil {
			t.Fatalf("expected computed gradient")
		}
		if !norm.Gradient.Glow {
			t.Errorf("expected glow=true")
		}
		if norm.Image != nil {
			t.Errorf("expected image=nil for color background")
		}
	})

	// 2. Valid custom image
	t.Run("valid custom image with crop and zoom", func(t *testing.T) {
		cfg := &BannerStyleConfig{
			BackgroundType: "image",
			Image: &ImageConfig{
				URL: strPtr("/api/v1/community/images/sample.jpg"),
				Crop: &CropRect{
					X:      0.1,
					Y:      0.1,
					Width:  0.8,
					Height: 0.45,
				},
				FocalPoint: &FocalPoint{
					X: 0.5,
					Y: 0.5,
				},
				Zoom: 1.5,
			},
		}
		norm, err := ValidateAndNormalizeStyleConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if norm.BackgroundType != "image" {
			t.Errorf("expected background_type image, got %s", norm.BackgroundType)
		}
		if norm.Image == nil || norm.Image.URL == nil || *norm.Image.URL != "/api/v1/community/images/sample.jpg" {
			t.Errorf("expected image url preserved")
		}
		// Fallback violet gradient must be attached for transparency support
		if norm.Gradient == nil || norm.Gradient.From != "#2A124D" || norm.Gradient.To != "#130924" {
			t.Errorf("expected violet fallback gradient for transparent images, got %+v", norm.Gradient)
		}
	})

	// 3. Validation rejects invalid hex color
	t.Run("rejects invalid hex colors", func(t *testing.T) {
		invalidColors := []string{"", "invalid", "#GGGGGG", "#12", "rgb(0,0,0)", "#12345678"}
		for _, col := range invalidColors {
			cfg := &BannerStyleConfig{
				BackgroundType:  "color",
				BackgroundColor: col,
			}
			_, err := ValidateAndNormalizeStyleConfig(cfg)
			if err == nil {
				t.Errorf("expected error for color %q, got nil", col)
			}
		}
	})

	// 4. Validation rejects invalid background_type
	t.Run("rejects invalid background_type", func(t *testing.T) {
		invalidTypes := []string{"", "video", "pattern", "custom"}
		for _, bt := range invalidTypes {
			cfg := &BannerStyleConfig{
				BackgroundType: bt,
			}
			_, err := ValidateAndNormalizeStyleConfig(cfg)
			if err == nil {
				t.Errorf("expected error for background_type %q, got nil", bt)
			}
		}
	})

	// 5. Validation rejects invalid crop coordinates
	t.Run("rejects invalid crop coordinates", func(t *testing.T) {
		cases := []CropRect{
			{X: -0.1, Y: 0.0, Width: 1.0, Height: 1.0},
			{X: 0.0, Y: 1.5, Width: 1.0, Height: 1.0},
			{X: 0.0, Y: 0.0, Width: 0.0, Height: 1.0},
			{X: 0.0, Y: 0.0, Width: 1.0, Height: -0.5},
			{X: 0.8, Y: 0.0, Width: 0.5, Height: 1.0}, // X + Width > 1.0001
		}
		for i, c := range cases {
			cfg := &BannerStyleConfig{
				BackgroundType: "image",
				Image: &ImageConfig{
					URL:  strPtr("/img.jpg"),
					Crop: &c,
				},
			}
			_, err := ValidateAndNormalizeStyleConfig(cfg)
			if err == nil {
				t.Errorf("case %d: expected crop error for %+v, got nil", i, c)
			}
		}
	})

	// 6. Validation bounds zoom between 1.0 and 3.0
	t.Run("bounds zoom to 1.0 - 3.0", func(t *testing.T) {
		cfg := &BannerStyleConfig{
			BackgroundType: "image",
			Image: &ImageConfig{
				URL:  strPtr("/img.jpg"),
				Zoom: 0.5,
			},
		}
		norm, err := ValidateAndNormalizeStyleConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if norm.Image.Zoom != 1.0 {
			t.Errorf("expected zoom clamped to 1.0, got %f", norm.Image.Zoom)
		}

		cfg.Image.Zoom = 5.0
		norm, err = ValidateAndNormalizeStyleConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if norm.Image.Zoom != 3.0 {
			t.Errorf("expected zoom clamped to 3.0, got %f", norm.Image.Zoom)
		}
	})

	// 7. Mutual exclusivity
	t.Run("mutual exclusivity cleans up opposite fields", func(t *testing.T) {
		cfg := &BannerStyleConfig{
			BackgroundType:  "color",
			BackgroundColor: "#00F0FF",
			Image: &ImageConfig{
				URL: strPtr("/should/be/removed.png"),
			},
		}
		norm, err := ValidateAndNormalizeStyleConfig(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if norm.Image != nil {
			t.Errorf("expected Image to be nil when background_type is color")
		}
	})
}

func TestBannerStyleConfig_ActivityLifecycle(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	repo := &Repository{pool: pool}
	svc := NewService(repo, nil)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `INSERT INTO rts (name, rt, rw, is_active) VALUES ($1, $2, $3, true) RETURNING id`, "RT Banner Activity "+suffix, suffix[len(suffix)-3:], 8000+time.Now().UnixNano()%999).Scan(&rtID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}

	var userID string
	err = pool.Raw().QueryRowContext(ctx, `INSERT INTO users (email, password_hash, full_name, is_active) VALUES ($1, 'hash', 'Activity Tester', true) RETURNING id`, "act-"+suffix+"@test.com").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// 1. Create draft activity with style_config
	styleCfg := &BannerStyleConfig{
		BackgroundType:  "color",
		BackgroundColor: "#A970FF",
		ShowDetail:      true,
	}

	createReq := CreateActivityRequest{
		Title:        "Bakti Sosial Warga",
		Description:  strPtr("Kerja bakti pembersihan saluran air"),
		StartsAt:     time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		EndsAt:       strPtr(time.Now().Add(24 * time.Hour).Format(time.RFC3339)),
		ShowOnBanner: true,
		PublishNow:   false,
		StyleConfig:  styleCfg,
	}

	act, err := svc.CreateActivity(ctx, rtID, userID, createReq)
	if err != nil {
		t.Fatalf("CreateActivity failed: %v", err)
	}
	if act.PublicationStatus != "draft" {
		t.Errorf("expected draft status, got %s", act.PublicationStatus)
	}
	if len(act.StyleConfig) == 0 {
		t.Fatalf("expected style_config stored on activity")
	}

	var parsedStyle BannerStyleConfig
	if err := json.Unmarshal(act.StyleConfig, &parsedStyle); err != nil {
		t.Fatalf("unmarshal activity style_config: %v", err)
	}
	if parsedStyle.BackgroundColor != "#A970FF" {
		t.Errorf("expected #A970FF, got %s", parsedStyle.BackgroundColor)
	}

	// Active banners must be empty because it's only a draft
	banners, err := svc.ListActiveBanners(ctx, rtID)
	if err != nil {
		t.Fatalf("ListActiveBanners: %v", err)
	}
	for _, b := range banners {
		if b.SourceID != nil && *b.SourceID == act.ID {
			t.Fatalf("draft activity should not have an active banner")
		}
	}

	// 2. Update draft activity with image style
	imgStyleCfg := &BannerStyleConfig{
		BackgroundType: "image",
		Image: &ImageConfig{
			URL: strPtr("/api/v1/community/images/clean.jpg"),
			Crop: &CropRect{
				X:      0.0,
				Y:      0.0,
				Width:  1.0,
				Height: 0.5625,
			},
			Zoom: 1.2,
		},
		ShowDetail: false,
	}
	updateReq := UpdateActivityRequest{
		Title:        act.Title,
		Description:  act.Description,
		StartsAt:     act.StartsAt.Format(time.RFC3339),
		ShowOnBanner: true,
		StyleConfig:  imgStyleCfg,
	}
	act, err = svc.UpdateActivity(ctx, rtID, act.ID, userID, updateReq)
	if err != nil {
		t.Fatalf("UpdateActivity: %v", err)
	}
	if err := json.Unmarshal(act.StyleConfig, &parsedStyle); err != nil {
		t.Fatalf("unmarshal updated activity style: %v", err)
	}
	if parsedStyle.BackgroundType != "image" || parsedStyle.ShowDetail {
		t.Errorf("expected updated image style with show_detail false, got %+v", parsedStyle)
	}

	// 3. Publish activity -> banner snapshot created
	err = svc.PublishActivity(ctx, rtID, act.ID, userID)
	if err != nil {
		t.Fatalf("PublishActivity: %v", err)
	}

	banners, err = svc.ListActiveBanners(ctx, rtID)
	if err != nil {
		t.Fatalf("ListActiveBanners: %v", err)
	}
	var actBanner *Banner
	for i := range banners {
		if banners[i].SourceID != nil && *banners[i].SourceID == act.ID {
			actBanner = &banners[i]
			break
		}
	}
	if actBanner == nil {
		t.Fatalf("expected published activity to have an active banner")
	}
	if len(actBanner.StyleConfig) == 0 {
		t.Fatalf("banner missing style_config")
	}
	var bannerStyle BannerStyleConfig
	if err := json.Unmarshal(actBanner.StyleConfig, &bannerStyle); err != nil {
		t.Fatalf("unmarshal banner style: %v", err)
	}
	if bannerStyle.BackgroundType != "image" || bannerStyle.Image == nil || bannerStyle.Image.URL == nil || *bannerStyle.Image.URL != "/api/v1/community/images/clean.jpg" {
		t.Errorf("banner style mismatch: %+v", bannerStyle)
	}

	// 4. Edit published activity -> activity updated, banner snapshot UNTOUCHED
	changedStyle := &BannerStyleConfig{
		BackgroundType:  "color",
		BackgroundColor: "#00F0FF",
	}
	updateReq2 := UpdateActivityRequest{
		Title:        "Bakti Sosial Warga (Diperbarui)",
		Description:  act.Description,
		StartsAt:     act.StartsAt.Format(time.RFC3339),
		ShowOnBanner: true,
		StyleConfig:  changedStyle,
	}
	_, err = svc.UpdateActivity(ctx, rtID, act.ID, userID, updateReq2)
	if err != nil {
		t.Fatalf("UpdateActivity published: %v", err)
	}

	// Check banner snapshot: must STILL be image style!
	banners, err = svc.ListActiveBanners(ctx, rtID)
	if err != nil {
		t.Fatalf("ListActiveBanners: %v", err)
	}
	for i := range banners {
		if banners[i].SourceID != nil && *banners[i].SourceID == act.ID {
			var snapshotCheck BannerStyleConfig
			_ = json.Unmarshal(banners[i].StyleConfig, &snapshotCheck)
			if snapshotCheck.BackgroundType != "image" {
				t.Errorf("snapshot isolation broken! live banner was mutated without republish: %+v", snapshotCheck)
			}
			if banners[i].Title != "Bakti Sosial Warga" {
				t.Errorf("snapshot title mutated without republish: %s", banners[i].Title)
			}
			break
		}
	}

	// 5. Republish -> updates banner snapshot to #00F0FF
	err = svc.PublishActivity(ctx, rtID, act.ID, userID)
	if err != nil {
		t.Fatalf("Republish Activity: %v", err)
	}
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	for i := range banners {
		if banners[i].SourceID != nil && *banners[i].SourceID == act.ID {
			var repubStyle BannerStyleConfig
			_ = json.Unmarshal(banners[i].StyleConfig, &repubStyle)
			if repubStyle.BackgroundType != "color" || repubStyle.BackgroundColor != "#00F0FF" {
				t.Errorf("republish failed to sync new style: %+v", repubStyle)
			}
			break
		}
	}

	// 6. Archive & Unarchive Activity -> banner visibility toggles with style preserved
	err = svc.ArchiveActivity(ctx, rtID, act.ID, userID)
	if err != nil {
		t.Fatalf("ArchiveActivity: %v", err)
	}
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	for _, b := range banners {
		if b.SourceID != nil && *b.SourceID == act.ID {
			t.Fatalf("archived activity banner should not be active")
		}
	}

	err = svc.UnarchiveActivity(ctx, rtID, act.ID, userID)
	if err != nil {
		t.Fatalf("UnarchiveActivity: %v", err)
	}
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	found := false
	for _, b := range banners {
		if b.SourceID != nil && *b.SourceID == act.ID {
			found = true
			var unarchStyle BannerStyleConfig
			_ = json.Unmarshal(b.StyleConfig, &unarchStyle)
			if unarchStyle.BackgroundColor != "#00F0FF" {
				t.Errorf("unarchived banner style lost: %+v", unarchStyle)
			}
			break
		}
	}
	if !found {
		t.Fatalf("unarchived activity banner should be reactivated")
	}

	// 7. Cancel Activity -> banner deactivated
	err = svc.CancelActivity(ctx, rtID, act.ID, userID)
	if err != nil {
		t.Fatalf("CancelActivity: %v", err)
	}
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	for _, b := range banners {
		if b.SourceID != nil && *b.SourceID == act.ID {
			t.Fatalf("cancelled activity banner should not be active")
		}
	}
}

func TestBannerStyleConfig_AnnouncementLifecycle(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	repo := &Repository{pool: pool}
	svc := NewService(repo, nil)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `INSERT INTO rts (name, rt, rw, is_active) VALUES ($1, $2, $3, true) RETURNING id`, "RT Banner An"+suffix, suffix[len(suffix)-3:], 7000+time.Now().UnixNano()%999).Scan(&rtID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}

	var userID string
	err = pool.Raw().QueryRowContext(ctx, `INSERT INTO users (email, password_hash, full_name, is_active) VALUES ($1, 'hash', 'Announcement Tester', true) RETURNING id`, "an-"+suffix+"@test.com").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// 1. Create draft announcement with style_config
	colorCfg := &BannerStyleConfig{
		BackgroundType:  "color",
		BackgroundColor: "#FF2A85",
		ShowDetail:      true,
	}
	createReq := CreateAnnouncementRequest{
		Title:       "Pemberitahuan Fogging",
		Body:        "Fogging nyamuk akan dilaksanakan besok pagi.",
		Priority:    5,
		PublishNow:  false,
		StyleConfig: colorCfg,
	}

	an, err := svc.CreateAnnouncement(ctx, rtID, userID, createReq)
	if err != nil {
		t.Fatalf("CreateAnnouncement: %v", err)
	}
	if an.Status != "draft" {
		t.Errorf("expected draft, got %s", an.Status)
	}
	if len(an.StyleConfig) == 0 {
		t.Fatalf("expected style_config stored on announcement")
	}

	// 2. Publish announcement -> creates banner snapshot
	err = svc.PublishAnnouncement(ctx, rtID, an.ID, userID)
	if err != nil {
		t.Fatalf("PublishAnnouncement: %v", err)
	}

	banners, err := svc.ListActiveBanners(ctx, rtID)
	if err != nil {
		t.Fatalf("ListActiveBanners: %v", err)
	}
	var anBanner *Banner
	for i := range banners {
		if banners[i].SourceID != nil && *banners[i].SourceID == an.ID {
			anBanner = &banners[i]
			break
		}
	}
	if anBanner == nil {
		t.Fatalf("expected published announcement banner")
	}
	var bStyle BannerStyleConfig
	if err := json.Unmarshal(anBanner.StyleConfig, &bStyle); err != nil {
		t.Fatalf("unmarshal banner style: %v", err)
	}
	if bStyle.BackgroundColor != "#FF2A85" {
		t.Errorf("expected #FF2A85, got %s", bStyle.BackgroundColor)
	}

	// 3. Edit announcement without republish -> snapshot preserved
	imgCfg := &BannerStyleConfig{
		BackgroundType: "image",
		Image: &ImageConfig{
			URL: strPtr("/api/v1/community/images/fogging.jpg"),
		},
	}
	updateReq := UpdateAnnouncementRequest{
		Title:       "Pemberitahuan Fogging (Jadwal Mundur)",
		Body:        an.Body,
		Priority:    an.Priority,
		StyleConfig: imgCfg,
	}
	_, err = svc.UpdateAnnouncement(ctx, rtID, an.ID, userID, updateReq)
	if err != nil {
		t.Fatalf("UpdateAnnouncement: %v", err)
	}

	// Verify banner snapshot remains unchanged (#FF2A85)
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	for i := range banners {
		if banners[i].SourceID != nil && *banners[i].SourceID == an.ID {
			var snapCheck BannerStyleConfig
			_ = json.Unmarshal(banners[i].StyleConfig, &snapCheck)
			if snapCheck.BackgroundType != "color" || snapCheck.BackgroundColor != "#FF2A85" {
				t.Errorf("announcement banner snapshot mutated without republish: %+v", snapCheck)
			}
			break
		}
	}

	// 4. Explicit republish -> updates banner snapshot
	err = svc.PublishAnnouncement(ctx, rtID, an.ID, userID)
	if err != nil {
		t.Fatalf("PublishAnnouncement (republish): %v", err)
	}
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	for i := range banners {
		if banners[i].SourceID != nil && *banners[i].SourceID == an.ID {
			var repubCheck BannerStyleConfig
			_ = json.Unmarshal(banners[i].StyleConfig, &repubCheck)
			if repubCheck.BackgroundType != "image" || repubCheck.Image == nil || repubCheck.Image.URL == nil || *repubCheck.Image.URL != "/api/v1/community/images/fogging.jpg" {
				t.Errorf("republish failed to update announcement banner snapshot: %+v", repubCheck)
			}
			break
		}
	}

	// 5. Archive announcement -> deactivates banner
	err = svc.ArchiveAnnouncement(ctx, rtID, an.ID, userID)
	if err != nil {
		t.Fatalf("ArchiveAnnouncement: %v", err)
	}
	banners, _ = svc.ListActiveBanners(ctx, rtID)
	for _, b := range banners {
		if b.SourceID != nil && *b.SourceID == an.ID {
			t.Fatalf("archived announcement banner must be deactivated")
		}
	}
}

func TestBannerStyleConfig_LegacyAndSystemBannerSafety(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	repo := &Repository{pool: pool}
	svc := NewService(repo, nil)

	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `INSERT INTO rts (name, rt, rw, is_active) VALUES ($1, $2, $3, true) RETURNING id`, "RT Safety "+suffix, suffix[len(suffix)-3:], 6000+time.Now().UnixNano()%999).Scan(&rtID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}

	// Insert a legacy banner with empty or null style_config directly
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO banners (rt_id, source_type, title, body, priority, display_order, is_active, style_config)
		VALUES ($1, 'announcement', 'Legacy Banner Test', 'Old banner without style', 1, 0, true, '{}'::jsonb)
	`, rtID)
	if err != nil {
		t.Fatalf("insert legacy banner: %v", err)
	}

	// Insert an unpaid bill to trigger synthetic iuran_reminder banner
	var dueID string
	err = pool.Raw().QueryRowContext(ctx, `INSERT INTO dues (rt_id, name, amount, period_type, is_active) VALUES ($1, 'Iuran Wajib', 50000, 'monthly', true) RETURNING id`, rtID).Scan(&dueID)
	if err != nil {
		t.Fatalf("insert due: %v", err)
	}
	var phID string
	err = pool.Raw().QueryRowContext(ctx, `INSERT INTO physical_houses (rt_id, house_number) VALUES ($1, 'A-1') RETURNING id`, rtID).Scan(&phID)
	if err != nil {
		t.Fatalf("insert physical_house: %v", err)
	}
	var houseID string
	err = pool.Raw().QueryRowContext(ctx, `INSERT INTO households (rt_id, head_name, is_active) VALUES ($1, 'Kepala Keluarga', true) RETURNING id`, rtID).Scan(&houseID)
	if err != nil {
		t.Fatalf("insert household: %v", err)
	}
	var occID string
	err = pool.Raw().QueryRowContext(ctx, `INSERT INTO household_occupancies (household_id, physical_house_id, occupancy_status, start_date) VALUES ($1, $2, 'OWNER', CURRENT_DATE) RETURNING id`, houseID, phID).Scan(&occID)
	if err != nil {
		t.Fatalf("insert occupancy: %v", err)
	}
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO bills (rt_id, household_occupancy_id, due_id, period, amount, status, is_obligatory, is_active, due_date)
		VALUES ($1, $2, $3, '2026-11', 50000, 'unpaid', true, true, CURRENT_DATE + INTERVAL '5 days')
	`, rtID, occID, dueID)
	if err != nil {
		t.Fatalf("insert bill: %v", err)
	}

	banners, err := svc.ListActiveBanners(ctx, rtID)
	if err != nil {
		t.Fatalf("ListActiveBanners: %v", err)
	}

	var hasLegacy, hasReminder bool
	for _, b := range banners {
		if b.Title == "Legacy Banner Test" {
			hasLegacy = true
			if len(b.StyleConfig) > 0 && strings.TrimSpace(string(b.StyleConfig)) != "{}" {
				t.Errorf("expected empty/default style_config on legacy banner, got %s", string(b.StyleConfig))
			}
		}
		if b.SourceType == "iuran_reminder" {
			hasReminder = true
		}
	}

	if !hasLegacy {
		t.Errorf("expected legacy banner to be returned safely")
	}
	if !hasReminder {
		t.Errorf("expected synthetic iuran reminder banner to be generated without interference")
	}
}
