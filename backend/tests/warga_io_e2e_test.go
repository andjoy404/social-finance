package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWargaIO_E2E(t *testing.T) {
	assertTestBackendTarget(t)
	baseURL := getBaseURL()

	// 1. Log in users
	tokenPengurus := login(t, emailPengurusA, testPassword)
	tokenBendahara := login(t, emailBendaharaA, testPassword)
	tokenWarga := login(t, emailWargaB, testPassword)

	nowNano := time.Now().UnixNano()
	house1 := fmt.Sprintf("H-E2E-%d", nowNano%10000)
	nik1 := fmt.Sprintf("317101%010d", nowNano%10000000000)
	nik2 := fmt.Sprintf("317102%010d", (nowNano+1)%10000000000)

	csvValid := fmt.Sprintf(`house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
%s,Bambang Pamungkas,OWNER,Jl. Melati E2E,Bambang Pamungkas,%s,0812111111,bambang@test.com,HEAD
%s,Bambang Pamungkas,OWNER,Jl. Melati E2E,Tribun Pamungkas,%s,0812111112,tribun@test.com,CHILD
`, house1, nik1, house1, nik2)

	// Scenario 1: Role authorization - Warga cannot import
	t.Run("Warga cannot import preview", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/warga/import/preview", strings.NewReader(csvValid))
		req.Header.Set("Authorization", "Bearer "+tokenWarga)
		req.Header.Set("Content-Type", "text/csv")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for Warga, got %d", resp.StatusCode)
		}
	})

	// Scenario 2: Pengurus preview valid CSV
	t.Run("Pengurus preview valid CSV", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/warga/import/preview", strings.NewReader(csvValid))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
		var preview struct {
			TotalRows int `json:"total_rows"`
			ValidRows int `json:"valid_rows"`
		}
		json.NewDecoder(resp.Body).Decode(&preview)
		if preview.TotalRows != 2 || preview.ValidRows != 2 {
			t.Errorf("expected 2 total and 2 valid rows, got %+v", preview)
		}
	})

	// Scenario 3: Validation Error Report on invalid CSV
	t.Run("Pengurus preview invalid CSV returns error report", func(t *testing.T) {
		csvInvalid := `house_number,head_name,occupancy_status,address,resident_name,nik,phone,email,relationship_to_head
,,INVALID_STATUS,,0812,,0812,,HEAD
`
		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/warga/import/preview", strings.NewReader(csvInvalid))
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", "text/csv")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200 OK with errors reported, got %d: %s", resp.StatusCode, string(body))
		}
		var preview struct {
			InvalidRows int `json:"invalid_rows"`
			Errors      []struct {
				Row   int    `json:"row"`
				Field string `json:"field"`
				Code  string `json:"code"`
			} `json:"errors"`
		}
		json.NewDecoder(resp.Body).Decode(&preview)
		if preview.InvalidRows == 0 || len(preview.Errors) == 0 {
			t.Errorf("expected row errors reported, got %+v", preview)
		}
	})

	// Scenario 4: Pengurus commit valid CSV via multipart
	t.Run("Pengurus commit valid CSV", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		part, err := writer.CreateFormFile("file", "import_test.csv")
		if err != nil {
			t.Fatalf("create multipart part: %v", err)
		}
		part.Write([]byte(csvValid))
		writer.Close()

		req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/warga/import/commit", body)
		req.Header.Set("Authorization", "Bearer "+tokenPengurus)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 201 Created, got %d: %s", resp.StatusCode, string(respBody))
		}
		var commitResp struct {
			Status             string `json:"status"`
			ImportedHouseholds int    `json:"imported_households"`
			ImportedResidents  int    `json:"imported_residents"`
		}
		json.NewDecoder(resp.Body).Decode(&commitResp)
		if commitResp.ImportedHouseholds != 1 || commitResp.ImportedResidents != 2 {
			t.Errorf("unexpected commit counts: %+v", commitResp)
		}
	})

	// Scenario 5: Export CSV includes committed data
	t.Run("Export CSV includes committed warga", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/api/v1/warga/export", nil)
		req.Header.Set("Authorization", "Bearer "+tokenBendahara)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
		}
		contentType := resp.Header.Get("Content-Type")
		if !strings.Contains(contentType, "text/csv") {
			t.Errorf("expected text/csv content type, got %s", contentType)
		}
		exportBytes, _ := io.ReadAll(resp.Body)
		exportStr := string(exportBytes)
		if !strings.Contains(exportStr, house1) || !strings.Contains(exportStr, "Bambang Pamungkas") {
			t.Errorf("exported CSV does not contain newly imported data: %s", exportStr)
		}
	})
}
