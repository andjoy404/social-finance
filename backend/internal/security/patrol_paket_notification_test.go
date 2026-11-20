package security_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
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

var testRTSeq uint64

func createTestRT(t *testing.T, pool *database.Pool, ctx context.Context, code string) string {
	t.Helper()
	rtID := uuid.New().String()
	seq := atomic.AddUint64(&testRTSeq, 1)
	rwNum := int(time.Now().UnixNano()%70000) + int(seq)*100 + 1000
	rtCode := fmt.Sprintf("%s-%d-%s", code, seq, uuid.NewString()[:6])
	_, err := pool.Raw().ExecContext(ctx, `
		INSERT INTO rts (id, rw, rt, name, is_active)
		VALUES ($1, $2, $3, $4, true)
	`, rtID, rwNum, rtCode, fmt.Sprintf("RT %s", rtCode))
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

	// 15. Paket: Recipient warga access control vs other warga
	t.Run("IsPaketReceiver_AuthorizationValidation", func(t *testing.T) {
		if validPaketA == nil {
			t.Skip("missing test report")
		}
		// Recipient warga in RT A must be authorized
		isRecv, err := svc.IsPaketReceiver(ctx, rtA, validPaketA.ID, wargaAUser)
		if err != nil {
			t.Fatalf("unexpected error checking recipient: %v", err)
		}
		if !isRecv {
			t.Error("expected wargaAUser to be confirmed as recipient of validPaketA")
		}

		// Other warga in RT B must be rejected
		isRecvB, err := svc.IsPaketReceiver(ctx, rtA, validPaketA.ID, wargaBUser)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isRecvB {
			t.Error("expected wargaBUser to NOT be recipient of validPaketA")
		}

		// Cross-RT check must be false
		isRecvCross, err := svc.IsPaketReceiver(ctx, rtB, validPaketA.ID, wargaAUser)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isRecvCross {
			t.Error("expected cross-RT check to return false")
		}

		// Photo receiver check
		rep, err := svc.GetPaketReport(ctx, rtA, validPaketA.ID)
		if err != nil || len(rep.Items) == 0 || len(rep.Items[0].Photos) == 0 {
			t.Skip("no photos in paket")
		}
		photoID := rep.Items[0].Photos[0].ID

		isPhotoRecv, err := svc.IsPaketPhotoReceiver(ctx, rtA, photoID, wargaAUser)
		if err != nil || !isPhotoRecv {
			t.Errorf("expected wargaAUser to be authorized for photo, got: %v, err: %v", isPhotoRecv, err)
		}

		isPhotoRecvB, err := svc.IsPaketPhotoReceiver(ctx, rtA, photoID, wargaBUser)
		if err != nil || isPhotoRecvB {
			t.Errorf("expected wargaBUser to NOT be authorized for photo, got: %v, err: %v", isPhotoRecvB, err)
		}
	})
}

func TestPaket_HandoverAuditAndNotificationDistribution(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtA := createTestRT(t, pool, ctx, fmt.Sprintf("HA-%d", time.Now().UnixNano()%10000))
	rtB := createTestRT(t, pool, ctx, fmt.Sprintf("HB-%d", time.Now().UnixNano()%10000))

	ketuaA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "ketua")
	sekretarisA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "sekretaris")
	bendaharaA := createTestUserWithJabatan(t, pool, ctx, rtA, "bendahara", "bendahara")
	officerReceiver := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	officerHandover := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")

	wargaAUser := createTestUserWithJabatan(t, pool, ctx, rtA, "warga", "anggota")
	wargaAResident := createTestResident(t, pool, ctx, rtA, wargaAUser)

	ketuaB := createTestUserWithJabatan(t, pool, ctx, rtB, "pengurus", "ketua")

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)

	// 1. Create a Paket Report by officerReceiver
	desc := "Paket Elektronik"
	rep, err := svc.CreatePaketReport(ctx, rtA, officerReceiver, wargaAResident, []security.CreatePaketItem{
		{
			Description: &desc,
			ImageKeys:   []string{fmt.Sprintf("community/%s/paket/laptop.jpg", rtA)},
		},
	})
	if err != nil {
		t.Fatalf("failed to create paket report: %v", err)
	}
	if len(rep.Items) == 0 {
		t.Fatalf("expected report items to have at least 1 item")
	}
	itemID := rep.Items[0].ID

	// Verify initial state
	if rep.CreatedBy != officerReceiver {
		t.Errorf("expected CreatedBy == officerReceiver (%s), got %s", officerReceiver, rep.CreatedBy)
	}
	if rep.HandedOverBy != nil {
		t.Errorf("expected initial HandedOverBy to be nil, got %v", *rep.HandedOverBy)
	}

	// 2. Perform handover by officerHandover
	proofPhoto := fmt.Sprintf("community/%s/paket/handover_proof.jpg", rtA)
	err = svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, itemID, "sudah_diambil", officerHandover, &proofPhoto)
	if err != nil {
		t.Fatalf("failed to update paket item status to sudah_diambil: %v", err)
	}

	// 3. Verify audit fields in database / GetPaketReport
	updatedRep, err := svc.GetPaketReport(ctx, rtA, rep.ID)
	if err != nil {
		t.Fatalf("failed to get updated paket report: %v", err)
	}
	if updatedRep.CreatedBy != officerReceiver {
		t.Errorf("expected CreatedBy to remain officerReceiver (%s), got %s", officerReceiver, updatedRep.CreatedBy)
	}
	if updatedRep.HandedOverBy == nil || *updatedRep.HandedOverBy != officerHandover {
		t.Errorf("expected report HandedOverBy == officerHandover (%s), got %v", officerHandover, updatedRep.HandedOverBy)
	}
	if updatedRep.HandedOverByName == "" {
		t.Errorf("expected report HandedOverByName to be set, got empty")
	}
	if updatedRep.HandedOverAt == nil {
		t.Errorf("expected report HandedOverAt to be set")
	}

	if len(updatedRep.Items) == 0 {
		t.Fatalf("expected items in updated report")
	}
	item := updatedRep.Items[0]
	if item.HandedOverBy == nil || *item.HandedOverBy != officerHandover {
		t.Errorf("expected item HandedOverBy == officerHandover (%s), got %v", officerHandover, item.HandedOverBy)
	}
	if item.HandedOverByName == "" {
		t.Errorf("expected item HandedOverByName to be set, got empty")
	}
	if item.HandedOverAt == nil {
		t.Errorf("expected item HandedOverAt to be set")
	}

	// 4. Verify notification distribution in notification_logs
	// a) Warga recipient must receive "Paket Telah Diambil"
	var countWargaNotif int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diambil'
	`, wargaAUser).Scan(&countWargaNotif)
	if err != nil {
		t.Fatalf("failed to query warga notification: %v", err)
	}
	if countWargaNotif != 1 {
		t.Errorf("expected exactly 1 'Paket Telah Diambil' notification for warga, got %d", countWargaNotif)
	}

	// b) Ketua RT A must receive "Paket Telah Diserahkan"
	var countKetuaNotif int
	var ketuaBody string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(body), '') FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan'
	`, ketuaA).Scan(&countKetuaNotif, &ketuaBody)
	if err != nil {
		t.Fatalf("failed to query ketua notification: %v", err)
	}
	if countKetuaNotif != 1 {
		t.Errorf("expected exactly 1 'Paket Telah Diserahkan' notification for ketua, got %d", countKetuaNotif)
	}
	if ketuaBody == "" {
		t.Errorf("expected non-empty notification body for ketua")
	}

	// c) Sekretaris A (has notification.manage) must receive "Paket Telah Diserahkan"
	var countSekretarisNotif int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan'
	`, sekretarisA).Scan(&countSekretarisNotif)
	if err != nil {
		t.Fatalf("failed to query sekretaris notification: %v", err)
	}
	if countSekretarisNotif != 1 {
		t.Errorf("expected exactly 1 'Paket Telah Diserahkan' notification for sekretaris (with notification.manage), got %d", countSekretarisNotif)
	}

	// d) Bendahara A (does not have notification.manage) must NOT receive "Paket Telah Diserahkan"
	var countBendaharaNotif int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan'
	`, bendaharaA).Scan(&countBendaharaNotif)
	if err != nil {
		t.Fatalf("failed to query bendahara notification: %v", err)
	}
	if countBendaharaNotif != 0 {
		t.Errorf("expected 0 'Paket Telah Diserahkan' notification for bendahara, got %d", countBendaharaNotif)
	}

	// e) Ketua RT B (different RT) must NOT receive any notification
	var countKetuaBNotif int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1
	`, ketuaB).Scan(&countKetuaBNotif)
	if err != nil {
		t.Fatalf("failed to query ketua B notification: %v", err)
	}
	if countKetuaBNotif != 0 {
		t.Errorf("expected 0 notification for RT B ketua, got %d", countKetuaBNotif)
	}

	// 5. Verify Idempotent status update retry
	err = svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, itemID, "sudah_diambil", officerHandover, &proofPhoto)
	if err != nil {
		t.Fatalf("expected idempotent update to succeed, got %v", err)
	}
	// Verify no new duplicate notifications were created
	var countKetuaNotifAfter int
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan'
	`, ketuaA).Scan(&countKetuaNotifAfter)
	if countKetuaNotifAfter != 1 {
		t.Errorf("expected still 1 notification after retry, got %d", countKetuaNotifAfter)
	}

	// 6. Verify Read Receipt for officer
	var notifID string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT id FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan'
		LIMIT 1
	`, sekretarisA).Scan(&notifID)
	if err != nil {
		t.Fatalf("failed to query sekretaris notification log: %v", err)
	}

	// Initially no read record
	var readCount int
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_reads
		WHERE notification_id = $1 AND user_id = $2
	`, notifID, sekretarisA).Scan(&readCount)
	if readCount != 0 {
		t.Errorf("expected initially unread notification, got readCount %d", readCount)
	}

	// Mark as read
	_, err = pool.Raw().ExecContext(ctx, `
		INSERT INTO notification_reads (notification_id, user_id, read_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (notification_id, user_id) DO NOTHING
	`, notifID, sekretarisA)
	if err != nil {
		t.Fatalf("failed to mark notification as read: %v", err)
	}

	// Verify now read
	_ = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_reads
		WHERE notification_id = $1 AND user_id = $2
	`, notifID, sekretarisA).Scan(&readCount)
	if readCount != 1 {
		t.Errorf("expected notification to be marked as read, got %d", readCount)
	}
}

func TestPaket_MultiItemHandoverByDifferentOfficers(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtA := createTestRT(t, pool, ctx, fmt.Sprintf("MHO-%d", time.Now().UnixNano()%10000))

	officerA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan") // Petugas Penerima
	officerB := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan") // Petugas Penyerah Item 1
	officerC := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan") // Petugas Penyerah Item 2

	wargaUser := createTestUserWithJabatan(t, pool, ctx, rtA, "warga", "anggota")
	wargaResident := createTestResident(t, pool, ctx, rtA, wargaUser)

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)

	// 1. Petugas A creates a 2-item package report
	desc1 := "Barang Pertama - Dokumen"
	desc2 := "Barang Kedua - Baju"
	rep, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaResident, []security.CreatePaketItem{
		{
			Description: &desc1,
			ImageKeys:   []string{fmt.Sprintf("community/%s/paket/item1.jpg", rtA)},
		},
		{
			Description: &desc2,
			ImageKeys:   []string{fmt.Sprintf("community/%s/paket/item2.jpg", rtA)},
		},
	})
	if err != nil {
		t.Fatalf("failed to create multi-item paket: %v", err)
	}
	if len(rep.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(rep.Items))
	}
	item1ID := rep.Items[0].ID
	item2ID := rep.Items[1].ID

	// Initial DB verification
	var dbCreatedBy string
	err = pool.Raw().QueryRowContext(ctx, "SELECT created_by FROM paket_reports WHERE id = $1", rep.ID).Scan(&dbCreatedBy)
	if err != nil || dbCreatedBy != officerA {
		t.Fatalf("expected DB created_by == officerA (%s), got %s, err: %v", officerA, dbCreatedBy, err)
	}

	// 2. Petugas B hands over Item 1 to warga
	proof1 := fmt.Sprintf("community/%s/paket/proof_item1.jpg", rtA)
	err = svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, item1ID, "sudah_diambil", officerB, &proof1)
	if err != nil {
		t.Fatalf("failed to update item 1: %v", err)
	}

	// Mid-state verification: Item 1 picked up by B, Item 2 still pending
	repMid, err := svc.GetPaketReport(ctx, rtA, rep.ID)
	if err != nil {
		t.Fatalf("GetPaketReport failed: %v", err)
	}
	// Report created_by must NOT change
	if repMid.CreatedBy != officerA {
		t.Errorf("expected CreatedBy to stay officerA (%s), got %s", officerA, repMid.CreatedBy)
	}
	if repMid.TotalItems != 2 || repMid.HandedOverItemsCount != 1 || repMid.PendingItemsCount != 1 {
		t.Errorf("expected mid counts (2, 1, 1), got (%d, %d, %d)", repMid.TotalItems, repMid.HandedOverItemsCount, repMid.PendingItemsCount)
	}
	if repMid.HandedOverByName != "Sebagian Diserahkan" || repMid.HandedOverBy != nil {
		t.Errorf("expected mid HandedOverByName == 'Sebagian Diserahkan' and nil HandedOverBy, got name %q, by %v", repMid.HandedOverByName, repMid.HandedOverBy)
	}

	var it1, it2 *security.PaketItem
	for i := range repMid.Items {
		if repMid.Items[i].ID == item1ID {
			it1 = &repMid.Items[i]
		} else if repMid.Items[i].ID == item2ID {
			it2 = &repMid.Items[i]
		}
	}
	if it1 == nil || it2 == nil {
		t.Fatalf("failed to find items by ID in mid report")
	}

	// Item 1 audit check
	if it1.Status != "sudah_diambil" {
		t.Errorf("expected item 1 status sudah_diambil, got %s", it1.Status)
	}
	if it1.HandedOverBy == nil || *it1.HandedOverBy != officerB {
		t.Errorf("expected item 1 handedOverBy officerB (%s), got %v", officerB, it1.HandedOverBy)
	}
	if it1.HandedOverAt == nil {
		t.Errorf("expected item 1 handedOverAt to be non-nil")
	}

	// Item 2 audit check (must still be pending without handover officer)
	if it2.Status != "dititipkan" {
		t.Errorf("expected item 2 status dititipkan, got %s", it2.Status)
	}
	if it2.HandedOverBy != nil {
		t.Errorf("expected item 2 handedOverBy to be nil, got %v", *it2.HandedOverBy)
	}
	if it2.HandedOverAt != nil {
		t.Errorf("expected item 2 handedOverAt to be nil, got %v", it2.HandedOverAt)
	}

	// 3. Petugas C hands over Item 2
	proof2 := fmt.Sprintf("community/%s/paket/proof_item2.jpg", rtA)
	err = svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, item2ID, "sudah_diambil", officerC, &proof2)
	if err != nil {
		t.Fatalf("failed to update item 2: %v", err)
	}

	// Final verification
	repFinal, err := svc.GetPaketReport(ctx, rtA, rep.ID)
	if err != nil {
		t.Fatalf("GetPaketReport final failed: %v", err)
	}
	if repFinal.CreatedBy != officerA {
		t.Errorf("expected CreatedBy to permanently remain officerA (%s), got %s", officerA, repFinal.CreatedBy)
	}
	if repFinal.TotalItems != 2 || repFinal.HandedOverItemsCount != 2 || repFinal.PendingItemsCount != 0 {
		t.Errorf("expected final counts (2, 2, 0), got (%d, %d, %d)", repFinal.TotalItems, repFinal.HandedOverItemsCount, repFinal.PendingItemsCount)
	}
	if repFinal.HandedOverByName != "Beberapa Petugas" || repFinal.HandedOverBy != nil {
		t.Errorf("expected multi-officer HandedOverByName == 'Beberapa Petugas' and nil HandedOverBy, got name %q, by %v", repFinal.HandedOverByName, repFinal.HandedOverBy)
	}

	var itFinal1, itFinal2 *security.PaketItem
	for i := range repFinal.Items {
		if repFinal.Items[i].ID == item1ID {
			itFinal1 = &repFinal.Items[i]
		} else if repFinal.Items[i].ID == item2ID {
			itFinal2 = &repFinal.Items[i]
		}
	}
	if itFinal1 == nil || itFinal2 == nil {
		t.Fatalf("failed to find items by ID in final report")
	}

	if itFinal1.HandedOverBy == nil || *itFinal1.HandedOverBy != officerB {
		t.Errorf("expected item 1 handedOverBy == officerB, got %v", itFinal1.HandedOverBy)
	}
	if itFinal2.HandedOverBy == nil || *itFinal2.HandedOverBy != officerC {
		t.Errorf("expected item 2 handedOverBy == officerC, got %v", itFinal2.HandedOverBy)
	}

	// Direct raw SQL check on paket_items table
	var it1HandedBy, it2HandedBy string
	var it1HandedAt, it2HandedAt time.Time
	err = pool.Raw().QueryRowContext(ctx, "SELECT handed_over_by_user_id, handed_over_at FROM paket_items WHERE id = $1", item1ID).Scan(&it1HandedBy, &it1HandedAt)
	if err != nil || it1HandedBy != officerB {
		t.Errorf("raw DB check item 1 failed: %v, handedBy: %s", err, it1HandedBy)
	}
	err = pool.Raw().QueryRowContext(ctx, "SELECT handed_over_by_user_id, handed_over_at FROM paket_items WHERE id = $1", item2ID).Scan(&it2HandedBy, &it2HandedAt)
	if err != nil || it2HandedBy != officerC {
		t.Errorf("raw DB check item 2 failed: %v, handedBy: %s", err, it2HandedBy)
	}
}

func TestPaket_ConcurrentHandoverSameItem(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtA := createTestRT(t, pool, ctx, fmt.Sprintf("CH-%d", time.Now().UnixNano()%10000))

	officerA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan") // creator
	officerB := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	officerC := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	ketuaA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "ketua")

	wargaUser := createTestUserWithJabatan(t, pool, ctx, rtA, "warga", "anggota")
	wargaResident := createTestResident(t, pool, ctx, rtA, wargaUser)

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)

	desc := "Paket Concurrency"
	rep, err := svc.CreatePaketReport(ctx, rtA, officerA, wargaResident, []security.CreatePaketItem{
		{Description: &desc, ImageKeys: []string{fmt.Sprintf("community/%s/paket/concurrent.jpg", rtA)}},
	})
	if err != nil {
		t.Fatalf("create paket: %v", err)
	}
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(rep.Items))
	}
	itemID := rep.Items[0].ID

	proofB := fmt.Sprintf("community/%s/paket/proof_b.jpg", rtA)
	proofC := fmt.Sprintf("community/%s/paket/proof_c.jpg", rtA)

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	officers := []string{officerB, officerC}
	proofs := []*string{&proofB, &proofC}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			errs[idx] = svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, itemID, "sudah_diambil", officers[idx], proofs[idx])
		}(i)
	}
	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("concurrent handover timed out (possible deadlock)")
	}

	for i, e := range errs {
		if e != nil {
			t.Fatalf("concurrent handover goroutine %d returned unexpected error: %v", i, e)
		}
	}

	var status string
	var handedBy string
	var handedAt time.Time
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT status, COALESCE(handed_over_by_user_id::text, ''), handed_over_at
		FROM paket_items WHERE id = $1
	`, itemID).Scan(&status, &handedBy, &handedAt)
	if err != nil {
		t.Fatalf("query item: %v", err)
	}
	if status != "sudah_diambil" {
		t.Errorf("expected status sudah_diambil, got %s", status)
	}
	if handedBy != officerB && handedBy != officerC {
		t.Errorf("expected handed_over_by_user_id to be officerB or officerC, got %q", handedBy)
	}
	if handedAt.IsZero() {
		t.Error("expected handed_over_at to be set")
	}
	winner := handedBy
	winnerAt := handedAt

	// Idempotent retry after concurrent completion must not change audit identity/time.
	retryProof := fmt.Sprintf("community/%s/paket/proof_retry.jpg", rtA)
	if err := svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, itemID, "sudah_diambil", officerB, &retryProof); err != nil {
		t.Fatalf("idempotent retry failed: %v", err)
	}
	var retryBy string
	var retryAt time.Time
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COALESCE(handed_over_by_user_id::text, ''), handed_over_at
		FROM paket_items WHERE id = $1
	`, itemID).Scan(&retryBy, &retryAt)
	if err != nil {
		t.Fatalf("query item after retry: %v", err)
	}
	if retryBy != winner {
		t.Errorf("idempotent retry changed handed_over_by_user_id: want %s got %s", winner, retryBy)
	}
	if !retryAt.Equal(winnerAt) {
		t.Errorf("idempotent retry changed handed_over_at: want %v got %v", winnerAt, retryAt)
	}

	// Notification counts: exactly one per recipient-item.
	var wargaPickup int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND dedup_key = $2 AND title = 'Paket Telah Diambil'
	`, wargaUser, fmt.Sprintf("paket_pickup:%s:%s", itemID, wargaUser)).Scan(&wargaPickup)
	if err != nil {
		t.Fatalf("count warga pickup: %v", err)
	}
	if wargaPickup != 1 {
		t.Errorf("expected 1 warga pickup notification, got %d", wargaPickup)
	}

	var ketuaHandover int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND dedup_key = $2 AND title = 'Paket Telah Diserahkan'
	`, ketuaA, fmt.Sprintf("paket_diserahkan:%s:%s", itemID, ketuaA)).Scan(&ketuaHandover)
	if err != nil {
		t.Fatalf("count ketua handover: %v", err)
	}
	if ketuaHandover != 1 {
		t.Errorf("expected 1 ketua handover notification, got %d", ketuaHandover)
	}

	var totalForItem int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE (dedup_key = $1 OR dedup_key = $2)
	`, fmt.Sprintf("paket_pickup:%s:%s", itemID, wargaUser), fmt.Sprintf("paket_diserahkan:%s:%s", itemID, ketuaA)).Scan(&totalForItem)
	if err != nil {
		t.Fatalf("count total notifications for known recipients: %v", err)
	}
	if totalForItem != 2 {
		t.Errorf("expected exactly 2 notifications (1 warga pickup + 1 ketua handover), got %d", totalForItem)
	}
}

func TestPaket_RecipientAlsoManager_NoDuplicateNotification(t *testing.T) {
	pool := testutil.GetTestPool(t)
	ctx := context.Background()

	rtA := createTestRT(t, pool, ctx, fmt.Sprintf("RM-%d", time.Now().UnixNano()%10000))

	officerA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	officerB := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "keamanan")
	sekretarisA := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "sekretaris") // has notification.manage, different user

	// Recipient warga who is also Ketua RT (manager criteria) => must not get duplicate handover+pickup
	ketuaRecipientUser := createTestUserWithJabatan(t, pool, ctx, rtA, "pengurus", "ketua")
	ketuaRecipientResident := createTestResident(t, pool, ctx, rtA, ketuaRecipientUser)

	repo := security.NewRepository(pool)
	svc := security.NewService(repo)

	desc := "Paket Ketua"
	rep, err := svc.CreatePaketReport(ctx, rtA, officerA, ketuaRecipientResident, []security.CreatePaketItem{
		{Description: &desc, ImageKeys: []string{fmt.Sprintf("community/%s/paket/ketua.jpg", rtA)}},
	})
	if err != nil {
		t.Fatalf("create paket: %v", err)
	}
	if len(rep.Items) != 1 {
		t.Fatalf("expected 1 item")
	}
	itemID := rep.Items[0].ID
	proof := fmt.Sprintf("community/%s/paket/proof_ketua.jpg", rtA)
	if err := svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, itemID, "sudah_diambil", officerB, &proof); err != nil {
		t.Fatalf("handover failed: %v", err)
	}

	type notifRow struct {
		title     string
		eventType string
		dedup     string
		payload   []byte
	}
	rows, err := pool.Raw().QueryContext(ctx, `
		SELECT title, event_type, dedup_key, payload
		FROM notification_logs
		WHERE user_id = $1 AND (title = 'Paket Telah Diambil' OR title = 'Paket Telah Diserahkan')
		ORDER BY title
	`, ketuaRecipientUser)
	if err != nil {
		t.Fatalf("query recipient notifications: %v", err)
	}
	defer rows.Close()

	var got []notifRow
	for rows.Next() {
		var r notifRow
		if err := rows.Scan(&r.title, &r.eventType, &r.dedup, &r.payload); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 notification for recipient-manager user, got %d (%+v)", len(got), got)
	}
	if got[0].title != "Paket Telah Diambil" {
		t.Errorf("expected recipient to keep warga pickup notification, got title %q", got[0].title)
	}
	if got[0].eventType != "paket_picked_up" {
		t.Errorf("expected event_type paket_picked_up, got %q", got[0].eventType)
	}
	expectedPickupDedup := fmt.Sprintf("paket_pickup:%s:%s", itemID, ketuaRecipientUser)
	if got[0].dedup != expectedPickupDedup {
		t.Errorf("expected dedup %q, got %q", expectedPickupDedup, got[0].dedup)
	}
	payloadStr := string(got[0].payload)
	if len(payloadStr) == 0 || (!strings.Contains(payloadStr, fmt.Sprintf(`"item_id":"%s"`, itemID)) && !strings.Contains(payloadStr, fmt.Sprintf(`"item_id": "%s"`, itemID))) {
		t.Errorf("expected pickup payload to contain item_id %s, got %s", itemID, payloadStr)
	}

	var managerDup int
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan' AND dedup_key = $2
	`, ketuaRecipientUser, fmt.Sprintf("paket_diserahkan:%s:%s", itemID, ketuaRecipientUser)).Scan(&managerDup)
	if err != nil {
		t.Fatalf("count manager dup: %v", err)
	}
	if managerDup != 0 {
		t.Errorf("expected no manager handover notification for recipient-manager user, got %d", managerDup)
	}

	// Other managers still notified.
	var sekretarisCount int
	var sekretarisDedup string
	err = pool.Raw().QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(MAX(dedup_key), '')
		FROM notification_logs
		WHERE user_id = $1 AND title = 'Paket Telah Diserahkan'
	`, sekretarisA).Scan(&sekretarisCount, &sekretarisDedup)
	if err != nil {
		t.Fatalf("count sekretaris: %v", err)
	}
	if sekretarisCount != 1 {
		t.Errorf("expected 1 handover notification for other manager, got %d", sekretarisCount)
	}
	wantSekDedup := fmt.Sprintf("paket_diserahkan:%s:%s", itemID, sekretarisA)
	if sekretarisDedup != wantSekDedup {
		t.Errorf("expected sekretaris dedup %q, got %q", wantSekDedup, sekretarisDedup)
	}

	// Idempotent retry must not create duplicates for anyone.
	retryProof := fmt.Sprintf("community/%s/paket/proof_retry_ketua.jpg", rtA)
	if err := svc.UpdatePaketItemStatus(ctx, rtA, rep.ID, itemID, "sudah_diambil", officerB, &retryProof); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	var recipientTotalAfter, sekretarisTotalAfter int
	_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE user_id = $1 AND (title='Paket Telah Diambil' OR title='Paket Telah Diserahkan')`, ketuaRecipientUser).Scan(&recipientTotalAfter)
	_ = pool.Raw().QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_logs WHERE user_id = $1 AND title='Paket Telah Diserahkan'`, sekretarisA).Scan(&sekretarisTotalAfter)
	if recipientTotalAfter != 1 {
		t.Errorf("expected recipient-manager to still have exactly 1 notification after retry, got %d", recipientTotalAfter)
	}
	if sekretarisTotalAfter != 1 {
		t.Errorf("expected other manager to still have exactly 1 notification after retry, got %d", sekretarisTotalAfter)
	}
}
