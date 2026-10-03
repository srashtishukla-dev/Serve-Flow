package tests_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/technicians"
)

func TestCustomerPaginationSearchSortAndTenantIsolation(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "quality-a+" + testEmail()
	emailB := "quality-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	userA, tokenA := createServiceTestUser(t, server, emailA)
	userB, tokenB := createServiceTestUser(t, server, emailB)

	for _, name := range []string{"Ann Baker", "Anne Cole", "Zoe Young"} {
		input := customers.Input{Name: name}
		if err := input.Validate(); err != nil {
			t.Fatalf("validate customer fixture: %v", err)
		}
		if _, err := server.customers.Create(t.Context(), userA.OrganizationID, input); err != nil {
			t.Fatalf("create organization A customer fixture: %v", err)
		}
	}
	inputB := customers.Input{Name: "Ann Other"}
	if err := inputB.Validate(); err != nil {
		t.Fatalf("validate organization B customer fixture: %v", err)
	}
	customerB, err := server.customers.Create(t.Context(), userB.OrganizationID, inputB)
	if err != nil {
		t.Fatalf("create organization B customer fixture: %v", err)
	}

	var defaultResult struct {
		Customers  []customers.Customer `json:"customers"`
		Pagination struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}
	response := requestService(t, server, http.MethodGet, "/api/v1/customers?organization_id="+url.QueryEscape(userB.OrganizationID), tokenA, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("default customer list status = %d: %s", response.Code, response.Body.String())
	}
	if err := json.Unmarshal(response.Body.Bytes(), &defaultResult); err != nil {
		t.Fatalf("decode default customer list: %v", err)
	}
	if len(defaultResult.Customers) != 3 || defaultResult.Pagination.Page != 1 ||
		defaultResult.Pagination.Limit != 20 || defaultResult.Pagination.Total != 3 ||
		defaultResult.Pagination.TotalPages != 1 {
		t.Fatalf("default list or pagination = %+v, %+v", defaultResult.Customers, defaultResult.Pagination)
	}
	maximumPage := requestService(t, server, http.MethodGet, "/api/v1/customers?limit=100", tokenA, nil)
	if maximumPage.Code != http.StatusOK {
		t.Fatalf("maximum allowed page size returned %d: %s", maximumPage.Code, maximumPage.Body.String())
	}
	var maximumPageResult struct {
		Pagination struct {
			Limit int `json:"limit"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(maximumPage.Body.Bytes(), &maximumPageResult); err != nil || maximumPageResult.Pagination.Limit != 100 {
		t.Fatalf("maximum allowed page size metadata = %+v, error %v", maximumPageResult.Pagination, err)
	}
	for _, customer := range defaultResult.Customers {
		if customer.ID == customerB.ID {
			t.Fatal("forged organization_id query returned another tenant's customer")
		}
	}

	search := requestService(t, server, http.MethodGet, "/api/v1/customers?search=ann&page=2&limit=1&sort=name&order=asc", tokenA, nil)
	if search.Code != http.StatusOK {
		t.Fatalf("search/pagination status = %d: %s", search.Code, search.Body.String())
	}
	var searched struct {
		Customers  []customers.Customer `json:"customers"`
		Pagination struct {
			Page       int `json:"page"`
			Limit      int `json:"limit"`
			Total      int `json:"total"`
			TotalPages int `json:"total_pages"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(search.Body.Bytes(), &searched); err != nil {
		t.Fatalf("decode searched customers: %v", err)
	}
	if len(searched.Customers) != 1 || searched.Customers[0].Name != "Anne Cole" ||
		searched.Pagination.Page != 2 || searched.Pagination.Limit != 1 ||
		searched.Pagination.Total != 2 || searched.Pagination.TotalPages != 2 {
		t.Fatalf("searched page = %+v, pagination = %+v", searched.Customers, searched.Pagination)
	}

	noResults := requestService(t, server, http.MethodGet, "/api/v1/customers?search=not-found", tokenA, nil)
	if noResults.Code != http.StatusOK {
		t.Fatalf("empty search status = %d", noResults.Code)
	}
	var empty struct {
		Customers []customers.Customer `json:"customers"`
	}
	if err := json.Unmarshal(noResults.Body.Bytes(), &empty); err != nil || len(empty.Customers) != 0 {
		t.Fatalf("empty search results = %+v, error %v", empty.Customers, err)
	}

	for _, path := range []string{
		"/api/v1/customers?limit=101",
		"/api/v1/customers?page=0",
		"/api/v1/customers?sort=name%3B%20DROP%20TABLE%20users",
		"/api/v1/customers?order=sideways",
	} {
		invalid := requestService(t, server, http.MethodGet, path, tokenA, nil)
		if invalid.Code != http.StatusBadRequest {
			t.Errorf("%s returned %d, want 400", path, invalid.Code)
		}
	}

	if crossTenant := requestService(t, server, http.MethodGet, "/api/v1/customers/"+customerB.ID, tokenA, nil); crossTenant.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant customer ID returned %d, want 404", crossTenant.Code)
	}
	if organizationBList := requestService(t, server, http.MethodGet, "/api/v1/customers?search=ann", tokenB, nil); organizationBList.Code != http.StatusOK {
		t.Fatalf("organization B customer list status = %d", organizationBList.Code)
	}

	technicianInput := technicians.Input{Name: "Alex Technician"}
	if err := technicianInput.Validate(); err != nil {
		t.Fatalf("validate technician fixture: %v", err)
	}
	if _, err := server.technicians.Create(t.Context(), userA.OrganizationID, technicianInput); err != nil {
		t.Fatalf("create technician fixture: %v", err)
	}
	filteredTechnicians := requestService(t, server, http.MethodGet, "/api/v1/technicians?status=active&search=alex", tokenA, nil)
	if filteredTechnicians.Code != http.StatusOK {
		t.Fatalf("technician status/search filter returned %d: %s", filteredTechnicians.Code, filteredTechnicians.Body.String())
	}
	var technicianList struct {
		Technicians []technicians.Technician `json:"technicians"`
		Pagination  struct {
			Total int `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(filteredTechnicians.Body.Bytes(), &technicianList); err != nil {
		t.Fatalf("decode filtered technicians: %v", err)
	}
	if len(technicianList.Technicians) != 1 || technicianList.Pagination.Total != 1 {
		t.Fatalf("filtered technicians = %+v, pagination = %+v", technicianList.Technicians, technicianList.Pagination)
	}
	if invalidStatus := requestService(t, server, http.MethodGet, "/api/v1/technicians?status=unknown", tokenA, nil); invalidStatus.Code != http.StatusBadRequest {
		t.Fatalf("invalid technician status returned %d, want %d", invalidStatus.Code, http.StatusBadRequest)
	}
	if invalidID := requestService(t, server, http.MethodGet, "/api/v1/customers/not-a-uuid", tokenA, nil); invalidID.Code != http.StatusNotFound {
		t.Fatalf("invalid customer ID returned %d, want %d", invalidID.Code, http.StatusNotFound)
	}
}
