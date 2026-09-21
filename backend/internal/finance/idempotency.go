package finance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"social-finance/internal/database"
)

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// HandleIdempotentRequest checks if an Idempotency-Key exists for this RT.
// If it exists, writes the cached response and returns true (meaning response already handled).
// If not, returns false, along with a finisher function to call after writing the response.
func HandleIdempotentRequest(ctx context.Context, pool *database.Pool, rtID, userID string, w http.ResponseWriter, r *http.Request) (bool, *responseRecorder, func()) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		return false, nil, func() {}
	}

	// Check if cached
	tx, err := pool.BeginTx(ctx)
	if err == nil {
		cached, err := IdempotencyGet(ctx, tx, rtID, key)
		_ = tx.Rollback()
		if err == nil && cached != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache-Lookup", "HIT")
			w.WriteHeader(cached.ResponseCode)
			_, _ = w.Write([]byte(cached.ResponseBody))
			return true, nil, func() {}
		}
	}

	// Record response
	rec := &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           new(bytes.Buffer),
	}

	// Calculate request body hash if available
	var reqHash string
	if r.Body != nil {
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		h := sha256.Sum256(bodyBytes)
		reqHash = hex.EncodeToString(h[:])
	}

	finish := func() {
		if rec.statusCode > 0 && rec.statusCode < 500 {
			saveTx, err := pool.BeginTx(context.Background())
			if err == nil {
				_ = IdempotencySave(context.Background(), saveTx, rtID, key, userID, r.URL.Path, reqHash, rec.statusCode, rec.body.String())
				_ = saveTx.Commit()
			}
		}
	}

	return false, rec, finish
}
