package tests_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/serveflow/serveflow/backend/internal/analytics"
	"github.com/serveflow/serveflow/backend/internal/customers"
)

func TestAnalyticsSummaryIsAccurateAndTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "analytics-a+")
	dataB := newAppointmentTestData(t, server, "analytics-b+")
	emptyEmail := "analytics-empty+" + testEmail()
	t.Cleanup(func() { cleanupUser(t, server, emptyEmail) })
	_, emptyToken := createServiceTestUser(t, server, emptyEmail)

	var currentDate, currentMonthDate, dueDate, currentMonth string
	if err := server.pool.QueryRow(context.Background(), `
		SELECT CURRENT_DATE::text,
		       (date_trunc('month', CURRENT_DATE)::date + 1)::text,
		       (CURRENT_DATE + 30)::text,
		       to_char(date_trunc('month', CURRENT_DATE), 'YYYY-MM')
	`).Scan(&currentDate, &currentMonthDate, &dueDate, &currentMonth); err != nil {
		t.Fatalf("read PostgreSQL test dates: %v", err)
	}

	additionalCustomerInput := customers.Input{Name: "Second analytics customer"}
	if err := additionalCustomerInput.Validate(); err != nil {
		t.Fatalf("validate analytics customer: %v", err)
	}
	if _, err := server.customers.Create(context.Background(), dataA.organizationID, additionalCustomerInput); err != nil {
		t.Fatalf("create second analytics customer: %v", err)
	}

	completedInput := appointmentRequest(dataA, dataA.customerID, dataA.serviceID, dataA.technicianID, "10:00", "11:00")
	completedInput["appointment_date"] = currentMonthDate
	completedResponse := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, completedInput)
	if completedResponse.Code != http.StatusCreated {
		t.Fatalf("create completed appointment fixture: %d %s", completedResponse.Code, completedResponse.Body.String())
	}
	completedAppointment := decodeCreatedAppointment(t, completedResponse)
	completedUpdate := appointmentRequest(dataA, dataA.customerID, dataA.serviceID, dataA.technicianID, "10:00", "11:00")
	completedUpdate["appointment_date"] = currentMonthDate
	completedUpdate["status"] = "COMPLETED"
	if response := requestAppointment(t, server, http.MethodPut, "/api/v1/appointments/"+completedAppointment.ID, dataA.token, completedUpdate); response.Code != http.StatusOK {
		t.Fatalf("complete appointment fixture: %d %s", response.Code, response.Body.String())
	}
	upcomingInput := appointmentRequest(dataA, dataA.customerID, dataA.serviceID, dataA.technicianID, "12:00", "13:00")
	upcomingInput["appointment_date"] = "2030-01-15"
	if response := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", dataA.token, upcomingInput); response.Code != http.StatusCreated {
		t.Fatalf("create upcoming appointment fixture: %d %s", response.Code, response.Body.String())
	}

	paidInvoiceInput := invoiceRequest(dataA.customerID, "")
	paidInvoiceInput["issue_date"] = currentDate
	paidInvoiceInput["due_date"] = dueDate
	paidInvoiceResponse := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, paidInvoiceInput)
	if paidInvoiceResponse.Code != http.StatusCreated {
		t.Fatalf("create paid invoice fixture: %d %s", paidInvoiceResponse.Code, paidInvoiceResponse.Body.String())
	}
	paidInvoice := decodeInvoiceResponse(t, paidInvoiceResponse)
	paymentInput := validPaymentPayload("1430.00")
	paymentInput["payment_date"] = currentDate
	if response := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+paidInvoice.ID+"/payments", dataA.token, paymentInput); response.Code != http.StatusCreated {
		t.Fatalf("record analytics payment fixture: %d %s", response.Code, response.Body.String())
	}
	unpaidInvoiceInput := invoiceRequest(dataA.customerID, "")
	unpaidInvoiceInput["issue_date"] = currentDate
	unpaidInvoiceInput["due_date"] = dueDate
	if response := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", dataA.token, unpaidInvoiceInput); response.Code != http.StatusCreated {
		t.Fatalf("create pending invoice fixture: %d %s", response.Code, response.Body.String())
	}
	if _, err := server.pool.Exec(context.Background(), `
		INSERT INTO invoices (organization_id, customer_id, invoice_number, status, issue_date, due_date, subtotal, tax, total)
		VALUES ($1::uuid, $2::uuid, 'DRAFT-ANALYTICS', 'DRAFT', $3::date, $4::date, 100, 0, 100)
	`, dataA.organizationID, dataA.customerID, currentDate, dueDate); err != nil {
		t.Fatalf("create draft invoice fixture: %v", err)
	}

	unauthenticated := requestAnalytics(t, server, "", "")
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated analytics status = %d, want %d", unauthenticated.Code, http.StatusUnauthorized)
	}

	responseA := requestAnalytics(t, server, dataA.token, "?organization_id="+dataB.organizationID)
	if responseA.Code != http.StatusOK {
		t.Fatalf("organization A analytics status = %d: %s", responseA.Code, responseA.Body.String())
	}
	var summaryA analytics.Summary
	if err := json.Unmarshal(responseA.Body.Bytes(), &summaryA); err != nil {
		t.Fatalf("decode organization A analytics: %v", err)
	}
	metricsA := summaryA.Metrics
	if metricsA.Customers != 2 || metricsA.Services != 1 || metricsA.Appointments != 2 ||
		metricsA.UpcomingAppointments != 1 || metricsA.CompletedAppointments != 1 ||
		metricsA.CancelledAppointments != 0 || metricsA.NoShowAppointments != 0 ||
		metricsA.TotalTechnicians != 1 || metricsA.ActiveTechnicians != 1 || metricsA.TotalInvoices != 3 ||
		metricsA.PaidInvoices != 1 || metricsA.PendingInvoices != 1 ||
		metricsA.TotalPayments != 1 || metricsA.TotalInvoicedAmount.String() != "2860.00" ||
		metricsA.TotalRevenue.String() != "1430.00" || metricsA.OutstandingBalance.String() != "1430.00" {
		t.Fatalf("organization A metrics = %+v", metricsA)
	}
	if len(summaryA.PopularServices) != 1 || summaryA.PopularServices[0].Bookings != 2 ||
		len(summaryA.TopCustomers) != 1 || summaryA.TopCustomers[0].Bookings != 2 ||
		len(summaryA.TechnicianWorkload) != 1 || summaryA.TechnicianWorkload[0].CompletedBookings != 1 {
		t.Fatalf("organization A insights = services %+v, customers %+v, technicians %+v",
			summaryA.PopularServices, summaryA.TopCustomers, summaryA.TechnicianWorkload)
	}
	if len(summaryA.Trends.Months) != 6 || summaryA.Trends.StartMonth == "" || summaryA.Trends.EndMonth == "" {
		t.Fatalf("trend range = %+v", summaryA.Trends)
	}
	var currentMonthTrend *analytics.MonthlyTrend
	for index := range summaryA.Trends.Months {
		if summaryA.Trends.Months[index].Month == currentMonth {
			currentMonthTrend = &summaryA.Trends.Months[index]
			break
		}
	}
	if currentMonthTrend == nil || currentMonthTrend.Revenue.String() != "1430.00" || currentMonthTrend.Appointments != 1 {
		t.Fatalf("current-month trend = %+v", currentMonthTrend)
	}

	responseB := requestAnalytics(t, server, dataB.token, "")
	var summaryB analytics.Summary
	if responseB.Code != http.StatusOK || json.Unmarshal(responseB.Body.Bytes(), &summaryB) != nil {
		t.Fatalf("organization B analytics failed: %d %s", responseB.Code, responseB.Body.String())
	}
	if summaryB.Metrics.Customers != 1 || summaryB.Metrics.Appointments != 0 ||
		summaryB.Metrics.ActiveTechnicians != 1 || summaryB.Metrics.TotalInvoices != 0 ||
		summaryB.Metrics.TotalRevenue.String() != "0" {
		t.Fatalf("organization B analytics includes other tenant data: %+v", summaryB.Metrics)
	}

	emptyResponse := requestAnalytics(t, server, emptyToken, "")
	var emptySummary analytics.Summary
	if emptyResponse.Code != http.StatusOK || json.Unmarshal(emptyResponse.Body.Bytes(), &emptySummary) != nil {
		t.Fatalf("empty organization analytics failed: %d %s", emptyResponse.Code, emptyResponse.Body.String())
	}
	if emptySummary.Metrics.Customers != 0 || emptySummary.Metrics.Services != 0 || emptySummary.Metrics.Appointments != 0 ||
		emptySummary.Metrics.UpcomingAppointments != 0 || emptySummary.Metrics.CompletedAppointments != 0 ||
		emptySummary.Metrics.ActiveTechnicians != 0 || emptySummary.Metrics.TotalInvoices != 0 ||
		emptySummary.Metrics.PaidInvoices != 0 || emptySummary.Metrics.PendingInvoices != 0 ||
		emptySummary.Metrics.TotalRevenue.String() != "0" || len(emptySummary.Trends.Months) != 6 {
		t.Fatalf("empty organization analytics = %+v", emptySummary)
	}
}

func TestAnalyticsActivitySupportsRangesAndIsTenantScoped(t *testing.T) {
	server := newAuthTestServer(t)
	dataA := newAppointmentTestData(t, server, "analytics-activity-a+")
	dataB := newAppointmentTestData(t, server, "analytics-activity-b+")
	var currentDate string
	var currentDayOfMonth int
	if err := server.pool.QueryRow(context.Background(), `
		SELECT CURRENT_DATE::text, EXTRACT(DAY FROM CURRENT_DATE)::int
	`).Scan(&currentDate, &currentDayOfMonth); err != nil {
		t.Fatalf("read PostgreSQL current date: %v", err)
	}
	for _, data := range []appointmentTestData{dataA, dataB} {
		input := appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00")
		input["appointment_date"] = currentDate
		if response := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token, input); response.Code != http.StatusCreated {
			t.Fatalf("create current-day activity fixture: %d %s", response.Code, response.Body.String())
		}
	}

	todayResponse := requestAnalyticsActivity(t, server, dataA.token, "?range=today&organization_id="+dataB.organizationID)
	if todayResponse.Code != http.StatusOK {
		t.Fatalf("today activity status = %d: %s", todayResponse.Code, todayResponse.Body.String())
	}
	var today analytics.Activity
	if err := json.Unmarshal(todayResponse.Body.Bytes(), &today); err != nil {
		t.Fatalf("decode today activity: %v", err)
	}
	if today.Range != analytics.RangeToday || today.StartDate != currentDate || today.EndDate != currentDate ||
		len(today.Days) != 1 || today.Days[0].Appointments != 1 {
		t.Fatalf("tenant-scoped today activity = %+v", today)
	}

	for _, dateRange := range []struct {
		name      string
		days      int
		startDate string
	}{
		{name: analytics.RangeLast7Days, days: 7},
		{name: analytics.RangeLast30Days, days: 30},
		{name: analytics.RangeCurrentMonth, days: currentDayOfMonth, startDate: currentDate[:8] + "01"},
	} {
		response := requestAnalyticsActivity(t, server, dataA.token, "?range="+dateRange.name)
		var activity analytics.Activity
		if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &activity) != nil {
			t.Fatalf("%s activity failed: %d %s", dateRange.name, response.Code, response.Body.String())
		}
		if len(activity.Days) != dateRange.days || activity.EndDate != currentDate ||
			(dateRange.startDate != "" && activity.StartDate != dateRange.startDate) {
			t.Fatalf("%s activity has %d days from %s through %s", dateRange.name, len(activity.Days), activity.StartDate, activity.EndDate)
		}
	}

	if response := requestAnalyticsActivity(t, server, dataA.token, "?range=all_time"); response.Code != http.StatusBadRequest {
		t.Fatalf("unsupported activity range status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func requestAnalytics(t *testing.T, server *authTestServer, token, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/summary"+query, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	return recorder
}

func requestAnalyticsActivity(t *testing.T, server *authTestServer, token, query string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/analytics/activity"+query, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	return recorder
}
