package pagination

import (
	"net/url"
	"strings"
	"testing"
)

func TestParsePaginationAndFilters(t *testing.T) {
	query, err := Parse(url.Values{"search": {"Jordan"}, "status": {"active"}, "page": {"2"}, "limit": {"10"}, "sort": {"name"}, "order": {"asc"}}, map[string]bool{"search": true, "status": true}, []string{"created_at", "name"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if query.Page != 2 || query.Limit != 10 || query.Offset != 10 || query.Search != "Jordan" || query.Status != "ACTIVE" || query.Sort != "name" || query.Order != "asc" {
		t.Fatalf("Parse() = %+v", query)
	}
}

func TestParseRejectsInvalidPaginationAndSort(t *testing.T) {
	for _, values := range []url.Values{
		{"page": {"0"}},
		{"page": {"not-a-number"}},
		{"limit": {"101"}},
		{"limit": {"0"}},
		{"sort": {"name; DROP TABLE users"}},
		{"order": {"sideways"}},
		{"page": {"1"}, "limit": {"10"}, "search": {"query"}}, // filter not enabled
	} {
		if _, err := Parse(values, nil, []string{"created_at", "name"}); err == nil {
			t.Errorf("Parse(%v) unexpectedly succeeded", values)
		}
	}
}

func TestMetadataAndSortAreBounded(t *testing.T) {
	query, err := Parse(url.Values{"limit": {"10"}, "sort": {"name"}, "order": {"asc"}}, nil, []string{"name"})
	if err != nil {
		t.Fatal(err)
	}
	metadata := NewMetadata(query, 21)
	if metadata.TotalPages != 3 || metadata.Total != 21 {
		t.Fatalf("metadata = %+v", metadata)
	}
	if got := OrderBy(query, map[string]string{"name": "customer.name"}); got != "customer.name ASC" {
		t.Fatalf("OrderBy() = %q", got)
	}
	if strings.Contains(OrderBy(query, map[string]string{"name": "customer.name"}), ";") {
		t.Fatal("whitelisted order contains unexpected SQL")
	}
}
