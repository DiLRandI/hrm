package shared

import (
	"net/http"
	"strconv"
)

type Pagination struct {
	Limit  int
	Offset int
}

func ParsePagination(r *http.Request, defaultLimit, maxLimit int) Pagination {
	limit := defaultLimit
	offset := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	if maxLimit > 0 && limit > maxLimit {
		limit = maxLimit
	}
	return Pagination{Limit: limit, Offset: offset}
}
