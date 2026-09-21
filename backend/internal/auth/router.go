package auth

import "net/http"

// authRouter mirrors chi.Router for the auth routes.
type authRouter interface {
	POST(path string, handler http.HandlerFunc)
	GET(path string, handler http.HandlerFunc)
	DELETE(path string, handler http.HandlerFunc)
	PATCH(path string, handler http.HandlerFunc)
	PUT(path string, handler http.HandlerFunc)
}
