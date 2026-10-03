package pagination

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Query struct {
	Page   int
	Limit  int
	Offset int
	Search string
	Status string
	IsRead *bool
	Sort   string
	Order  string
}

type Metadata struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func Parse(values url.Values, filters map[string]bool, sortable []string) (Query, error) {
	query := Query{Page: 1, Limit: DefaultLimit, Sort: "created_at", Order: "desc"}
	allowed := map[string]bool{
		"page": true, "limit": true, "sort": true, "order": true,
		"organization_id": true,
	}
	for key := range filters {
		allowed[key] = true
	}
	for key, entries := range values {
		if !allowed[key] {
			return Query{}, fmt.Errorf("unsupported query parameter: %s", key)
		}
		if len(entries) != 1 {
			return Query{}, fmt.Errorf("query parameter %s must be provided once", key)
		}
	}

	var err error
	if value := values.Get("page"); value != "" {
		query.Page, err = strconv.Atoi(value)
		if err != nil || query.Page < 1 || query.Page > 1_000_000 {
			return Query{}, fmt.Errorf("page must be between 1 and 1000000")
		}
	}
	if value := values.Get("limit"); value != "" {
		query.Limit, err = strconv.Atoi(value)
		if err != nil || query.Limit < 1 || query.Limit > MaxLimit {
			return Query{}, fmt.Errorf("limit must be between 1 and %d", MaxLimit)
		}
	}
	if query.Page > int(^uint(0)>>1)/query.Limit {
		return Query{}, fmt.Errorf("page and limit exceed the supported range")
	}
	query.Offset = (query.Page - 1) * query.Limit

	if filters["search"] {
		query.Search = strings.TrimSpace(values.Get("search"))
		if len(query.Search) > 100 {
			return Query{}, fmt.Errorf("search must be 100 characters or fewer")
		}
	}
	if filters["status"] {
		query.Status = strings.ToUpper(strings.TrimSpace(values.Get("status")))
		if len(query.Status) > 32 {
			return Query{}, fmt.Errorf("status is too long")
		}
	}
	if filters["is_read"] && values.Has("is_read") {
		value, parseErr := strconv.ParseBool(values.Get("is_read"))
		if parseErr != nil {
			return Query{}, fmt.Errorf("is_read must be true or false")
		}
		query.IsRead = &value
	}

	if value := values.Get("sort"); value != "" {
		valid := false
		for _, field := range sortable {
			if value == field {
				valid = true
				break
			}
		}
		if !valid {
			return Query{}, fmt.Errorf("sort field is not supported")
		}
		query.Sort = value
	} else if len(sortable) > 0 {
		query.Sort = sortable[0]
	}
	if value := values.Get("order"); value != "" {
		query.Order = strings.ToLower(value)
		if query.Order != "asc" && query.Order != "desc" {
			return Query{}, fmt.Errorf("order must be asc or desc")
		}
	}
	return query, nil
}

func NewMetadata(query Query, total int) Metadata {
	totalPages := 0
	if total > 0 {
		totalPages = (total + query.Limit - 1) / query.Limit
	}
	return Metadata{Page: query.Page, Limit: query.Limit, Total: total, TotalPages: totalPages}
}

func OrderBy(query Query, columns map[string]string) string {
	column, ok := columns[query.Sort]
	if !ok {
		panic("pagination sort field was not validated")
	}
	direction := "DESC"
	if query.Order == "asc" {
		direction = "ASC"
	}
	return column + " " + direction
}
