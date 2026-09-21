package http

import (
	"net/http"
	"strconv"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// PaginationParams holds parsed pagination query parameters.
type PaginationParams struct {
	Page    int
	PerPage int
}

// ParsePagination extracts and validates page/page_size from query parameters.
// Returns zero values when no pagination params are provided (defaults applied by caller).
func ParsePagination(r *http.Request) PaginationParams {
	page := DefaultPage
	perPage := DefaultPageSize

	p := r.URL.Query().Get("page")
	if p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	pp := r.URL.Query().Get("page_size")
	if pp != "" {
		if n, err := strconv.Atoi(pp); err == nil && n > 0 {
			pagePer := n
			if pagePer > MaxPageSize {
				pagePer = MaxPageSize
			}
			perPage = pagePer
		}
	}

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
	}
}
