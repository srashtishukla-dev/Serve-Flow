package tests_test

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/notifications"
	"github.com/serveflow/serveflow/backend/internal/routes"
	"github.com/serveflow/serveflow/backend/internal/users"
)

func TestNotificationsAreUserAndOrganizationScoped(t *testing.T) {
	server := newAuthTestServer(t)
	emailA := "notification-a+" + testEmail()
	emailB := "notification-b+" + testEmail()
	t.Cleanup(func() {
		cleanupUser(t, server, emailA)
		cleanupUser(t, server, emailB)
	})
	userA, tokenA := createServiceTestUser(t, server, emailA)
	userB, tokenB := createServiceTestUser(t, server, emailB)
	input := notifications.Input{Type: "appointment.created", Title: "Appointment scheduled", Message: "A new appointment was scheduled."}
	created, err := server.notifications.Create(context.Background(), userA.OrganizationID, userA.ID, input)
	if err != nil {
		t.Fatalf("create notification: %v", err)
	}
	if created.IsRead || created.UserID != userA.ID || created.OrganizationID != userA.OrganizationID {
		t.Fatalf("created notification = %+v", created)
	}
	if _, err := server.notifications.Create(context.Background(), userA.OrganizationID, userB.ID, input); err != notifications.ErrUserNotFound {
		t.Fatalf("create notification for another organization's user error = %v, want ErrUserNotFound", err)
	}

	listA := requestNotification(t, server, http.MethodGet, "/api/v1/notifications?organization_id="+userB.OrganizationID, tokenA)
	var response struct {
		Notifications []notifications.Notification `json:"notifications"`
	}
	if listA.Code != http.StatusOK || json.Unmarshal(listA.Body.Bytes(), &response) != nil {
		t.Fatalf("list notifications failed: %d %s", listA.Code, listA.Body.String())
	}
	if len(response.Notifications) != 1 || response.Notifications[0].ID != created.ID {
		t.Fatalf("user A notifications = %+v, want only the created notification", response.Notifications)
	}
	listB := requestNotification(t, server, http.MethodGet, "/api/v1/notifications", tokenB)
	if listB.Code != http.StatusOK || json.Unmarshal(listB.Body.Bytes(), &response) != nil || len(response.Notifications) != 0 {
		t.Fatalf("user B could see user A notification: %d %s", listB.Code, listB.Body.String())
	}

	if recorder := requestNotification(t, server, http.MethodGet, "/api/v1/notifications", ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if recorder := requestNotification(t, server, http.MethodPut, "/api/v1/notifications/invalid/read", tokenA); recorder.Code != http.StatusNotFound {
		t.Fatalf("invalid notification ID status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if recorder := requestNotification(t, server, http.MethodPut, "/api/v1/notifications/"+created.ID+"/read", ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated mark-read status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if recorder := requestNotification(t, server, http.MethodPut, "/api/v1/notifications/"+created.ID+"/read", tokenB); recorder.Code != http.StatusNotFound {
		t.Fatalf("cross-user mark-read status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if recorder := requestNotification(t, server, http.MethodPost, "/api/v1/notifications", tokenA); recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("client notification creation status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
	}

	marked := requestNotification(t, server, http.MethodPut, "/api/v1/notifications/"+created.ID+"/read", tokenA)
	var markedResponse struct {
		Notification notifications.Notification `json:"notification"`
	}
	if marked.Code != http.StatusOK || json.Unmarshal(marked.Body.Bytes(), &markedResponse) != nil || !markedResponse.Notification.IsRead {
		t.Fatalf("mark notification read failed: %d %s", marked.Code, marked.Body.String())
	}
	items, err := server.notifications.List(context.Background(), userA.OrganizationID, userA.ID)
	if err != nil || len(items) != 1 || !items[0].IsRead {
		t.Fatalf("repository list after mark-read = %+v, error = %v", items, err)
	}
}

func TestAppointmentEventIsPersistedByBackgroundWorker(t *testing.T) {
	server := newAuthTestServer(t)
	data := newAppointmentTestData(t, server, "notification-event+")
	worker := jobs.NewWorker(server.notifications, log.New(io.Discard, "", 0), 8)
	worker.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = worker.Shutdown(ctx)
	})
	server.handler = routes.New(nil, server.tokens, users.NewRepository(server.pool), server.organizations,
		server.services, server.customers, server.bookings, server.technicians, server.invoices, server.notifications, worker, server.analytics)

	created := requestAppointment(t, server, http.MethodPost, "/api/v1/appointments", data.token,
		appointmentRequest(data, data.customerID, data.serviceID, data.technicianID, "10:00", "11:00"))
	if created.Code != http.StatusCreated {
		t.Fatalf("create appointment status = %d: %s", created.Code, created.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("drain notification worker: %v", err)
	}
	principal, err := server.tokens.Verify(data.token)
	if err != nil {
		t.Fatalf("verify appointment user token: %v", err)
	}
	items, err := server.notifications.List(ctx, data.organizationID, principal.UserID)
	if err != nil {
		t.Fatalf("list event notifications: %v", err)
	}
	if len(items) != 1 || items[0].Type != "appointment.created" {
		t.Fatalf("appointment event notifications = %+v, want one appointment.created notification", items)
	}
}

func TestInvoiceAndPaymentEventsArePersistedByBackgroundWorker(t *testing.T) {
	server := newAuthTestServer(t)
	data := newBookingTestData(t, server, "notification-invoice-event+")
	worker := jobs.NewWorker(server.notifications, log.New(io.Discard, "", 0), 8)
	worker.Start()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = worker.Shutdown(ctx)
	})
	server.handler = routes.New(nil, server.tokens, users.NewRepository(server.pool), server.organizations,
		server.services, server.customers, server.bookings, server.technicians, server.invoices, server.notifications, worker, server.analytics)

	created := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices", data.token, invoiceRequest(data.customerID, ""))
	if created.Code != http.StatusCreated {
		t.Fatalf("create invoice status = %d: %s", created.Code, created.Body.String())
	}
	invoice := decodeInvoiceResponse(t, created)
	payment := requestInvoice(t, server, http.MethodPost, "/api/v1/invoices/"+invoice.ID+"/payments", data.token, validPaymentPayload("1.00"))
	if payment.Code != http.StatusCreated {
		t.Fatalf("record payment status = %d: %s", payment.Code, payment.Body.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("drain notification worker: %v", err)
	}
	principal, err := server.tokens.Verify(data.token)
	if err != nil {
		t.Fatalf("verify invoice user token: %v", err)
	}
	items, err := server.notifications.List(ctx, data.organizationID, principal.UserID)
	if err != nil {
		t.Fatalf("list invoice event notifications: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("invoice and payment event notifications = %+v, want 2", items)
	}
	types := map[string]bool{items[0].Type: true, items[1].Type: true}
	if !types["invoice.created"] || !types["payment.recorded"] {
		t.Fatalf("invoice event notification types = %v", types)
	}
}

func requestNotification(t *testing.T, server *authTestServer, method, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(""))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	server.handler.ServeHTTP(recorder, request)
	return recorder
}
