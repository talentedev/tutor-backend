package checkr

import (
	"net/url"

	"github.com/google/go-querystring/query"
)

// Pagination is enabled for endpoints that return a list of records.
type Pagination struct {
	NextHref     string `json:"next_href"`
	PreviousHref string `json:"previous_href"`
	Count        int    `json:"count"`
}

// PaginationParams store the two parameters that control pagination: page, which specifies the
// page number to retrieve, and per_page, which indicates how many records each page should contain.
type PaginationParams struct {
	PerPage int `json:"per_page" url:"per_page,omitempty"` // between 0 and 100
	Page    int `json:"page" url:"page,omitempty"`         // greater than or equal to 1
}

func (p PaginationParams) SetValues(values *url.Values) error {
	v, err := query.Values(&p)
	if err != nil {
		return err
	}
	for k := range v {
		values.Set(k, v.Get(k))
	}
	return nil
}
