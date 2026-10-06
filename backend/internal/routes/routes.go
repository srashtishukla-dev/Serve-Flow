package routes

import (
	"net/http"
	"time"

	"github.com/serveflow/serveflow/backend/internal/analytics"
	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/handler"
	"github.com/serveflow/serveflow/backend/internal/handlers"
	"github.com/serveflow/serveflow/backend/internal/invoices"
	"github.com/serveflow/serveflow/backend/internal/jobs"
	"github.com/serveflow/serveflow/backend/internal/middleware"
	"github.com/serveflow/serveflow/backend/internal/notifications"
	"github.com/serveflow/serveflow/backend/internal/organizations"
	"github.com/serveflow/serveflow/backend/internal/services"
	"github.com/serveflow/serveflow/backend/internal/technicians"
	"github.com/serveflow/serveflow/backend/internal/users"
)

func New(authService *auth.Service, tokenManager *auth.TokenManager, userRepository *users.Repository, organizationRepository *organizations.Repository, serviceRepository *services.Repository, customerRepository *customers.Repository, bookingRepository *bookings.Repository, technicianRepository *technicians.Repository, invoiceRepository *invoices.Repository, notificationRepository *notifications.Repository, notificationWorker *jobs.Worker, analyticsRepository *analytics.Repository, healthHandlers ...http.HandlerFunc) *http.ServeMux {
	mux := http.NewServeMux()
	protected := middleware.JWT(tokenManager)
	adminOnly := func(next http.Handler) http.Handler { return protected(middleware.RequireAdmin(userRepository)(next)) }
	adminOrCustomer := func(next http.Handler) http.Handler {
		return protected(middleware.RequireRoles(userRepository, users.RoleAdmin, users.RoleCustomer)(next))
	}
	anyRole := func(next http.Handler) http.Handler {
		return protected(middleware.RequireRoles(userRepository, users.RoleAdmin, users.RoleCustomer, users.RoleTechnician)(next))
	}
	healthHandler := http.HandlerFunc(handler.Health)
	if len(healthHandlers) > 0 && healthHandlers[0] != nil {
		healthHandler = healthHandlers[0]
	}
	mux.Handle("GET /api/v1/health", healthHandler)
	mux.Handle("POST /api/v1/auth/register", middleware.RateLimit(10, time.Minute)(handlers.Register(authService, notificationWorker)))
	mux.Handle("POST /api/v1/auth/login", middleware.RateLimit(10, time.Minute)(handlers.Login(authService)))
	mux.Handle("GET /api/v1/me", protected(handlers.Me(userRepository)))
	mux.Handle("GET /api/v1/organization", protected(handlers.Organization(organizationRepository)))
	mux.Handle("POST /api/v1/services", adminOnly(handlers.CreateService(serviceRepository)))
	mux.Handle("GET /api/v1/services", protected(handlers.ListServices(serviceRepository)))
	mux.Handle("GET /api/v1/services/{id}", protected(handlers.GetService(serviceRepository)))
	mux.Handle("PUT /api/v1/services/{id}", adminOnly(handlers.UpdateService(serviceRepository)))
	mux.Handle("DELETE /api/v1/services/{id}", adminOnly(handlers.DeleteService(serviceRepository)))
	mux.Handle("POST /api/v1/customers", adminOnly(handlers.CreateCustomer(customerRepository)))
	mux.Handle("GET /api/v1/customers", adminOnly(handlers.ListCustomers(customerRepository)))
	mux.Handle("GET /api/v1/customers/{id}", adminOnly(handlers.GetCustomer(customerRepository)))
	mux.Handle("PUT /api/v1/customers/{id}", adminOnly(handlers.UpdateCustomer(customerRepository)))
	mux.Handle("DELETE /api/v1/customers/{id}", adminOnly(handlers.DeleteCustomer(customerRepository)))
	mux.Handle("POST /api/v1/bookings", adminOnly(handlers.CreateBooking(bookingRepository, notificationWorker)))
	mux.Handle("GET /api/v1/bookings", adminOnly(handlers.ListBookings(bookingRepository)))
	mux.Handle("GET /api/v1/bookings/{id}", adminOnly(handlers.GetBooking(bookingRepository)))
	mux.Handle("PUT /api/v1/bookings/{id}", adminOnly(handlers.UpdateBooking(bookingRepository)))
	mux.Handle("DELETE /api/v1/bookings/{id}", adminOnly(handlers.DeleteBooking(bookingRepository)))
	mux.Handle("POST /api/v1/appointments", anyRole(handlers.CreateAppointment(bookingRepository, notificationWorker)))
	mux.Handle("GET /api/v1/appointments", anyRole(handlers.ListAppointments(bookingRepository)))
	mux.Handle("GET /api/v1/appointments/{id}", anyRole(handlers.GetAppointment(bookingRepository)))
	mux.Handle("PUT /api/v1/appointments/{id}", anyRole(handlers.UpdateAppointment(bookingRepository, notificationWorker)))
	mux.Handle("DELETE /api/v1/appointments/{id}", anyRole(handlers.CancelAppointment(bookingRepository, notificationWorker)))
	mux.Handle("POST /api/v1/appointments/{id}/status", anyRole(handlers.ChangeAppointmentStatus(bookingRepository, notificationWorker)))
	mux.Handle("POST /api/v1/customers/{id}/login", adminOnly(handlers.CreateLinkedLogin(userRepository, users.RoleCustomer)))
	mux.Handle("POST /api/v1/technicians/{id}/login", adminOnly(handlers.CreateLinkedLogin(userRepository, users.RoleTechnician)))
	mux.Handle("POST /api/v1/technicians", adminOnly(handlers.CreateTechnician(technicianRepository)))
	mux.Handle("GET /api/v1/technicians", adminOnly(handlers.ListTechnicians(technicianRepository)))
	mux.Handle("GET /api/v1/technicians/{id}", adminOnly(handlers.GetTechnician(technicianRepository)))
	mux.Handle("PUT /api/v1/technicians/{id}", adminOnly(handlers.UpdateTechnician(technicianRepository)))
	mux.Handle("DELETE /api/v1/technicians/{id}", adminOnly(handlers.DeactivateTechnician(technicianRepository)))
	mux.Handle("POST /api/v1/invoices", adminOnly(handlers.CreateInvoice(invoiceRepository, notificationWorker)))
	mux.Handle("GET /api/v1/invoices", adminOrCustomer(handlers.ListInvoices(invoiceRepository)))
	mux.Handle("GET /api/v1/invoices/{id}", adminOrCustomer(handlers.GetInvoice(invoiceRepository)))
	mux.Handle("PUT /api/v1/invoices/{id}", adminOnly(handlers.UpdateInvoice(invoiceRepository)))
	mux.Handle("DELETE /api/v1/invoices/{id}", adminOnly(handlers.CancelInvoice(invoiceRepository)))
	mux.Handle("POST /api/v1/invoices/{id}/payments", adminOnly(handlers.RecordPayment(invoiceRepository, notificationWorker)))
	mux.Handle("GET /api/v1/notifications", protected(handlers.ListNotifications(notificationRepository)))
	mux.Handle("PUT /api/v1/notifications/{id}/read", protected(handlers.MarkNotificationRead(notificationRepository)))
	mux.Handle("GET /api/v1/analytics/summary", adminOnly(handlers.AnalyticsSummary(analyticsRepository)))
	mux.Handle("GET /api/v1/analytics/activity", adminOnly(handlers.AnalyticsActivity(analyticsRepository)))
	mux.Handle("GET /api/v1/admin/dashboard/summary", adminOnly(handlers.AdminDashboardSummary(analyticsRepository)))
	return mux
}
