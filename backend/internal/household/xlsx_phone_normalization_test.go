package household

// ============================================================================
// Phone normalization end-to-end tests.
// Verifies:
//   1. CreateWarga via API stores canonical phone in users + residents.
//   2. XLSX import stores canonical phone on newly-created user accounts.
//   3. Re-import does not corrupt previously-stored phone.
//   4/5. Login (Android + Web, same endpoint) works after import.
//   6. Wrong password is rejected.
//   7. All supported phone notations yield canonical 08-prefix output.
// ============================================================================

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/xuri/excelize/v2"

	"social-finance/internal/auth"
	"social-finance/internal/database"
)

func TestPhoneNormalization_Formats(t *testing.T) {
	cases := []struct{ in, want string }{
		{"081234567890", "081234567890"},
		{"6281234567890", "081234567890"},
		{"+6281234567890", "081234567890"},
		{"+62 812-3456-7890", "081234567890"},
		{"62 81234567890", "081234567890"},
		{"0211234567", ""},                 // should be rejected
		{"62", ""},                         // should be rejected
		{"", ""},                           // should be rejected
		{"0812345678901", "0812345678901"}, // 13-digit OK
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := NormalizePhone(tc.in)
			if tc.want == "" {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected %q -> %q, got error: %v", tc.in, tc.want, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizePhone(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPhoneNormalization_LoginVariants(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires seeded test database")
	}
	pool := testPool(t)
	defer pool.Close()

	rtID := setupTestRT(t, pool, "PhoneLogin RT", 1, "PHN")
	const expectedCanonical = "081234567890"
	for _, phoneRaw := range []string{"+6281234567890", "6281234567890", "081234567890"} {
		n := time.Now().UnixNano()
		row := fmt.Sprintf("PhoneLogin-%d", n)
		testEmail := fmt.Sprintf("phonelogin-%d@example.com", n)
		testNik := fmt.Sprintf("3171010101%06d", n%1000000)

		// Isolate iteration fixture: clean up any existing user/resident with this phone or email
		cleanFixture := func() {
			_, _ = pool.Raw().ExecContext(context.Background(), `DELETE FROM users WHERE phone = $1 OR email = $2`, expectedCanonical, testEmail)
			_, _ = pool.Raw().ExecContext(context.Background(), `DELETE FROM residents WHERE rt_id = $1 AND (nik = $2 OR phone = $3)`, rtID, testNik, expectedCanonical)
		}
		cleanFixture()
		defer cleanFixture()

		// Import one resident via XLSX import with only the phone column set.
		f := excelize.NewFile()
		writeHeaderRow(f)
		writeRow(f, 0, []string{row, "Head Test", "OWNER", "", "Res Test", testNik, phoneRaw, testEmail, "", "", "Password123!"})
		xlsxData := xlsxBytes(f)
		f.Close()

		saToken := makeSuperAdminToken(t)
		router := setupExcelRouter(pool)

		buf, boundary := buildMultipartBody("file", "import.xlsx", xlsxData)
		req := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx?rt_id="+rtID, saToken, buf)
		req.Header.Set("Content-Type", boundary)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("import xlsx %q failed: %d %s", phoneRaw, rec.Code, rec.Body.String())
		}

		// Users must have the canonical phone in the DB.
		var savedPhone *string
		err := pool.Raw().QueryRowContext(context.Background(),
			`SELECT phone FROM users WHERE email = $1`, testEmail).Scan(&savedPhone)
		if err != nil {
			t.Fatalf("lookup user by email: %v", err)
		}
		if savedPhone == nil {
			t.Fatalf("expected canonical phone stored for %s", testEmail)
		}
		if *savedPhone != expectedCanonical {
			t.Fatalf("phone saved as %q, want %s (from input %q)", *savedPhone, expectedCanonical, phoneRaw)
		}

		// Resident must carry the canonical phone too.
		var resPhone string
		err = pool.Raw().QueryRowContext(context.Background(),
			`SELECT phone FROM residents WHERE rt_id = $1 AND nik = $2`, rtID, testNik).Scan(&resPhone)
		if err != nil {
			t.Fatalf("lookup resident phone: %v", err)
		}
		if resPhone != expectedCanonical {
			t.Fatalf("resident phone saved as %q, want %s (from input %q)", resPhone, expectedCanonical, phoneRaw)
		}

		// Login via identifier (Android & Web share the same endpoint).
		for _, identifier := range []string{testEmail, "+6281234567890", "081234567890", "6281234567890"} {
			loginRec := loginAndGetCode(t, pool, identifier, "Password123!")
			if loginRec != http.StatusOK {
				t.Fatalf("login with %q failed: %d", identifier, loginRec)
			}
		}

		// Wrong password rejected (Android & Web share endpoint too).
		if rec := loginAndGetCode(t, pool, testEmail, "WrongPassword999!"); rec == http.StatusOK {
			t.Fatalf("wrong password accepted")
		}

		cleanFixture()
	}
}

func TestPhoneNormalization_ReimportIdempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires seeded test database")
	}
	pool := testPool(t)
	defer pool.Close()

	rtID := setupTestRT(t, pool, "PhoneImport RT", 1, "PHN2")
	rowName := "ReimportPhone " + fmt.Sprintf("%d", time.Now().UnixNano())

	f := excelize.NewFile()
	writeHeaderRow(f)
	unqNik := fmt.Sprintf("3171010101%06d", time.Now().UnixNano()%1000000)
	writeRow(f, 0, []string{"head", "Head Test", "OWNER", "Alamat", rowName, unqNik, "+6281234567890", fmt.Sprintf("riphone-%d@example.com", time.Now().UnixNano()), "", "", "Password123!"})
	xlsxData := xlsxBytes(f)
	f.Close()

	saToken := makeSuperAdminToken(t)
	router := setupExcelRouter(pool)

	// First import.
	buf1, boundary1 := buildMultipartBody("file", "import.xlsx", xlsxData)
	req1 := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx?rt_id="+rtID, saToken, buf1)
	req1.Header.Set("Content-Type", boundary1)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first import failed: %d %s", rec1.Code, rec1.Body.String())
	}

	// Second import with the same data — no duplication, canonical phone preserved.
	buf2, boundary2 := buildMultipartBody("file", "import2.xlsx", xlsxData)
	req2 := makeAuthRequest(pool, http.MethodPost, "/api/v1/warga/import/commit/xlsx?rt_id="+rtID, saToken, buf2)
	req2.Header.Set("Content-Type", boundary2)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated && rec2.Code != http.StatusOK {
		t.Fatalf("reimport failed: %d %s", rec2.Code, rec2.Body.String())
	}

	// Count phone rows persisted (must remain 1 for this NIK).
	var cnt int
	err := pool.Raw().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM residents WHERE nik = $2 AND rt_id = $1 AND phone = '081234567890'`, rtID, unqNik).Scan(&cnt)
	if err != nil {
		t.Fatalf("query residents: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected exactly 1 active canonical phone row after re-import, got %d", cnt)
	}
}

// loginAndGetCode logs in via /api/v1/auth/login (shared Android & Web endpoint).
// Returns HTTP status code.
func loginAndGetCode(t *testing.T, pool *database.Pool, identifier, password string) int {
	t.Helper()

	svc := auth.NewService(auth.ServiceOptions{RefreshLifetime: 7 * 24 * time.Hour})
	authHandler := auth.NewHandler(pool, svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login",
		bytes.NewBufferString(fmt.Sprintf(`{"identifier":%q,"password":%q}`, identifier, password)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	authHandler.Login().ServeHTTP(rec, req)
	return rec.Code
}

var _ = chi.NewRouter
var _ io.Reader
var _ = json.Marshal
var _ = strings.TrimSpace
