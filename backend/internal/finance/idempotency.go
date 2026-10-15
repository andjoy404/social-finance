package finance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"social-finance/internal/database"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.body.Write(b)
}

func (r *responseRecorder) replace(status int, body string) {
	r.statusCode = status
	r.body.Reset()
	_, _ = r.body.WriteString(body)
}

func (r *responseRecorder) flush() {
	status := r.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	r.ResponseWriter.WriteHeader(status)
	_, _ = r.ResponseWriter.Write(r.body.Bytes())
}

type idempotencySession struct {
	ctx         context.Context
	conn        *sql.Conn
	rtID        string
	key         string
	userID      string
	requestPath string
	requestHash string
	recorder    *responseRecorder
	mu          sync.Mutex
	saved       bool
	finished    bool
}

func (s *idempotencySession) BeginTx(ctx context.Context, pool *database.Pool) (*sql.Tx, error) {
	if s != nil {
		return s.conn.BeginTx(ctx, nil)
	}
	return pool.BeginTx(ctx)
}

func (s *idempotencySession) Save(tx *sql.Tx) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.saved || s.recorder.statusCode >= http.StatusInternalServerError {
		return nil
	}
	if err := IdempotencySave(s.ctx, tx, s.rtID, s.key, s.userID, s.requestPath, s.requestHash,
		s.recorder.statusCode, s.recorder.body.String()); err != nil {
		return err
	}
	s.saved = true
	return nil
}

func (s *idempotencySession) Finish() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true

	if !s.saved && s.recorder.statusCode < http.StatusInternalServerError {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx, err := s.conn.BeginTx(ctx, nil)
		if err == nil {
			err = IdempotencySave(ctx, tx, s.rtID, s.key, s.userID, s.requestPath, s.requestHash,
				s.recorder.statusCode, s.recorder.body.String())
			if err == nil {
				err = tx.Commit()
			} else {
				_ = tx.Rollback()
			}
			if err != nil {
				_ = tx.Rollback()
			}
		}
		if err != nil {
			slog.Error("save idempotency response failed", "error", err, "request_path", s.requestPath)
			s.recorder.replace(http.StatusInternalServerError, `{"error":{"code":"internal_error","message":"could not persist idempotent response"}}`)
		}
	}

	s.recorder.flush()
	if _, err := s.conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, idempotencyLockID(s.rtID, s.key)); err != nil {
		slog.Error("release idempotency lock failed", "error", err, "request_path", s.requestPath)
	}
	if err := s.conn.Close(); err != nil {
		slog.Error("close idempotency connection failed", "error", err, "request_path", s.requestPath)
	}
}

// HandleIdempotentRequest serializes a key across API instances with a PostgreSQL
// session advisory lock. Successful payment handlers save the response inside
// the payment transaction by calling session.Save(tx).
func HandleIdempotentRequest(ctx context.Context, pool *database.Pool, rtID, userID string, w http.ResponseWriter, r *http.Request) (bool, *responseRecorder, *idempotencySession) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		return false, nil, nil
	}

	var requestHash string
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			writeIdempotencyError(w, http.StatusBadRequest, "invalid_request", "could not read request body")
			return true, nil, nil
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		hash := sha256.Sum256(bodyBytes)
		requestHash = hex.EncodeToString(hash[:])
	}

	lockID := idempotencyLockID(rtID, key)
	var conn *sql.Conn
	var err error
	for {
		conn, err = pool.Raw().Conn(ctx)
		if err != nil {
			writeIdempotencyError(w, http.StatusInternalServerError, "internal_error", "idempotency storage unavailable")
			return true, nil, nil
		}
		var acquired bool
		if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, lockID).Scan(&acquired); err != nil {
			releaseIdempotencyLock(conn, lockID)
			writeIdempotencyError(w, http.StatusInternalServerError, "internal_error", "could not acquire idempotency lock")
			return true, nil, nil
		}
		if acquired {
			break
		}
		_ = conn.Close()
		select {
		case <-ctx.Done():
			writeIdempotencyError(w, http.StatusRequestTimeout, "idempotency_timeout", "timed out waiting for idempotency key")
			return true, nil, nil
		case <-time.After(10 * time.Millisecond):
		}
	}

	var cached CachedResponse
	err = conn.QueryRowContext(ctx,
		`SELECT user_id::text, request_path, request_hash, response_code, response_body
		 FROM idempotency_keys WHERE rt_id = $1 AND key = $2`,
		rtID, key,
	).Scan(&cached.UserID, &cached.RequestPath, &cached.RequestHash, &cached.ResponseCode, &cached.ResponseBody)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		releaseIdempotencyLock(conn, lockID)
		writeIdempotencyError(w, http.StatusInternalServerError, "internal_error", "could not read idempotency record")
		return true, nil, nil
	}
	if err == nil {
		if cached.UserID != userID || cached.RequestPath != r.URL.Path || cached.RequestHash != requestHash {
			releaseIdempotencyLock(conn, lockID)
			writeIdempotencyError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
			return true, nil, nil
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache-Lookup", "HIT")
		w.WriteHeader(cached.ResponseCode)
		_, _ = io.WriteString(w, cached.ResponseBody)
		releaseIdempotencyLock(conn, lockID)
		return true, nil, nil
	}

	recorder := &responseRecorder{
		ResponseWriter: w,
		body:           new(bytes.Buffer),
	}
	return false, recorder, &idempotencySession{
		ctx:         ctx,
		conn:        conn,
		rtID:        rtID,
		key:         key,
		userID:      userID,
		requestPath: r.URL.Path,
		requestHash: requestHash,
		recorder:    recorder,
	}
}

func idempotencyLockID(rtID, key string) int64 {
	hash := sha256.Sum256([]byte(rtID + "\x00" + key))
	return int64(binary.BigEndian.Uint64(hash[:8]))
}

func releaseIdempotencyLock(conn *sql.Conn, lockID int64) {
	releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(releaseCtx, `SELECT pg_advisory_unlock($1)`, lockID); err != nil {
		slog.Error("release idempotency lock failed", "error", err)
	}
	if err := conn.Close(); err != nil {
		slog.Error("close idempotency connection failed", "error", err)
	}
}

func writeIdempotencyError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
