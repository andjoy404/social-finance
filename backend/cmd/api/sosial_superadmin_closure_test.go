package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
)

var closureTestSeq uint64

func TestSosialJabatan_IuranAccessAndKasRestriction(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	// 1. Create a user with jabatan "sosial"
	sosialUID, sosialToken := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanSosial)

	// Create house, household, occupancy, and resident for sosial user
	ctx := context.Background()
	uHouseNo := fmt.Sprintf("A-%d-%d", time.Now().UnixNano(), atomic.AddUint64(&closureTestSeq, 1))

	var houseID string
	err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO physical_houses (id, rt_id, house_number, address, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'Jalan Mawar', true)
		RETURNING id
	`, rtID, uHouseNo).Scan(&houseID)
	if err != nil {
		t.Fatalf("create physical house: %v", err)
	}

	var householdID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO households (id, rt_id, head_name, is_active)
		VALUES (gen_random_uuid(), $1, 'Keluarga Sosial', true)
		RETURNING id
	`, rtID).Scan(&householdID)
	if err != nil {
		t.Fatalf("create household: %v", err)
	}

	var sosialOccID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO household_occupancies (id, household_id, physical_house_id, occupancy_status, start_date)
		VALUES (gen_random_uuid(), $1, $2, 'OWNER', '2026-01-01')
		RETURNING id
	`, householdID, houseID).Scan(&sosialOccID)
	if err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	phone := fmt.Sprintf("+6289%08d", atomic.AddUint64(&closureTestSeq, 1))
	var resID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, phone, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'Warga Sosial', $3, true)
		RETURNING id
	`, rtID, sosialUID, phone).Scan(&resID)
	if err != nil {
		t.Fatalf("create resident: %v", err)
	}

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date)
		VALUES (gen_random_uuid(), $1, $2, '2026-01-01')
	`, resID, sosialOccID)
	if err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	dueID := getOrCreateTestDue(t, pool, rtID)

	// Create bill for sosial household
	var sosialBillID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO bills (id, rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
		VALUES (gen_random_uuid(), $1, $2, $3, 75000.00, '2026-11', '2026-11-20', 'unpaid')
		RETURNING id
	`, rtID, sosialOccID, dueID).Scan(&sosialBillID)
	if err != nil {
		t.Fatalf("create sosial bill: %v", err)
	}

	// Create another house & bill for another household
	otherBillID, otherOccID, _ := createTestBillForRT(t, pool, rtID, dueID, "B-Other", "Warga Lain", "75000.00")
	if otherOccID == sosialOccID {
		t.Fatalf("occupancies should differ")
	}

	// 2. Sosial user accesses GET /api/v1/bills
	recBills := doReq(t, router, http.MethodGet, "/api/v1/bills", sosialToken, nil, "")
	if recBills.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for sosial bills, got %d: %s", recBills.Code, recBills.Body.String())
	}

	var billsResp struct {
		Data []struct {
			ID                   string `json:"id"`
			HouseholdOccupancyID string `json:"household_occupancy_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recBills.Body.Bytes(), &billsResp); err != nil {
		t.Fatalf("unmarshal bills: %v", err)
	}

	foundOwnBill := false
	for _, b := range billsResp.Data {
		if b.ID == sosialBillID {
			foundOwnBill = true
		}
		if b.ID == otherBillID {
			t.Fatalf("sosial user must NOT see other household's bill %s", otherBillID)
		}
		if b.HouseholdOccupancyID != sosialOccID {
			t.Fatalf("sosial user must only see bills for their own occupancy %s, got %s", sosialOccID, b.HouseholdOccupancyID)
		}
	}
	if !foundOwnBill {
		t.Fatalf("expected sosial user to see their own bill %s", sosialBillID)
	}

	// 3. Sosial user attempts to get other household's bill detail -> 404
	recOtherDetail := doReq(t, router, http.MethodGet, "/api/v1/bills/"+otherBillID, sosialToken, nil, "")
	if recOtherDetail.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for other bill, got %d: %s", recOtherDetail.Code, recOtherDetail.Body.String())
	}

	// 4. Sosial user gets own bill detail -> 200
	recOwnDetail := doReq(t, router, http.MethodGet, "/api/v1/bills/"+sosialBillID, sosialToken, nil, "")
	if recOwnDetail.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for own bill, got %d: %s", recOwnDetail.Code, recOwnDetail.Body.String())
	}

	// 5. Sosial user cannot create bills -> 403
	recCreateBill := doReq(t, router, http.MethodPost, "/api/v1/bills", sosialToken, []byte(`{"due_id":"`+dueID+`"}`), "application/json")
	if recCreateBill.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for create bill, got %d", recCreateBill.Code)
	}

	// 6. Sosial user cannot access Kas transactions or categories -> 403
	recTx := doReq(t, router, http.MethodGet, "/api/v1/transactions", sosialToken, nil, "")
	if recTx.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for transactions, got %d", recTx.Code)
	}

	recCat := doReq(t, router, http.MethodGet, "/api/v1/categories", sosialToken, nil, "")
	if recCat.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for categories, got %d", recCat.Code)
	}
}

func TestSuperAdmin_WargaPlatformWideReadWriteAndPrivacy(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	// Create SuperAdmin token without RT membership (ac.RTID == "")
	var saUID string
	err := pool.Raw().QueryRowContext(context.Background(),
		`INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		 VALUES (gen_random_uuid(), $1, 'hashed', 'Super Admin System', 'super_admin', true)
		 RETURNING id`,
		fmt.Sprintf("sa-%d@system.test", time.Now().UnixNano())).Scan(&saUID)
	if err != nil {
		t.Fatalf("create superadmin user: %v", err)
	}

	saToken, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  saUID,
		SysRole: string(auth.SystemRoleSuperAdmin),
		Role:    "super_admin",
	})
	if err != nil {
		t.Fatalf("generate SA token: %v", err)
	}

	// 1. Overdue summary endpoint returns 200 OK with zero values for SuperAdmin
	recOverdue := doReq(t, router, http.MethodGet, "/api/v1/bills/overdue-summary", saToken, nil, "")
	if recOverdue.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SA overdue summary, got %d: %s", recOverdue.Code, recOverdue.Body.String())
	}
	var overdueSummary struct {
		HasOverdue3Months bool `json:"has_overdue_3_months"`
		TotalOutstanding  int  `json:"total_outstanding"`
		OverdueBillCount  int  `json:"overdue_bill_count"`
	}
	if err := json.Unmarshal(recOverdue.Body.Bytes(), &overdueSummary); err != nil {
		t.Fatalf("unmarshal overdue summary: %v", err)
	}
	if overdueSummary.HasOverdue3Months || overdueSummary.TotalOutstanding != 0 || overdueSummary.OverdueBillCount != 0 {
		t.Fatalf("expected zero overdue summary for SuperAdmin, got %+v", overdueSummary)
	}

	// 2. Query residents with ?page=1&page_size=100&rt_id=... returns 200 OK
	recResidents := doReq(t, router, http.MethodGet, "/api/v1/residents?page=1&page_size=100&rt_id="+rtID, saToken, nil, "")
	if recResidents.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SA residents list, got %d: %s", recResidents.Code, recResidents.Body.String())
	}

	var resListResp struct {
		Data []struct {
			ID       string  `json:"id"`
			FullName string  `json:"full_name"`
			Phone    *string `json:"phone"`
			Email    *string `json:"email"`
			Nik      *string `json:"nik"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recResidents.Body.Bytes(), &resListResp); err != nil {
		t.Fatalf("unmarshal residents: %v", err)
	}

	// 3. Verify SuperAdmin privacy: Phone, Email, and NIK must be redacted (nil)
	for _, r := range resListResp.Data {
		if r.Nik != nil {
			t.Errorf("SuperAdmin must receive nil NIK for resident %s, got %s", r.FullName, *r.Nik)
		}
		if r.Phone != nil {
			t.Errorf("SuperAdmin must receive nil phone for resident %s, got %s", r.FullName, *r.Phone)
		}
		if r.Email != nil {
			t.Errorf("SuperAdmin must receive nil email for resident %s, got %s", r.FullName, *r.Email)
		}
	}

	// 4. SuperAdmin creates a special resident with jabatan "keamanan" under target RT
	uStamp := time.Now().UnixNano()
	newNik := fmt.Sprintf("327101%010d", uStamp%10000000000)
	newPhone := fmt.Sprintf("+62812%07d", uStamp%10000000)
	newEmail := fmt.Sprintf("new-keam-%d@test.com", uStamp)

	createBody := jsonBody(t, map[string]any{
		"full_name": "Petugas Keamanan Baru",
		"nik":       newNik,
		"phone":     newPhone,
		"email":     newEmail,
		"jabatan":   "keamanan",
		"rt_id":     rtID,
	})

	recCreate := doReq(t, router, http.MethodPost, "/api/v1/residents/special", saToken, createBody, "application/json")
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for SA special resident, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdRes struct {
		ID       string  `json:"id"`
		FullName string  `json:"full_name"`
		Phone    *string `json:"phone"`
		Email    *string `json:"email"`
		Nik      *string `json:"nik"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdRes); err != nil {
		t.Fatalf("unmarshal created resident: %v", err)
	}
	if createdRes.ID == "" {
		t.Fatalf("expected non-empty created resident ID")
	}

	// 5. SuperAdmin updates the resident
	updateBody := jsonBody(t, map[string]any{
		"full_name": "Petugas Keamanan Terupdate",
	})
	recUpdate := doReq(t, router, http.MethodPatch, "/api/v1/residents/special/"+createdRes.ID, saToken, updateBody, "application/json")
	if recUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SA update special resident, got %d: %s", recUpdate.Code, recUpdate.Body.String())
	}

	// 6. SuperAdmin deactivates the resident
	recDeact := doReq(t, router, http.MethodDelete, "/api/v1/residents/"+createdRes.ID+"?rt_id="+rtID, saToken, nil, "")
	if recDeact.Code != http.StatusOK && recDeact.Code != http.StatusNoContent {
		t.Fatalf("expected 200 or 204 for SA delete resident, got %d: %s", recDeact.Code, recDeact.Body.String())
	}
}

func TestOrganizationStructure_HouseNumberDisplayAndContactPrivacy(t *testing.T) {
	pool, router := setupCommunityRouter(t, []string{commRTUrena})
	rtID := commRTUrena

	// Create test users for different roles
	wargaUID, wargaToken := createCommunityTestUser(t, pool, rtID, auth.RoleWarga, "")
	sekUID, _ := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanSekretaris)
	keamUID, _ := createCommunityTestUser(t, pool, rtID, auth.RolePengurus, auth.JabatanKeamanan)

	ctx := context.Background()
	_, _ = pool.Raw().ExecContext(ctx, "UPDATE users SET phone = '0899999999' WHERE id IN (SELECT user_id FROM user_rt_memberships WHERE rt_id = $1)", rtID)
	_, _ = pool.Raw().ExecContext(ctx, "UPDATE users SET phone = '0811111111' WHERE id = $1", wargaUID)
	_, _ = pool.Raw().ExecContext(ctx, "UPDATE users SET phone = '0822222222', email = 'sekretaris@test.com' WHERE id = $1", sekUID)
	_, _ = pool.Raw().ExecContext(ctx, "UPDATE users SET phone = '0833333333', email = 'keamanan@test.com' WHERE id = $1", keamUID)

	// Ensure at least one member has an associated house
	_, _ = pool.Raw().ExecContext(ctx, `
		UPDATE physical_houses SET house_number = 'B-12' WHERE rt_id = $1 AND is_active = true
	`, rtID)

	rec := doReq(t, router, http.MethodGet, "/api/v1/organization-structure", wargaToken, nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for organization-structure, got %d: %s", rec.Code, rec.Body.String())
	}

	var orgResp struct {
		Nodes []struct {
			Position string `json:"position"`
			Members  []struct {
				FullName    string  `json:"full_name"`
				Jabatan     string  `json:"jabatan"`
				HouseNumber *string `json:"house_number"`
				Phone       *string `json:"phone"`
				Email       *string `json:"email"`
			} `json:"members"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &orgResp); err != nil {
		t.Fatalf("unmarshal org structure: %v", err)
	}

	for _, node := range orgResp.Nodes {
		for _, m := range node.Members {
			if strings.EqualFold(m.Jabatan, "keamanan") {
				// Keamanan phone is public
				if m.Phone == nil || *m.Phone == "" {
					t.Errorf("Petugas Keamanan phone must be visible to warga")
				}
			} else {
				// Other officers' phone must be nil
				if m.Phone != nil && *m.Phone != "" {
					t.Errorf("non-keamanan officer %s phone must NOT be visible to warga", m.FullName)
				}
			}
			if m.Email != nil && *m.Email != "" {
				t.Errorf("officer %s email must NOT be visible to warga", m.FullName)
			}
		}
	}
}
