// Package pagination parses page/page_size query parameters.
package pagination

import (
	"net/http"
	"strconv"

	"referralautomation/internal/platform/apperr"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

type Params struct {
	Page     int
	PageSize int
}

func (p Params) Limit() int32  { return int32(p.PageSize) }
func (p Params) Offset() int32 { return int32((p.Page - 1) * p.PageSize) }

type Meta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewMeta(p Params, total int64) Meta {
	pages := int((total + int64(p.PageSize) - 1) / int64(p.PageSize))
	return Meta{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPages: pages}
}

func Parse(r *http.Request) (Params, error) {
	p := Params{Page: 1, PageSize: DefaultPageSize}
	q := r.URL.Query()

	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1_000_000 {
			return p, apperr.BadRequest("page must be a positive integer")
		}
		p.Page = n
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > MaxPageSize {
			return p, apperr.BadRequest("page_size must be between 1 and " + strconv.Itoa(MaxPageSize))
		}
		p.PageSize = n
	}
	return p, nil
}
