package finance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiMonth_IssuanceLimitOncePerYearPerRT(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID1, tokenBendahara1, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID1)

	rtID2, tokenBendahara2, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID2)

	// 1. First issuance for year 2026 in RT 1 -> MUST SUCCEED
	body1 := `{"name":"IPL 2026 RT1","amount":"50000","period_type":"monthly","due_type":"multi_month","year":2026}`
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body1))
	req1.Header.Set("Authorization", "Bearer "+tokenBendahara1)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	var due1 Due
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &due1))
	require.Equal(t, "multi_month", due1.DueType)
	require.Equal(t, 2026, *due1.Year)

	// 2. Second issuance for year 2026 in the SAME RT 1 -> MUST FAIL with specific error message
	body2 := `{"name":"IPL 2026 Duplikat","amount":"55000","period_type":"monthly","due_type":"multi_month","year":2026}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body2))
	req2.Header.Set("Authorization", "Bearer "+tokenBendahara1)
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	require.Contains(t, rec2.Body.String(), "IPL Multi-Bulan untuk tahun 2026 sudah pernah diterbitkan di RT ini.")

	// 3. Issuance for year 2027 in the SAME RT 1 -> MUST SUCCEED (different calendar year)
	body3 := `{"name":"IPL 2027 RT1","amount":"60000","period_type":"monthly","due_type":"multi_month","year":2027}`
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body3))
	req3.Header.Set("Authorization", "Bearer "+tokenBendahara1)
	req3.Header.Set("Content-Type", "application/json")
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	require.Equal(t, http.StatusCreated, rec3.Code)

	// 4. Issuance for year 2026 in a DIFFERENT RT 2 -> MUST SUCCEED (tenant isolation)
	body4 := `{"name":"IPL 2026 RT2","amount":"50000","period_type":"monthly","due_type":"multi_month","year":2026}`
	req4 := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body4))
	req4.Header.Set("Authorization", "Bearer "+tokenBendahara2)
	req4.Header.Set("Content-Type", "application/json")
	rec4 := httptest.NewRecorder()
	router.ServeHTTP(rec4, req4)
	require.Equal(t, http.StatusCreated, rec4.Code)
}

func TestMultiMonth_ConcurrentIssuanceRace(t *testing.T) {
	pool := testPool(t)
	defer pool.Close()
	router := setupFinanceRouter(pool)

	rtID, tokenBendahara, _ := setupValidationRT(t, pool)
	defer cleanupTestRT(t, pool, rtID)

	year := 2028
	concurrency := 4
	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make([]int, concurrency)
	bodies := make([]string, concurrency)

	for i := 0; i < concurrency; i++ {
		idx := i
		go func() {
			defer wg.Done()
			body := fmt.Sprintf(`{"name":"IPL %d Thread %d","amount":"50000","period_type":"monthly","due_type":"multi_month","year":%d}`, year, idx, year)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/dues", bytes.NewBufferString(body))
			req.Header.Set("Authorization", "Bearer "+tokenBendahara)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			results[idx] = rec.Code
			bodies[idx] = rec.Body.String()
		}()
	}

	wg.Wait()

	successCount := 0
	failureCount := 0
	for i := 0; i < concurrency; i++ {
		if results[i] == http.StatusCreated {
			successCount++
		} else if results[i] == http.StatusBadRequest {
			failureCount++
			require.Contains(t, bodies[i], fmt.Sprintf("IPL Multi-Bulan untuk tahun %d sudah pernah diterbitkan di RT ini.", year))
		}
	}

	require.Equal(t, 1, successCount, "Exactly 1 concurrent request should succeed")
	require.Equal(t, concurrency-1, failureCount, "All other concurrent requests should fail")
}

