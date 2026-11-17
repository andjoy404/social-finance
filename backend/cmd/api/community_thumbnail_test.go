package main

// ============================================================================
// Integration test: an announcement created from a banner template that only
// carries an image_url must render that template image on the announcement
// API response AND on the /api/v1/banners response. This is the contract the
// Web and Flutter Pengumuman thumbnails depend on.
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"social-finance/internal/auth"
)

func TestCommunity_AnnouncementBannerInheritsTemplateImage(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena
	_, ketuaToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKetua)

	const imageKey = "community/11111111-2222-3333-4444-555555555555/announcement/tpl.png"
	imageURL := "/api/v1/community/images/" + imageKey

	var tmplID string
	if err := pool.Raw().QueryRowContext(context.Background(), `
		INSERT INTO banner_templates (id, rt_id, name, category, title_template, body_template, image_url, is_active)
		VALUES (gen_random_uuid(), $1, 'Thumbnail Template', 'general', 'T', 'B', $2, true)
		RETURNING id`, rtID, imageURL).Scan(&tmplID); err != nil {
		t.Fatalf("insert banner_template: %v", err)
	}

	now := time.Now()
	body := jsonBody(t, map[string]any{
		"title":           "Template thumbnail test",
		"body":            "should inherit image_url from template",
		"template_id":     tmplID,
		"publish_now":     true,
		"show_on_banner":  true,
		"scheduled_start": now.Add(-time.Minute).Format(time.RFC3339),
		"scheduled_end":   now.Add(time.Hour).Format(time.RFC3339),
	})
	create := doReq(t, router, http.MethodPost, "/api/v1/announcements", ketuaToken, body, "application/json")
	if create.Code != http.StatusCreated {
		t.Fatalf("create announcement = %d: %s", create.Code, create.Body.String())
	}
	var ann map[string]any
	_ = json.Unmarshal(create.Body.Bytes(), &ann)
	if annID, _ := ann["id"].(string); annID == "" {
		t.Fatalf("announcement id missing")
	}
	got := fmt.Sprint(ann["image_url"])
	if !strings.Contains(got, "announcement/tpl.png") {
		t.Fatalf("announcement image_url = %q, want template image_url %q", got, imageURL)
	}

	rec := doReq(t, router, http.MethodGet, "/api/v1/banners", ketuaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list banners = %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	data, _ := resp["data"].([]any)
	found := false
	for _, item := range data {
		banner, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if banner["source_type"] == "announcement" && banner["source_id"] == ann["id"] {
			found = true
			got := fmt.Sprint(banner["image_url"])
			if !strings.Contains(got, "announcement/tpl.png") {
				t.Fatalf("banner image_url = %q, want template image_url %q", got, imageURL)
			}
		}
	}
	if !found {
		t.Fatalf("announcement banner not found in list")
	}
}
