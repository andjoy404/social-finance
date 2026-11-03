package rw

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"social-finance/internal/auth"
)

// TestSAWithNilPool returns ErrRWInfrastructure when pool is nil.
// This verifies the nil-pool check in ValidateSAForRW returns
// the infrastructure error (wrapped in ErrRWInfrastructure) rather than panicking.
func TestSAWithNilPool(t *testing.T) {
	ctx := context.Background()
	rwID := uuid.New()
	ac := &auth.AuthContext{
		UserID:     "sa-user-id",
		SystemRole: "super_admin",
		TenantRole: "",
		Jabatan:    "",
	}

	scopingCfg := &auth.ScopingConfig{
		RTIDs: nil,
		RWIDs: []string{rwID.String()},
	}

	// pool = nil → ValidateSAForRW should return ErrRWInfrastructure
	svc := NewService(nil, nil, scopingCfg)

	err := svc.checkRWAccess(ctx, ac, rwID.String())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "rw access denied: infrastructure error: database unavailable" {
		t.Fatalf("expected ErrRWInfrastructure message, got %q", err.Error())
	}
}
