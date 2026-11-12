package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	httpx "social-finance/internal/http"
)

type contextKey string

const deviceContextKey contextKey = "security_device_context"

// GetDeviceContext extracts the authenticated Pos device context from an HTTP request.
func GetDeviceContext(r *http.Request) *DeviceContext {
	if r == nil {
		return nil
	}
	dc, _ := r.Context().Value(deviceContextKey).(*DeviceContext)
	return dc
}

// WithDeviceContext returns a copy of ctx with the provided DeviceContext attached.
func WithDeviceContext(ctx context.Context, dc *DeviceContext) context.Context {
	return context.WithValue(ctx, deviceContextKey, dc)
}

// GenerateRandomSecret returns a cryptographically random URL-safe secret string.
func GenerateRandomSecret(numBytes int) (string, error) {
	if numBytes <= 0 {
		numBytes = 32
	}
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken computes the lowercase hex SHA-256 hash of a raw token/secret.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// SplitCompositeToken parses "<device_id>.<secret>" and validates UUID format.
func SplitCompositeToken(raw string) (deviceID, secret string, ok bool) {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	id := strings.TrimSpace(parts[0])
	sec := strings.TrimSpace(parts[1])
	if id == "" || sec == "" {
		return "", "", false
	}
	if _, err := uuid.Parse(id); err != nil {
		return "", "", false
	}
	return id, sec, true
}

func extractDeviceCredential(r *http.Request) string {
	if tok := strings.TrimSpace(r.Header.Get("X-Device-Token")); tok != "" {
		return tok
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Device ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}

// DeviceAuthMiddleware authenticates Security Pos Devices using hashed device tokens
// bound to a single active, allowed RT. User JWT Bearer tokens are intentionally rejected here.
func DeviceAuthMiddleware(repo *Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken := extractDeviceCredential(r)
			deviceID, secret, ok := SplitCompositeToken(rawToken)
			if !ok {
				httpx.WriteUnauthorized(w, "missing or malformed security device credential")
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			rec, err := repo.GetDeviceAuthRecord(ctx, deviceID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					httpx.WriteUnauthorized(w, "invalid security device credential")
					return
				}
				httpx.WriteInternalServerError(w, "failed to verify device credential")
				return
			}

			if !rec.IsActive || rec.RevokedAt != nil || !rec.TokenHash.Valid || rec.TokenHash.String == "" {
				httpx.WriteUnauthorized(w, "security device is inactive or revoked")
				return
			}

			if !rec.RTActive {
				httpx.WriteForbidden(w, "device RT is disabled or not allowed")
				return
			}

			providedHash := HashToken(secret)
			if subtle.ConstantTimeCompare([]byte(providedHash), []byte(rec.TokenHash.String)) != 1 {
				httpx.WriteUnauthorized(w, "invalid security device credential")
				return
			}

			// Record activity timestamp (best-effort; do not block request on telemetry update failure).
			_ = repo.TouchDeviceActivity(ctx, rec.DeviceID, NowJakarta(nil).UTC())

			dc := &DeviceContext{
				DeviceID:      rec.DeviceID,
				RTID:          rec.RTID,
				RTName:        rec.RTName,
				RTCode:        rec.RTCode,
				RWID:          rec.RWID.String,
				DeviceName:    rec.DeviceName,
				LocationLabel: rec.LocationLabel,
			}
			next.ServeHTTP(w, r.WithContext(WithDeviceContext(r.Context(), dc)))
		})
	}
}
