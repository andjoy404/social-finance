package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

var houseSeq uint64 = uint64(time.Now().UnixNano())

// helper to insert a test house, household, occupancy, and bill in a given RT
func createTestBillForRT(t *testing.T, pool *database.Pool, rtID string, dueID string, houseNo string, residentName string, amount string) (string, string, string) {
	t.Helper()
	ctx := context.Background()

	uniqueHouseNo := fmt.Sprintf("%s-%d", houseNo, atomic.AddUint64(&houseSeq, 1))

	// 1. Physical house
	var houseID string
	err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO physical_houses (id, rt_id, house_number, address, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'Test Street', true)
		RETURNING id
	`, rtID, uniqueHouseNo).Scan(&houseID)
	if err != nil {
		t.Fatalf("create physical house: %v", err)
	}

	// 2. Household
	var householdID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO households (id, rt_id, head_name, is_active)
		VALUES (gen_random_uuid(), $1, $2, true)
		RETURNING id
	`, rtID, residentName).Scan(&householdID)
	if err != nil {
		t.Fatalf("create household: %v", err)
	}

	// 3. Occupancy
	var occID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO household_occupancies (id, household_id, physical_house_id, occupancy_status, start_date)
		VALUES (gen_random_uuid(), $1, $2, 'OWNER', '2026-01-01')
		RETURNING id
	`, householdID, houseID).Scan(&occID)
	if err != nil {
		t.Fatalf("create occupancy: %v", err)
	}

	phone := fmt.Sprintf("+628%09d", time.Now().UnixNano()%1000000000)

	// 4. Resident
	var residentID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO residents (id, rt_id, full_name, phone, is_active)
		VALUES (gen_random_uuid(), $1, $2, $3, true)
		RETURNING id
	`, rtID, residentName, phone).Scan(&residentID)
	if err != nil {
		t.Fatalf("create resident: %v", err)
	}

	// 5. Residency period
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO residency_periods (id, resident_id, household_occupancy_id, start_date)
		VALUES (gen_random_uuid(), $1, $2, '2026-01-01')
	`, residentID, occID)
	if err != nil {
		t.Fatalf("create residency period: %v", err)
	}

	// 6. Bill
	var billID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO bills (id, rt_id, household_occupancy_id, due_id, amount, period, due_date, status)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, '2026-10', '2026-10-15', 'unpaid')
		RETURNING id
	`, rtID, occID, dueID, amount).Scan(&billID)
	if err != nil {
		t.Fatalf("create bill: %v", err)
	}

	return billID, occID, residentID
}

func getOrCreateTestDue(t *testing.T, pool *database.Pool, rtID string) string {
	t.Helper()
	ctx := context.Background()
	var dueID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM dues WHERE rt_id = $1 AND is_active = true LIMIT 1`, rtID).Scan(&dueID)
	if err == nil {
		return dueID
	}

	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO dues (id, rt_id, name, amount, period_type, payment_rule, is_active)
		VALUES (gen_random_uuid(), $1, 'Iuran Uji Coba', 50000.00, 'monthly', 'full', true)
		RETURNING id
	`, rtID).Scan(&dueID)
	if err != nil {
		t.Fatalf("create due: %v", err)
	}
	return dueID
}

// Scenario 1: Ketua RT dengan finance.read dan RT yang valid bisa melihat List Iuran Warga
func TestRBAC_KetuaRTCanViewAllBillsInRT(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}

	dueID := getOrCreateTestDue(t, pool, rtID)

	// Create 3 bills for different households in RT 003
	bill1, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("K%d", time.Now().UnixNano()%1000), "Warga A", "50000.00")
	bill2, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("K%d", (time.Now().UnixNano()+1)%1000), "Warga B", "50000.00")
	bill3, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("K%d", (time.Now().UnixNano()+2)%1000), "Warga C", "50000.00")

	// Create a Ketua RT account (role=warga, jabatan=ketua)
	var ketuaUserID, ketuaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Ketua RT 003', NULL, true)
		RETURNING id
	`, fmt.Sprintf("ketua-rbac-%d@local.com", time.Now().UnixNano())).Scan(&ketuaUserID)
	if err != nil {
		t.Fatalf("create ketua user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'ketua', true)
		RETURNING id
	`, ketuaUserID, rtID).Scan(&ketuaMID)
	if err != nil {
		t.Fatalf("create ketua membership: %v", err)
	}

	token := makeTestToken(t, ketuaUserID, ketuaMID, rtID, auth.RoleWarga, auth.JabatanKetua)

	// 1. GET /api/v1/bills -> must return all bills in the RT
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for Ketua RT ListBills, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal bills response: %v", err)
	}

	if resp.Pagination.Total < 3 {
		t.Errorf("expected at least 3 bills in RT, got total %d", resp.Pagination.Total)
	}

	foundMap := make(map[string]bool)
	for _, b := range resp.Data {
		foundMap[b.ID] = true
	}
	if !foundMap[bill1] || !foundMap[bill2] || !foundMap[bill3] {
		t.Errorf("Ketua RT did not receive all bills: foundMap=%v, wanted bill1=%s, bill2=%s, bill3=%s", foundMap, bill1, bill2, bill3)
	}

	// 2. GET /api/v1/bills/{id} on citizen's bill -> must succeed (HTTP 200)
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+bill1, nil)
	reqDetail.Header.Set("Authorization", "Bearer "+token)
	recDetail := httptest.NewRecorder()
	router.ServeHTTP(recDetail, reqDetail)

	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for Ketua RT GetBill on citizen bill, got %d: %s", recDetail.Code, recDetail.Body.String())
	}
}

// Scenario 2: Bendahara RT dengan finance.read dan RT yang valid bisa melihat List Iuran Warga
func TestRBAC_BendaharaRTCanViewAllBillsInRT(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}

	dueID := getOrCreateTestDue(t, pool, rtID)
	bill1, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("B%d", time.Now().UnixNano()%1000), "Warga D", "50000.00")

	// Case 2A: Bendahara with role=warga and jabatan=bendahara
	var benUserID, benMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Bendahara Warga', NULL, true)
		RETURNING id
	`, fmt.Sprintf("ben-warga-%d@local.com", time.Now().UnixNano())).Scan(&benUserID)
	if err != nil {
		t.Fatalf("create bendahara user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'bendahara', true)
		RETURNING id
	`, benUserID, rtID).Scan(&benMID)
	if err != nil {
		t.Fatalf("create bendahara membership: %v", err)
	}

	token := makeTestToken(t, benUserID, benMID, rtID, auth.RoleWarga, auth.JabatanBendahara)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for Bendahara RT (warga+bendahara), got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal bills response: %v", err)
	}
	if resp.Pagination.Total < 1 {
		t.Errorf("expected at least 1 bill, got %d", resp.Pagination.Total)
	}

	// Can view detail of citizen's bill
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+bill1, nil)
	reqDetail.Header.Set("Authorization", "Bearer "+token)
	recDetail := httptest.NewRecorder()
	router.ServeHTTP(recDetail, reqDetail)
	if recDetail.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for Bendahara GetBill, got %d: %s", recDetail.Code, recDetail.Body.String())
	}
}

// Scenario 3: Warga biasa tanpa finance.read tidak mendapat akses pengurus ke daftar iuran
func TestRBAC_WargaBiasaCannotViewAllBillsInRT(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}

	dueID := getOrCreateTestDue(t, pool, rtID)

	// Create Bill 1 for Citizen 1
	billCitizen1, occID1, residentID1 := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("W%d", time.Now().UnixNano()%1000), "Citizen One", "50000.00")
	// Create Bill 2 for Citizen 2
	billCitizen2, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("W%d", (time.Now().UnixNano()+1)%1000), "Citizen Two", "50000.00")

	// Create user for Citizen 1 (role=warga, jabatan=empty) linked to resident 1
	var wargaUserID, wargaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Citizen One User', NULL, true)
		RETURNING id
	`, fmt.Sprintf("citizen1-%d@local.com", time.Now().UnixNano())).Scan(&wargaUserID)
	if err != nil {
		t.Fatalf("create warga user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', NULL, true)
		RETURNING id
	`, wargaUserID, rtID).Scan(&wargaMID)
	if err != nil {
		t.Fatalf("create warga membership: %v", err)
	}

	// Link resident 1 to user 1
	_, err = pool.Raw().ExecContext(ctx, `UPDATE residents SET user_id = $1 WHERE id = $2`, wargaUserID, residentID1)
	if err != nil {
		t.Fatalf("link resident to user: %v", err)
	}

	token := makeTestToken(t, wargaUserID, wargaMID, rtID, auth.RoleWarga, "")

	// 1. Calling GET /api/v1/bills: warga biasa must ONLY get their own bill (billCitizen1), NEVER billCitizen2
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for Warga ListBills, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data []struct {
			ID                   string `json:"id"`
			HouseholdOccupancyID string `json:"household_occupancy_id"`
		} `json:"data"`
		Pagination struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal bills response: %v", err)
	}

	for _, b := range resp.Data {
		if b.ID == billCitizen2 {
			t.Fatalf("SECURITY VIOLATION: ordinary citizen received other citizen's bill %s", billCitizen2)
		}
		if b.HouseholdOccupancyID != occID1 {
			t.Fatalf("SECURITY VIOLATION: ordinary citizen received bill for different occupancy %s", b.HouseholdOccupancyID)
		}
	}

	// 2. Attempting to view Citizen 2's bill directly via GET /api/v1/bills/{id} must return 404
	reqOther := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billCitizen2, nil)
	reqOther.Header.Set("Authorization", "Bearer "+token)
	recOther := httptest.NewRecorder()
	router.ServeHTTP(recOther, reqOther)

	if recOther.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for citizen attempting to view another resident's bill, got %d: %s", recOther.Code, recOther.Body.String())
	}

	// 3. Citizen 1 can view their own bill
	reqOwn := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billCitizen1, nil)
	reqOwn.Header.Set("Authorization", "Bearer "+token)
	recOwn := httptest.NewRecorder()
	router.ServeHTTP(recOwn, reqOwn)

	if recOwn.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for citizen viewing their own bill, got %d: %s", recOwn.Code, recOwn.Body.String())
	}
}

// Scenario 4: Pengurus tanpa konteks RT yang valid ditolak
func TestRBAC_PengurusWithoutValidRTContextDenied(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	// Insert valid user in DB
	var uID string
	err := pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'User Without RT', NULL, true)
		RETURNING id
	`, fmt.Sprintf("no-rt-%d@local.com", time.Now().UnixNano())).Scan(&uID)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Token with empty RTID
	tokenNoRT, err := auth.GenerateAccessToken(auth.TokenClaims{
		UserID:  uID,
		MID:     "",
		RTID:    "",
		Role:    string(auth.RolePengurus),
		Jabatan: string(auth.JabatanKetua),
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+tokenNoRT)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Must be rejected with 401 or 403
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 or 403 for user without RT context, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Scenario 5: Pengurus RT lain tidak dapat membaca data iuran di luar kewenangannya
func TestRBAC_PengurusOtherRTCannotReadOtherRTBills(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rt003ID, rt002ID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rt003ID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '002' LIMIT 1`).Scan(&rt002ID)
	if err != nil {
		t.Fatalf("get rt 002: %v", err)
	}

	due3 := getOrCreateTestDue(t, pool, rt003ID)
	billRT003, _, _ := createTestBillForRT(t, pool, rt003ID, due3, fmt.Sprintf("X%d", time.Now().UnixNano()%1000), "Warga RT003", "50000.00")

	// Create Ketua RT in RT 002
	var ketua2User, ketua2MID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Ketua RT 002 User', NULL, true)
		RETURNING id
	`, fmt.Sprintf("ketua-rt2-%d@local.com", time.Now().UnixNano())).Scan(&ketua2User)
	if err != nil {
		t.Fatalf("create ketua 2 user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'ketua', true)
		RETURNING id
	`, ketua2User, rt002ID).Scan(&ketua2MID)
	if err != nil {
		t.Fatalf("create ketua 2 membership: %v", err)
	}

	token2 := makeTestToken(t, ketua2User, ketua2MID, rt002ID, auth.RoleWarga, auth.JabatanKetua)

	// 1. ListBills by RT 002 Ketua must NOT contain bill from RT 003
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	reqList.Header.Set("Authorization", "Bearer "+token2)
	recList := httptest.NewRecorder()
	router.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for RT 002 ListBills, got %d: %s", recList.Code, recList.Body.String())
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(recList.Body.Bytes(), &resp)
	for _, b := range resp.Data {
		if b.ID == billRT003 {
			t.Fatalf("SECURITY VIOLATION: RT 002 Ketua received bill from RT 003: %s", billRT003)
		}
	}

	// 2. GET /api/v1/bills/{billRT003} by RT 002 Ketua must return 404 (Not Found within tenant scope)
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/bills/"+billRT003, nil)
	reqGet.Header.Set("Authorization", "Bearer "+token2)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusNotFound {
		t.Fatalf("expected HTTP 404 for cross-tenant bill access, got %d: %s", recGet.Code, recGet.Body.String())
	}
}

// Scenario 6: Jabatan tanpa finance.read ditolak sesuai kebijakan RBAC
func TestRBAC_PositionWithoutFinanceReadDenied(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}

	// Create user with jabatan=keamanan (keamanan has NO finance.read permission)
	var secUser, secMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Petugas Keamanan', NULL, true)
		RETURNING id
	`, fmt.Sprintf("keamanan-rbac-%d@local.com", time.Now().UnixNano())).Scan(&secUser)
	if err != nil {
		t.Fatalf("create keamanan user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'keamanan', true)
		RETURNING id
	`, secUser, rtID).Scan(&secMID)
	if err != nil {
		t.Fatalf("create keamanan membership: %v", err)
	}

	token := makeTestToken(t, secUser, secMID, rtID, auth.RoleWarga, "keamanan")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Must be forbidden (HTTP 403) by RequirePermission("finance.read")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 for user with position lacking finance.read, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Scenario 7: Permission baca tanpa permission approve tidak otomatis dapat menyetujui pembayaran
func TestRBAC_FinanceReadCannotApproveWithoutFinanceApprove(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '003' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt 003: %v", err)
	}

	dueID := getOrCreateTestDue(t, pool, rtID)
	billID, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("Y%d", time.Now().UnixNano()%1000), "Warga Approve Test", "50000.00")

	// Create pending payment for bill
	var paymentID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO payments (id, rt_id, bill_id, due_id, amount, method, origin, status)
		VALUES (gen_random_uuid(), $1, $2, NULL, 50000.00, 'TRANSFER', 'SELF_SUBMITTED', 'PENDING')
		RETURNING id
	`, rtID, billID).Scan(&paymentID)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// Create a user with jabatan=wakil_ketua (wakil_ketua HAS finance.read, but NO finance.approve)
	var wkUser, wkMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Wakil Ketua', NULL, true)
		RETURNING id
	`, fmt.Sprintf("wakil-ketua-%d@local.com", time.Now().UnixNano())).Scan(&wkUser)
	if err != nil {
		t.Fatalf("create wakil ketua user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', 'wakil_ketua', true)
		RETURNING id
	`, wkUser, rtID).Scan(&wkMID)
	if err != nil {
		t.Fatalf("create wakil ketua membership: %v", err)
	}

	token := makeTestToken(t, wkUser, wkMID, rtID, auth.RoleWarga, "wakil_ketua")

	// 1. wakil_ketua CAN read bills (HTTP 200)
	reqRead := httptest.NewRequest(http.MethodGet, "/api/v1/bills", nil)
	reqRead.Header.Set("Authorization", "Bearer "+token)
	recRead := httptest.NewRecorder()
	router.ServeHTTP(recRead, reqRead)
	if recRead.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for wakil_ketua reading bills, got %d: %s", recRead.Code, recRead.Body.String())
	}

	// 2. wakil_ketua CANNOT approve payment (HTTP 403 Forbidden)
	verifyBody := bytes.NewBufferString(`{"action":"APPROVED"}`)
	reqApprove := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", verifyBody)
	reqApprove.Header.Set("Authorization", "Bearer "+token)
	reqApprove.Header.Set("Content-Type", "application/json")
	recApprove := httptest.NewRecorder()
	router.ServeHTTP(recApprove, reqApprove)

	if recApprove.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 for wakil_ketua attempting to approve payment without finance.approve, got %d: %s", recApprove.Code, recApprove.Body.String())
	}
}

// Scenario 8: Akun Warga biasa ditolak HTTP 403 saat mencoba memverifikasi pembayaran
func TestRBAC_WargaCannotVerifyPayment(t *testing.T) {
	pool, router := setupRouterTest(t)
	ctx := context.Background()

	var rtID string
	err := pool.Raw().QueryRowContext(ctx, `SELECT id FROM rts WHERE rt = '002' LIMIT 1`).Scan(&rtID)
	if err != nil {
		t.Fatalf("get rt 002: %v", err)
	}

	dueID := getOrCreateTestDue(t, pool, rtID)
	billID, _, _ := createTestBillForRT(t, pool, rtID, dueID, fmt.Sprintf("W%d", time.Now().UnixNano()%1000), "Warga Verify Check", "50000.00")

	// Create pending payment
	var paymentID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO payments (id, rt_id, bill_id, due_id, amount, method, origin, status)
		VALUES (gen_random_uuid(), $1, $2, NULL, 50000.00, 'TRANSFER', 'SELF_SUBMITTED', 'PENDING')
		RETURNING id
	`, rtID, billID).Scan(&paymentID)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}

	// Create ordinary warga user
	var wargaUser, wargaMID string
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, system_role, is_active)
		VALUES (gen_random_uuid(), $1, 'dummy', 'Warga Biasa', NULL, true)
		RETURNING id
	`, fmt.Sprintf("warga-verify-%d@local.com", time.Now().UnixNano())).Scan(&wargaUser)
	if err != nil {
		t.Fatalf("create warga user: %v", err)
	}
	err = pool.Raw().QueryRowContext(ctx, `
		INSERT INTO user_rt_memberships (id, user_id, rt_id, role, jabatan, is_active)
		VALUES (gen_random_uuid(), $1, $2, 'warga', NULL, true)
		RETURNING id
	`, wargaUser, rtID).Scan(&wargaMID)
	if err != nil {
		t.Fatalf("create warga membership: %v", err)
	}

	token := makeTestToken(t, wargaUser, wargaMID, rtID, auth.RoleWarga, "")

	// Warga attempts to verify payment (approve) -> MUST be 403 Forbidden
	verifyBody := bytes.NewBufferString(`{"action":"APPROVED"}`)
	reqVerify := httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+paymentID+"/verify", verifyBody)
	reqVerify.Header.Set("Authorization", "Bearer "+token)
	reqVerify.Header.Set("Content-Type", "application/json")
	recVerify := httptest.NewRecorder()
	router.ServeHTTP(recVerify, reqVerify)

	if recVerify.Code != http.StatusForbidden {
		t.Fatalf("expected HTTP 403 Forbidden for warga trying to verify payment, got %d: %s", recVerify.Code, recVerify.Body.String())
	}
}
