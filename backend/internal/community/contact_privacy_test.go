package community

import (
	"context"
	"fmt"
	"testing"
	"time"

	"social-finance/internal/auth"
	"social-finance/internal/testutil"
)

func TestOrganizationStructure_ContactPrivacyExactKetuaOnly(t *testing.T) {
	acKetua := &auth.AuthContext{SystemRole: "user", Jabatan: "ketua", RTID: "rt-1"}
	acKetuaRT := &auth.AuthContext{SystemRole: "user", Jabatan: "ketua_rt", RTID: "rt-1"}
	acSekretaris := &auth.AuthContext{SystemRole: "user", Jabatan: "sekretaris", RTID: "rt-1"}
	acBendahara := &auth.AuthContext{SystemRole: "user", Jabatan: "bendahara", RTID: "rt-1"}
	acWarga := &auth.AuthContext{SystemRole: "user", Jabatan: "warga", RTID: "rt-1"}
	acSA := &auth.AuthContext{SystemRole: auth.SystemRoleSuperAdmin, Jabatan: "ketua", RTID: "rt-1"}

	h := &Handler{}
	if !h.hasContactPermission(context.Background(), acKetua) {
		t.Fatalf("exact ketua must have contact permission")
	}
	for name, ac := range map[string]*auth.AuthContext{
		"ketua_rt alias": acKetuaRT,
		"sekretaris":     acSekretaris,
		"bendahara":      acBendahara,
		"warga":          acWarga,
		"super_admin":    acSA,
	} {
		if h.hasContactPermission(context.Background(), ac) {
			t.Fatalf("%s must NOT have contact permission", name)
		}
	}
}

func TestHomeDashboard_RedactsSecurityOnDutyPhoneForNonKetua(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()
	var rtID string
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%100000000)
	err := pool.Raw().QueryRowContext(ctx, `INSERT INTO rts (name, rt, rw, is_active) VALUES ($1, $2, $3, true) RETURNING id`, "RT Privacy "+suffix, suffix[len(suffix)-3:], 9000+time.Now().UnixNano()%999).Scan(&rtID)
	if err != nil {
		t.Fatalf("insert rt: %v", err)
	}

	phone := "08123456789"
	repo := &Repository{pool: pool}
	svc := NewService(repo, nil)
	svc.WithOnDutyReader(func(ctx context.Context, gotRTID string) ([]SecurityOfficer, error) {
		return []SecurityOfficer{{UserID: "u1", FullName: "Satpam", PhoneNumber: &phone}}, nil
	})

	data, err := svc.GetHomeDashboard(ctx, rtID, "Warga", "warga", "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !data.SecurityOnDuty.IsAvailable || len(data.SecurityOnDuty.Officers) != 1 {
		t.Fatalf("expected on-duty officer")
	}
	if data.SecurityOnDuty.Officers[0].PhoneNumber != nil {
		t.Fatalf("expected phone redacted for non-ketua")
	}

	data2, err := svc.GetHomeDashboard(ctx, rtID, "Ketua", "pengurus", "ketua", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data2.SecurityOnDuty.Officers[0].PhoneNumber == nil || *data2.SecurityOnDuty.Officers[0].PhoneNumber != phone {
		t.Fatalf("expected phone preserved for exact ketua")
	}
}
