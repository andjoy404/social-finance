package security_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"social-finance/internal/database"
	"social-finance/internal/security"
	"social-finance/internal/testutil"
)

func createTestUserWithJabatan(t *testing.T, pool *database.Pool, ctx context.Context, rtID, role, jabatan string) string {
	t.Helper()
	userID := uuid.New().String()
	email := fmt.Sprintf("%s-%s@test.com", jabatan, userID[:8])
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, is_active)
		VALUES ($1, $2, 'hash', $3, true)
	`, userID, email, fmt.Sprintf("User %s", jabatan))
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO user_rt_memberships (user_id, rt_id, role, jabatan, is_active)
		VALUES ($1, $2, $3, $4, true)
	`, userID, rtID, role, jabatan)
	if err != nil {
		t.Fatalf("failed to create user_rt_membership: %v", err)
	}
	return userID
}

func createTestResident(t *testing.T, pool *database.Pool, ctx context.Context, rtID, userID string) string {
	t.Helper()
	resID := uuid.New().String()
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO residents (id, rt_id, user_id, full_name, is_active)
		VALUES ($1, $2, $3, 'Warga Test', true)
	`, resID, rtID, userID)
	if err != nil {
		t.Fatalf("failed to create resident: %v", err)
	}
	return resID
}

func createTestRT(t *testing.T, pool *database.Pool, ctx context.Context, code string) string {
	t.Helper()
	rtID := uuid.New().String()
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO rts (id, rw, rt, name, is_active)
		VALUES ($1, 99, $2, $3, true)
	`, rtID, code, fmt.Sprintf("RT %s", code))
	if err != nil {
		t.Fatalf("failed to create RT: %v", err)
	}
	return rtID
}

func TestPatrolAndPaket_NotificationsSentOnlyToKetuaRT(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtA := createTestRT(t, pool, ctx, fmt.Sprintf("01-%d", time.Now().UnixNano()%10000))
	rtB := createTestRT(t, pool, ctx, fmt.Sprintf("02-%d", time.Now().UnixNano()%10000))

	ketuaA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "ketua")
	sekretarisA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "sekretaris")
	bendaharaA := createTestUserWithJabatan(t, pool, ctx, rtA, "bendahara", "bendahara")
	officerA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	wargaAUser := createTestUserWithJabatan(t, pool, ctx, rtA, "warga", "anggota")
	wargaAResident := createTestResident(t, pool, ctx, rtA, wargaAUser)

	ketuaB := createTestUserWithJabatan(t, pool, ctx, rtB, "pengurus", "ketua")

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)

	// Create a patrol template in rtA
	tpl, err := svc.CreatePatrolTemplate(ctx, rtA, "Pos Depan", "Gerbang Utama", ketuaA)
	if err != nil {
		t.Fatalf("failed to create patrol template: %v", err)
	}

	// 1. Submit Patrol Bundle
	bundle, err := svc.CreatePatrolBundle(ctx, rtA, officerA, nil, []security.CreatePatrolPhoto{
		{
			PatrolTemplateID: tpl.ID,
			ImageKey:         "patrol/test.jpg",
			CapturedAt:       time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to create patrol bundle: %v", err)
	}
	if bundle == nil {
		t.Fatal("expected non-nil bundle")
	}

	// Verify patrol notification recipients
	var countKetuaA, countSekretarisA, countBendaharaA, countKetuaB int
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'patrol_bundle_submitted' AND user_id = $1
	`, ketuaA).Scan(&countKetuaA)
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'patrol_bundle_submitted' AND user_id = $1
	`, sekretarisA).Scan(&countSekretarisA)
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'patrol_bundle_submitted' AND user_id = $1
	`, bendaharaA).Scan(&countBendaharaA)
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'patrol_bundle_submitted' AND user_id = $1
	`, ketuaB).Scan(&countKetuaB)

	if countKetuaA != 1 {
		t.Errorf("expected Ketua RT to receive 1 patrol notification, got %d", countKetuaA)
	}
	if countSekretarisA != 0 {
		t.Errorf("expected Sekretaris to receive 0 patrol notifications, got %d", countSekretarisA)
	}
	if countBendaharaA != 0 {
		t.Errorf("expected Bendahara to receive 0 patrol notifications, got %d", countBendaharaA)
	}
	if countKetuaB != 0 {
		t.Errorf("expected Ketua RT B to receive 0 patrol notifications, got %d", countKetuaB)
	}

	// 2. Submit Paket Report
	desc := "JNE - Bpk. Warga"
	paket, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaAResident, []security.CreatePaketItem{
		{
			Description: &desc,
			ImageKeys:   []string{"paket/box.jpg"},
		},
	})
	if err != nil {
		t.Fatalf("failed to create paket report: %v", err)
	}
	if paket == nil {
		t.Fatal("expected non-nil paket report")
	}

	// Verify paket incoming notification recipients
	var pKetuaA, pSekretarisA, pBendaharaA, pKetuaB int
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'paket_received' AND title = 'Laporan Paket Masuk' AND user_id = $1
	`, ketuaA).Scan(&pKetuaA)
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'paket_received' AND title = 'Laporan Paket Masuk' AND user_id = $1
	`, sekretarisA).Scan(&pSekretarisA)
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'paket_received' AND title = 'Laporan Paket Masuk' AND user_id = $1
	`, bendaharaA).Scan(&pBendaharaA)
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE event_type = 'paket_received' AND title = 'Laporan Paket Masuk' AND user_id = $1
	`, ketuaB).Scan(&pKetuaB)

	if pKetuaA != 1 {
		t.Errorf("expected Ketua RT to receive 1 paket notification, got %d", pKetuaA)
	}
	if pSekretarisA != 0 {
		t.Errorf("expected Sekretaris to receive 0 paket notifications, got %d", pSekretarisA)
	}
	if pBendaharaA != 0 {
		t.Errorf("expected Bendahara to receive 0 paket notifications, got %d", pBendaharaA)
	}
	if pKetuaB != 0 {
		t.Errorf("expected Ketua RT B to receive 0 paket notifications, got %d", pKetuaB)
	}
}

func TestPatrolAndPaket_TenantIsolationAndSecurityAudit(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtA := createTestRT(t, pool, ctx, fmt.Sprintf("A-%d", time.Now().UnixNano()%10000))
	rtB := createTestRT(t, pool, ctx, fmt.Sprintf("B-%d", time.Now().UnixNano()%10000))

	ketuaA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "ketua")
	officerA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	wargaAUser := createTestUserWithJabatan(t, pool, ctx, rtA, "warga", "anggota")
	wargaAResident := createTestResident(t, pool, ctx, rtA, wargaAUser)

	ketuaB := createTestUserWithJabatan(t, pool, ctx, rtB, "pengurus", "ketua")
	officerB := createTestUserWithJabatan(t, pool, ctx, rtB, "pengurus", "keamanan")
	wargaBUser := createTestUserWithJabatan(t, pool, ctx, rtB, "warga", "anggota")
	wargaBResident := createTestResident(t, pool, ctx, rtB, wargaBUser)

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)

	// Create templates in RT A and RT B
	tplA, err := svc.CreatePatrolTemplate(ctx, rtA, "Pos Depan RT A", "Gerbang RT A", ketuaA)
	if err != nil {
		t.Fatalf("failed to create tplA: %v", err)
	}
	tplB, err := svc.CreatePatrolTemplate(ctx, rtB, "Pos Depan RT B", "Gerbang RT B", ketuaB)
	if err != nil {
		t.Fatalf("failed to create tplB: %v", err)
	}

	// 1. Patrol: Cross-RT template rejected
	t.Run("CreatePatrolBundle_RejectsTemplateFromAnotherRT", func(t *testing.T) {
		_, err := svc.CreatePatrolBundle(ctx, rtA, officerA, nil, []security.CreatePatrolPhoto{
			{
				PatrolTemplateID: tplB.ID,
				ImageKey:         fmt.Sprintf("community/%s/patrol/p1.jpg", rtA),
				CapturedAt:       time.Now(),
			},
		})
		if err == nil {
			t.Fatal("expected error using tplB in RT A, but got nil")
		}
	})

	// 2. Patrol: Cross-RT photo image key rejected
	t.Run("CreatePatrolBundle_RejectsPhotoFromAnotherRT", func(t *testing.T) {
		_, err := svc.CreatePatrolBundle(ctx, rtA, officerA, nil, []security.CreatePatrolPhoto{
			{
				PatrolTemplateID: tplA.ID,
				ImageKey:         fmt.Sprintf("community/%s/patrol/stolen.jpg", rtB),
				CapturedAt:       time.Now(),
			},
		})
		if err == nil {
			t.Fatal("expected error using RT B image key in RT A, but got nil")
		}
	})

	// 3. Patrol: Valid submission succeeds and captures snapshots
	var validBundleA *security.PatrolBundle
	t.Run("CreatePatrolBundle_ValidSubmissionCapturesSnapshots", func(t *testing.T) {
		b, err := svc.CreatePatrolBundle(ctx, rtA, officerA, nil, []security.CreatePatrolPhoto{
			{
				PatrolTemplateID: tplA.ID,
				ImageKey:         fmt.Sprintf("community/%s/patrol/valid.jpg", rtA),
				CapturedAt:       time.Now(),
			},
		})
		if err != nil {
			t.Fatalf("unexpected error creating patrol bundle: %v", err)
		}
		if len(b.Photos) == 0 {
			t.Fatal("expected photos in created bundle")
		}
		if b.Photos[0].TemplateName != "Pos Depan RT A" {
			t.Errorf("expected snapshot template name 'Pos Depan RT A', got %s", b.Photos[0].TemplateName)
		}
		validBundleA = b
	})

	// 4. Patrol: Cross-RT photo access rejected (404)
	t.Run("GetPatrolPhotoKey_CrossRTAccessDenied", func(t *testing.T) {
		if validBundleA == nil || len(validBundleA.Photos) == 0 {
			t.Skip("no valid bundle")
		}
		photoID := validBundleA.Photos[0].ID
		// RT A can access
		keyA, err := svc.GetPatrolPhotoKey(ctx, rtA, photoID)
		if err != nil || keyA == "" {
			t.Fatalf("RT A should be able to get its own photo key, got: %v", err)
		}
		// RT B cannot access
		_, err = svc.GetPatrolPhotoKey(ctx, rtB, photoID)
		if err == nil {
			t.Fatal("RT B must not be able to get RT A photo key, expected 404")
		}
	})

	// 5. Patrol: Template deletion preserves historical photos
	t.Run("DeletePatrolTemplate_PreservesReferencedPhotos", func(t *testing.T) {
		// tplA is referenced by validBundleA. Photos should not be broken or deleted.
		err := svc.DeletePatrolTemplate(ctx, rtA, tplA.ID)
		if err != nil {
			t.Fatalf("DeletePatrolTemplate returned error: %v", err)
		}
		// Template should now be is_active = false
		list, err := svc.ListPatrolTemplates(ctx, rtA)
		if err != nil {
			t.Fatalf("ListPatrolTemplates error: %v", err)
		}
		for _, tmpl := range list {
			if tmpl.ID == tplA.ID && tmpl.IsActive {
				t.Error("expected referenced template to become inactive")
			}
		}
		// Bundle photos should still be readable with snapshots
		b, err := svc.GetPatrolBundle(ctx, rtA, validBundleA.ID)
		if err != nil {
			t.Fatalf("GetPatrolBundle error: %v", err)
		}
		if len(b.Photos) == 0 || b.Photos[0].TemplateName != "Pos Depan RT A" {
			t.Errorf("historical photo snapshots lost after template soft-delete: %v", b.Photos)
		}
	})

	// 6. Paket: Cross-RT receiver resident rejected
	t.Run("CreatePaketReport_RejectsReceiverFromAnotherRT", func(t *testing.T) {
		desc := "Paket Kurir"
		_, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaBResident, []security.CreatePaketItem{
			{
				Description: &desc,
				ImageKeys:   []string{fmt.Sprintf("community/%s/paket/box.jpg", rtA)},
			},
		})
		if err == nil {
			t.Fatal("expected error using wargaBResident in RT A, but got nil")
		}
	})

	// 7. Paket: Cross-RT image key rejected
	t.Run("CreatePaketReport_RejectsPhotoFromAnotherRT", func(t *testing.T) {
		desc := "Paket Kurir"
		_, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaAResident, []security.CreatePaketItem{
			{
				Description: &desc,
				ImageKeys:   []string{fmt.Sprintf("community/%s/paket/stolen.jpg", rtB)},
			},
		})
		if err == nil {
			t.Fatal("expected error using RT B image key in RT A, but got nil")
		}
	})

	// 8. Paket: Valid report creation
	var validPaketA *security.PaketReport
	var validPaketA2 *security.PaketReport
	t.Run("CreatePaketReport_ValidSubmissionSucceeds", func(t *testing.T) {
		desc := "Lazada"
		rep, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaAResident, []security.CreatePaketItem{
			{
				Description: &desc,
				ImageKeys:   []string{fmt.Sprintf("community/%s/paket/lazada.jpg", rtA)},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error creating paket report: %v", err)
		}
		validPaketA = rep

		rep2, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaAResident, []security.CreatePaketItem{
			{
				Description: &desc,
				ImageKeys:   []string{fmt.Sprintf("community/%s/paket/lazada2.jpg", rtA)},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error creating paket report 2: %v", err)
		}
		validPaketA2 = rep2
	})

	// 9. Paket: Item status update cross-report mismatch rejected
	t.Run("UpdatePaketItemStatus_MismatchedReportAndItemRejected", func(t *testing.T) {
		if validPaketA == nil || validPaketA2 == nil || len(validPaketA.Items) == 0 || len(validPaketA2.Items) == 0 {
			t.Skip("missing test reports")
		}
		proof := fmt.Sprintf("community/%s/paket/proof.jpg", rtA)
		// Try to update item of report 2 using report 1 ID
		err := svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, validPaketA2.Items[0].ID, "sudah_diambil", officerA, &proof)
		if err == nil {
			t.Fatal("expected 404 for item belonging to a different report, got nil")
		}
	})

	// 10. Paket: Cross-RT status update rejected
	t.Run("UpdatePaketItemStatus_CrossRTAccessDenied", func(t *testing.T) {
		if validPaketA == nil || len(validPaketA.Items) == 0 {
			t.Skip("missing test report")
		}
		proof := fmt.Sprintf("community/%s/paket/proof.jpg", rtA)
		// Officer B from RT B tries to update RT A report
		err := svc.UpdatePaketItemStatus(ctx, rtB, validPaketA.ID, validPaketA.Items[0].ID, "sudah_diambil", officerB, &proof)
		if err == nil {
			t.Fatal("expected 404 for cross-RT update, got nil")
		}
	})

	// 11. Paket: Missing proof photo for sudah_diambil rejected
	t.Run("UpdatePaketItemStatus_ProofPhotoRequiredForPickup", func(t *testing.T) {
		if validPaketA == nil || len(validPaketA.Items) == 0 {
			t.Skip("missing test report")
		}
		// Pass nil proof photo
		err := svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, validPaketA.Items[0].ID, "sudah_diambil", officerA, nil)
		if err == nil {
			t.Fatal("expected proof_required error, got nil")
		}
	})

	// 12. Paket: Cross-RT proof photo key rejected
	t.Run("UpdatePaketItemStatus_CrossRTProofPhotoRejected", func(t *testing.T) {
		if validPaketA == nil || len(validPaketA.Items) == 0 {
			t.Skip("missing test report")
		}
		proofB := fmt.Sprintf("community/%s/paket/proof_b.jpg", rtB)
		err := svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, validPaketA.Items[0].ID, "sudah_diambil", officerA, &proofB)
		if err == nil {
			t.Fatal("expected forbidden error for RT B proof photo in RT A, got nil")
		}
	})

	// 13. Paket: Valid pickup and status transition lifecycle
	t.Run("UpdatePaketItemStatus_LifecycleAndIdempotency", func(t *testing.T) {
		if validPaketA == nil || len(validPaketA.Items) == 0 {
			t.Skip("missing test report")
		}
		proof := fmt.Sprintf("community/%s/paket/proof_valid.jpg", rtA)
		itemID := validPaketA.Items[0].ID

		// A. Valid pickup succeeds
		err := svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, itemID, "sudah_diambil", officerA, &proof)
		if err != nil {
			t.Fatalf("pickup failed: %v", err)
		}

		// B. Duplicate pickup is idempotent
		err = svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, itemID, "sudah_diambil", officerA, &proof)
		if err != nil {
			t.Fatalf("idempotent duplicate pickup returned error: %v", err)
		}

		// C. Cannot revert sudah_diambil to dititipkan
		err = svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, itemID, "dititipkan", officerA, nil)
		if err == nil {
			t.Fatal("expected invalid_transition error when reverting picked up item, got nil")
		}

		// D. Cannot revert sudah_diambil to dibatalkan
		err = svc.UpdatePaketItemStatus(ctx, rtA, validPaketA.ID, itemID, "dibatalkan", officerA, nil)
		if err == nil {
			t.Fatal("expected invalid_transition error when cancelling picked up item, got nil")
		}

		// E. Canceled item cannot be changed
		if validPaketA2 != nil && len(validPaketA2.Items) > 0 {
			item2ID := validPaketA2.Items[0].ID
			err = svc.UpdatePaketItemStatus(ctx, rtA, validPaketA2.ID, item2ID, "dibatalkan", officerA, nil)
			if err != nil {
				t.Fatalf("cancellation failed: %v", err)
			}
			// Attempt to mark canceled item as sudah_diambil -> must fail
			err = svc.UpdatePaketItemStatus(ctx, rtA, validPaketA2.ID, item2ID, "sudah_diambil", officerA, &proof)
			if err == nil {
				t.Fatal("expected invalid_transition error for canceled item, got nil")
			}
		}
	})

	// 14. Paket: Cross-RT photo access rejected (404)
	t.Run("GetPaketPhotoKey_CrossRTAccessDenied", func(t *testing.T) {
		if validPaketA == nil || len(validPaketA.Items) == 0 {
			t.Skip("missing test report")
		}
		// Retrieve report with photos
		rep, err := svc.GetPaketReport(ctx, rtA, validPaketA.ID)
		if err != nil || len(rep.Items) == 0 || len(rep.Items[0].Photos) == 0 {
			t.Skip("no photos in paket")
		}
		photoID := rep.Items[0].Photos[0].ID

		// RT A can access
		keyA, err := svc.GetPaketPhotoKey(ctx, rtA, photoID)
		if err != nil || keyA == "" {
			t.Fatalf("RT A should be able to get its own paket photo key, got: %v", err)
		}
		// RT B cannot access
		_, err = svc.GetPaketPhotoKey(ctx, rtB, photoID)
		if err == nil {
			t.Fatal("RT B must not be able to get RT A paket photo key, expected 404")
		}
	})
}
