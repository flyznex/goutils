package httpext

import (
	"net/http"
	"strconv"
)

// ParsePagination use to get limit and offset from url query string (limit, offset)
// Return: limit, offset
func ParsePagination(r *http.Request, defaultOffset, defaultLimit, maxLimit int) (int, int) {
	var offset, limit int

	if offsetParam := r.URL.Query().Get("offset"); offsetParam == "" {
		offset = defaultOffset
	} else {
		if offset64, err := strconv.ParseInt(offsetParam, 10, 64); err != nil {
			offset = defaultOffset
		} else {
			offset = int(offset64)
		}
	}

	if limitParam := r.URL.Query().Get("limit"); limitParam == "" {
		limit = defaultLimit
	} else {
		if limit64, err := strconv.ParseInt(limitParam, 10, 64); err != nil {
			limit = defaultLimit
		} else {
			limit = int(limit64)
		}
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	if limit < 0 {
		limit = 0
	}

	if offset < 0 {
		offset = 0
	}

	return limit, offset
}

// ParsePaginationFromIndexSize use to get limit and offset from url query string (index, size)
// Return: limit, offset
func ParsePaginationFromIndexSize(r *http.Request, defaultOffset, defaultLimit, maxLimit int) (int, int) {
	var offset, limit int

	if sizeParam := r.URL.Query().Get("size"); sizeParam == "" {
		limit = defaultLimit
	} else {
		if size, err := strconv.ParseInt(sizeParam, 10, 64); err != nil {
			limit = defaultLimit
		} else {
			limit = int(size)
		}
	}
	if indexParam := r.URL.Query().Get("index"); indexParam == "" {
		offset = defaultOffset
	} else {
		if index, err := strconv.ParseInt(indexParam, 10, 64); err != nil {
			offset = defaultOffset
		} else {
			offset = (int(index) - 1) * limit
		}
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	if limit < 0 {
		limit = 0
	}

	if offset < 0 {
		offset = 0
	}

	return limit, offset
}
