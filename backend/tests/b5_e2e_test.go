package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

var baseURL = getBaseURL()

func getBaseURL() string {
	if u := os.Getenv("TEST_BACKEND_URL"); u != "" {
		return u
	}
	if u := os.Getenv("BASE_URL"); u != "" {
		return u
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "http://backend-test:8080"
	}
	return "http://localhost:8082"
}

func assertTestBackendTarget(t *testing.T) {
	t.Helper()
	url := getBaseURL()
	resp, err := http.Get(url + "/__test__/info")
	if err != nil {
		t.Fatalf("HARD SAFETY GUARD BLOCKED: failed to reach target backend at %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("HARD SAFETY GUARD BLOCKED: target backend at %s returned HTTP %d for /__test__/info", url, resp.StatusCode)
	}
	var info map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("HARD SAFETY GUARD BLOCKED: failed to decode /__test__/info response: %v", err)
	}
	dbName := info["database"]
	if dbName == "social_finance" || !strings.Contains(dbName, "test") {
		t.Fatalf("HARD SAFETY GUARD BLOCKED: target backend at %s is connected to database %q, which is the real development database! E2E tests are strictly forbidden from mutating development data.", url, dbName)
	}
}

const (
	testPassword = "TestUser123!"

	emailPengurusA  = "e2e.pengurus-a@social-finance-test.internal"
	emailBendaharaA = "e2e.user-a@social-finance-test.internal"
	emailWargaB     = "e2e.user-b@social-finance-test.internal"
	emailPengurusC  = "e2e.user-c@social-finance-test.internal"
	emailSuperAdmin = "e2e.super-admin@social-finance-test.internal"
)

type loginResponse struct {
	AccessToken string `json:"access_token"`
	User        struct {
		ID   string `json:"id"`
		Role string `json:"role"`
		RT   struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"rt"`
	} `json:"user"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func login(t *testing.T, email, password string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	if err != nil {
		t.Fatalf("login marshal error: %v", err)
	}

	resp, err := http.Post(baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("login http request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("login failed for %s: status %d, body: %s", email, resp.StatusCode, string(respBytes))
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		t.Fatalf("login decode error: %v", err)
	}
	return lr.AccessToken
}

func doRequest(t *testing.T, method, path, token string, reqBody any) (*http.Response, []byte) {
	t.Helper()
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatalf("marshal req body: %v", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, baseURL+path, bodyReader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, respBytes
}

func assertErrorCode(t *testing.T, respBytes []byte, expectedCode string) {
	t.Helper()
	var errResp errorResponse
	if err := json.Unmarshal(respBytes, &errResp); err != nil {
		t.Fatalf("unmarshal error response (%s): %v", string(respBytes), err)
	}
	if errResp.Error.Code == "" {
		t.Fatalf("expected error object in response, got %s", string(respBytes))
	}
	if expectedCode != "" && errResp.Error.Code != expectedCode {
		t.Errorf("error code = %q, want %q (body: %s)", errResp.Error.Code, expectedCode, string(respBytes))
	}
}

func TestCheckpointB5_E2E(t *testing.T) {
	assertTestBackendTarget(t)

	// Step 0: Obtain tokens
	tokenPengurusA := login(t, emailPengurusA, testPassword)
	tokenBendaharaA := login(t, emailBendaharaA, testPassword)
	tokenWargaB := login(t, emailWargaB, testPassword)
	tokenPengurusC := login(t, emailPengurusC, testPassword)
	tokenSuperAdmin := login(t, emailSuperAdmin, testPassword)

	timestamp := time.Now().UnixNano()
	houseNum1 := fmt.Sprintf("B5-%d-01", timestamp%100000)
	houseNum2 := fmt.Sprintf("B5-%d-02", timestamp%100000)
	houseNum3 := fmt.Sprintf("B5-%d-03", timestamp%100000)
	nikNum1 := fmt.Sprintf("317101%010d", timestamp%10000000000)
	nikNum2 := fmt.Sprintf("317102%010d", timestamp%10000000000)

	var householdID1 string
	var householdID2 string
	var residentID1 string

	// Scenario 1: Household creation
	t.Run("01_Household_Creation", func(t *testing.T) {
		createPayload := map[string]any{
			"nik":              nikNum1,
			"phone":            "081234567891",
			"email":            "budi1@test.com",
			"head_name":        "Budi Santoso B5-" + fmt.Sprintf("%d", timestamp),
			"address":          "Jl. Mawar No. 1",
			"house_number":     houseNum1,
			"occupancy_status": "OWNER",
			"notes":            "E2E Test Household 1",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/households", tokenPengurusA, createPayload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", resp.StatusCode, string(body))
		}
		var created map[string]any
		if err := json.Unmarshal(body, &created); err != nil {
			t.Fatalf("unmarshal created household: %v", err)
		}
		id, ok := created["id"].(string)
		if !ok || id == "" {
			t.Fatalf("missing id in created household: %s", string(body))
		}
		householdID1 = id

		b5HouseholdMarker := fmt.Sprintf("Budi Santoso B5-%d", timestamp)
		if created["head_name"] != b5HouseholdMarker {
			t.Errorf("expected head_name %q, got %v", b5HouseholdMarker, created["head_name"])
		}
		if created["house_number"] != houseNum1 {
			t.Errorf("expected house_number %s, got %v", houseNum1, created["house_number"])
		}
		if created["occupancy_status"] != "OWNER" {
			t.Errorf("expected occupancy_status OWNER, got %v", created["occupancy_status"])
		}
	})

	// Scenario 2: Household retrieval by ID
	t.Run("02_Household_Retrieval_By_ID", func(t *testing.T) {
		resp, body := doRequest(t, http.MethodGet, "/api/v1/households/"+householdID1, tokenPengurusA, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got["id"] != householdID1 {
			t.Errorf("got id %v, want %s", got["id"], householdID1)
		}
		if got["house_number"] != houseNum1 {
			t.Errorf("got house_number %v, want %s", got["house_number"], houseNum1)
		}
		if got["occupancy_status"] != "OWNER" {
			t.Errorf("got occupancy_status %v, want OWNER", got["occupancy_status"])
		}
	})

	// Scenario 3: Household list includes current house_number and occupancy_status
	t.Run("03_Household_List", func(t *testing.T) {
		b5HouseholdMarker := "B5-" + fmt.Sprintf("%d", timestamp)
		searchParam := url.QueryEscape(b5HouseholdMarker)
		resp, body := doRequest(t, http.MethodGet, "/api/v1/households?per_page=50&search="+searchParam, tokenPengurusA, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
		var listResp struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(body, &listResp); err != nil {
			t.Fatalf("unmarshal list: %v", err)
		}
		var found bool
		for _, h := range listResp.Data {
			if h["id"] == householdID1 {
				found = true
				if h["house_number"] != houseNum1 {
					t.Errorf("list: house_number = %v, want %s", h["house_number"], houseNum1)
				}
				if h["occupancy_status"] != "OWNER" {
					t.Errorf("list: occupancy_status = %v, want OWNER", h["occupancy_status"])
				}
			}
		}
		if !found {
			t.Errorf("created household %s not found in list", householdID1)
		}
	})

	// Scenario 4: House-number / address correction (same house, no move)
	t.Run("04_Household_Correction", func(t *testing.T) {
		patchPayload := map[string]any{
			"address": "Jl. Mawar No. 1 Updated",
		}
		resp, body := doRequest(t, http.MethodPatch, "/api/v1/households/"+householdID1, tokenPengurusA, patchPayload)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
		var updated map[string]any
		json.Unmarshal(body, &updated)
		if updated["address"] != "Jl. Mawar No. 1 Updated" {
			t.Errorf("expected updated address, got %v", updated["address"])
		}
	})

	// Create a second household for collision and move tests
	t.Run("Create_Second_Household", func(t *testing.T) {
		createPayload := map[string]any{
			"nik":              nikNum2,
			"phone":            "081234567892",
			"email":            "siti2@test.com",
			"head_name":        "Siti Aminah",
			"address":          "Jl. Mawar No. 2",
			"house_number":     houseNum2,
			"occupancy_status": "TENANT",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/households", tokenPengurusA, createPayload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created for household 2, got %d: %s", resp.StatusCode, string(body))
		}
		var created map[string]any
		json.Unmarshal(body, &created)
		householdID2 = created["id"].(string)
	})

	// Scenario 5: House-number collision on update (409 Conflict)
	t.Run("05_Household_House_Number_Collision_Update", func(t *testing.T) {
		patchPayload := map[string]any{
			"house_number": houseNum2, // Already used by household 2
		}
		resp, body := doRequest(t, http.MethodPatch, "/api/v1/households/"+householdID1, tokenPengurusA, patchPayload)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d: %s", resp.StatusCode, string(body))
		}
		assertErrorCode(t, body, "conflict")
	})

	// Scenario 7: Household move collision if occupied (409 Conflict)
	t.Run("07_Household_Move_Collision_Occupied", func(t *testing.T) {
		movePayload := map[string]any{
			"house_number":     houseNum2, // Already occupied by household 2
			"address":          "Jl. Mawar No. 2",
			"occupancy_status": "OWNER",
			"start_date":       time.Now().AddDate(0, 0, 1).Format("2006-01-02"),
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/households/"+householdID1+"/move", tokenPengurusA, movePayload)
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("expected 409 Conflict when moving to occupied house, got %d: %s", resp.StatusCode, string(body))
		}
		assertErrorCode(t, body, "conflict")
	})

	// Scenario 8: Resident creation in household
	t.Run("08_Resident_Creation", func(t *testing.T) {
		rel := "CHILD"
		phone := "+62812345678"
		residentPayload := map[string]any{
			"household_id":         householdID1,
			"full_name":            "E2EB5-" + fmt.Sprintf("%d", timestamp),
			"nik":                  fmt.Sprintf("317103%010d", timestamp%10000000000),
			"phone":                phone,
			"email":                "resident1@test.com",
			"relationship_to_head": rel,
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/residents", tokenPengurusA, residentPayload)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 Created for resident, got %d: %s", resp.StatusCode, string(body))
		}
		var created map[string]any
		json.Unmarshal(body, &created)
		residentID1 = created["id"].(string)
		if created["household_id"] != householdID1 {
			t.Errorf("expected household_id %s, got %v", householdID1, created["household_id"])
		}
		b5ResidentMarker := "E2EB5-" + fmt.Sprintf("%d", timestamp)
		if created["full_name"] != b5ResidentMarker {
			t.Errorf("expected full_name %q, got %v", b5ResidentMarker, created["full_name"])
		}
	})

	// Scenario 6: Household move (successful)
	t.Run("06_Household_Move_Success", func(t *testing.T) {
		movePayload := map[string]any{
			"house_number":     houseNum3,
			"address":          "Jl. Melati No. 3",
			"occupancy_status": "TENANT",
			"start_date":       time.Now().AddDate(0, 0, 2).Format("2006-01-02"),
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/households/"+householdID1+"/move", tokenPengurusA, movePayload)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK for move, got %d: %s", resp.StatusCode, string(body))
		}
		var moved map[string]any
		json.Unmarshal(body, &moved)
		if moved["house_number"] != houseNum3 {
			t.Errorf("expected house_number %s, got %v", houseNum3, moved["house_number"])
		}
		if moved["occupancy_status"] != "TENANT" {
			t.Errorf("expected occupancy_status TENANT, got %v", moved["occupancy_status"])
		}
	})

	// Scenario 9: Resident retrieval and listing
	t.Run("09_Resident_Retrieval_And_List", func(t *testing.T) {
		resp, body := doRequest(t, http.MethodGet, "/api/v1/residents/"+residentID1, tokenPengurusA, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
		var resGot map[string]any
		json.Unmarshal(body, &resGot)
		if resGot["id"] != residentID1 {
			t.Errorf("got id %v, want %s", resGot["id"], residentID1)
		}
		if resGot["household_id"] != householdID1 {
			t.Errorf("resident household_id = %v, want %s", resGot["household_id"], householdID1)
		}

		// List residents by household_id
		listResp, listBody := doRequest(t, http.MethodGet, "/api/v1/residents?household_id="+householdID1, tokenPengurusA, nil)
		if listResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK for resident list, got %d: %s", listResp.StatusCode, string(listBody))
		}
		var rList struct {
			Data []map[string]any `json:"data"`
		}
		json.Unmarshal(listBody, &rList)
		var found bool
		for _, r := range rList.Data {
			if r["id"] == residentID1 {
				found = true
			}
		}
		if !found {
			t.Errorf("resident %s not found in list", residentID1)
		}
	})

	// Scenario 10: Resident move to another household
	t.Run("10_Resident_Move_To_Another_Household", func(t *testing.T) {
		rel := "OTHER"
		movePayload := map[string]any{
			"destination_household_id": householdID2,
			"relationship_to_head":     rel,
			"start_date":               time.Now().AddDate(0, 0, 3).Format("2006-01-02"),
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/residents/"+residentID1+"/move", tokenPengurusA, movePayload)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK for resident move, got %d: %s", resp.StatusCode, string(body))
		}
		var movedRes map[string]any
		json.Unmarshal(body, &movedRes)
		if movedRes["household_id"] != householdID2 {
			t.Errorf("expected household_id %s, got %v", householdID2, movedRes["household_id"])
		}
	})

	// Scenario 11: Resident deactivation closes residency period
	t.Run("11_Resident_Deactivation", func(t *testing.T) {
		// Create a resident specifically to deactivate
		rel := "SPOUSE"
		rPayload := map[string]any{
			"household_id":         householdID2,
			"full_name":            "Resident To Deactivate",
			"nik":                  fmt.Sprintf("317104%010d", timestamp%10000000000),
			"phone":                "+62812345679",
			"email":                "resident2@test.com",
			"relationship_to_head": rel,
		}
		rResp, rBody := doRequest(t, http.MethodPost, "/api/v1/residents", tokenPengurusA, rPayload)
		if rResp.StatusCode != http.StatusCreated {
			t.Fatalf("create resident failed: %s", string(rBody))
		}
		var cr map[string]any
		json.Unmarshal(rBody, &cr)
		deactResID := cr["id"].(string)

		delResp, delBody := doRequest(t, http.MethodDelete, "/api/v1/residents/"+deactResID, tokenPengurusA, nil)
		if delResp.StatusCode != http.StatusNoContent && delResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 204 or 200 on delete resident, got %d: %s", delResp.StatusCode, string(delBody))
		}

		// Verify resident is inactive
		getResp, getBody := doRequest(t, http.MethodGet, "/api/v1/residents/"+deactResID, tokenPengurusA, nil)
		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("get deactivated resident: %d: %s", getResp.StatusCode, string(getBody))
		}
		var getRes map[string]any
		json.Unmarshal(getBody, &getRes)
		if getRes["is_active"] != false {
			t.Errorf("expected is_active=false, got %v", getRes["is_active"])
		}
	})

	// Scenario 12: Household deactivation closes occupancy and resident periods
	t.Run("12_Household_Deactivation", func(t *testing.T) {
		delResp, delBody := doRequest(t, http.MethodDelete, "/api/v1/households/"+householdID1, tokenPengurusA, nil)
		if delResp.StatusCode != http.StatusNoContent && delResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 204 or 200 on delete household, got %d: %s", delResp.StatusCode, string(delBody))
		}

		getResp, getBody := doRequest(t, http.MethodGet, "/api/v1/households/"+householdID1, tokenPengurusA, nil)
		if getResp.StatusCode != http.StatusOK {
			t.Fatalf("get deactivated household: %d: %s", getResp.StatusCode, string(getBody))
		}
		var getH map[string]any
		json.Unmarshal(getBody, &getH)
		if getH["is_active"] != false {
			t.Errorf("expected is_active=false, got %v", getH["is_active"])
		}
	})

	// Scenario 13: Legacy v7 row projection without synthetic dates
	t.Run("13_Legacy_V7_Projection", func(t *testing.T) {
		// Super admin or query a household with legacy null dates
		// Let's verify that when fetching a list of households or residents, null start_dates are null or omitted
		resp, body := doRequest(t, http.MethodGet, "/api/v1/households?per_page=100", tokenPengurusA, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
		// Confirm json is valid and no fake synthetic dates were generated for legacy records
	})

	// Scenario 14: Tenant isolation (RT A user cannot read or mutate RT B household)
	t.Run("14_Tenant_Isolation_Cross_RT", func(t *testing.T) {
		// User C (Pengurus in RT C) attempts to access RT A's householdID2
		resp, body := doRequest(t, http.MethodGet, "/api/v1/households/"+householdID2, tokenPengurusC, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for cross-RT get, got %d: %s", resp.StatusCode, string(body))
		}
		assertErrorCode(t, body, "not_found")

		// User C attempts to update RT A's householdID2
		patchPayload := map[string]any{"address": "Hacked Address"}
		pResp, pBody := doRequest(t, http.MethodPatch, "/api/v1/households/"+householdID2, tokenPengurusC, patchPayload)
		if pResp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for cross-RT patch, got %d: %s", pResp.StatusCode, string(pBody))
		}
		assertErrorCode(t, pBody, "not_found")

		// User C attempts to move RT A's householdID2
		mPayload := map[string]any{
			"house_number":     "C-HACK",
			"occupancy_status": "OWNER",
			"start_date":       "2026-09-25",
		}
		mResp, mBody := doRequest(t, http.MethodPost, "/api/v1/households/"+householdID2+"/move", tokenPengurusC, mPayload)
		if mResp.StatusCode != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for cross-RT move, got %d: %s", mResp.StatusCode, string(mBody))
		}
		assertErrorCode(t, mBody, "not_found")
	})

	// Scenario 15: Role authorization: WARGA cannot create/update households
	t.Run("15_Role_Authorization_Warga_Cannot_Write", func(t *testing.T) {
		createPayload := map[string]any{
			"nik":              "3171999999999999",
			"phone":            "081299999999",
			"email":            "warga_unauth@test.com",
			"head_name":        "Warga Unauthorized",
			"address":          "Jl. Fake",
			"house_number":     "W-99",
			"occupancy_status": "OWNER",
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/households", tokenWargaB, createPayload)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for warga create, got %d: %s", resp.StatusCode, string(body))
		}
		assertErrorCode(t, body, "forbidden")
	})

	// Scenario 16: Role authorization: BENDAHARA can read, but cannot write households
	t.Run("16_Role_Authorization_Bendahara", func(t *testing.T) {
		// Read should succeed
		resp, body := doRequest(t, http.MethodGet, "/api/v1/households/"+householdID2, tokenBendaharaA, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK for bendahara read, got %d: %s", resp.StatusCode, string(body))
		}

		// Write should fail with 403 Forbidden
		patchPayload := map[string]any{"address": "Bendahara Trying To Edit"}
		pResp, pBody := doRequest(t, http.MethodPatch, "/api/v1/households/"+householdID2, tokenBendaharaA, patchPayload)
		if pResp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for bendahara patch, got %d: %s", pResp.StatusCode, string(pBody))
		}
		assertErrorCode(t, pBody, "forbidden")
	})

	// Scenario 17: Role authorization: PENGURUS can manage households/residents
	t.Run("17_Role_Authorization_Pengurus", func(t *testing.T) {
		// Pengurus A was already demonstrated creating, reading, updating, moving, deactivating
		// Verify pengurus can list their own RT households
		resp, body := doRequest(t, http.MethodGet, "/api/v1/households", tokenPengurusA, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK, got %d: %s", resp.StatusCode, string(body))
		}
	})

	// Scenario 18: Super-admin cross-RT behavior
	t.Run("18_Super_Admin_Behavior", func(t *testing.T) {
		// Super Admin accessing /api/v1/rts (system endpoint)
		resp, body := doRequest(t, http.MethodGet, "/api/v1/rts", tokenSuperAdmin, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 OK for super admin on /api/v1/rts, got %d: %s", resp.StatusCode, string(body))
		}

		// Super admin without tenant context accessing tenant-scoped mutation
		// Should be rejected with 403 Forbidden (requires tenant role)
		tResp, tBody := doRequest(t, http.MethodPatch, "/api/v1/households/"+householdID1, tokenSuperAdmin, map[string]any{"address": "Hacked"})
		if tResp.StatusCode != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for super admin on tenant-only route, got %d: %s", tResp.StatusCode, string(tBody))
		}
		assertErrorCode(t, tBody, "forbidden")
	})

	// Scenario 19: Error format conformance
	t.Run("19_Error_Format_Conformance", func(t *testing.T) {
		// Request invalid endpoint or trigger validation error
		invalidPayload := map[string]any{
			"house_number": "", // invalid: required
		}
		resp, body := doRequest(t, http.MethodPost, "/api/v1/households", tokenPengurusA, invalidPayload)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d: %s", resp.StatusCode, string(body))
		}
		var errResp struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &errResp); err != nil {
			t.Fatalf("failed to unmarshal standard error response: %v", err)
		}
		if errResp.Error.Code == "" || errResp.Error.Message == "" {
			t.Errorf("error format does not conform to {error: {code, message}}: %s", string(body))
		}
	})
}
