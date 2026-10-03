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
	healthHandler := http.HandlerFunc(handler.Health)
	if len(healthHandlers) > 0 && healthHandlers[0] != nil {
		healthHandler = healthHandlers[0]
	}
	mux.Handle("GET /api/v1/health", healthHandler)
	mux.Handle("POST /api/v1/auth/register", middleware.RateLimit(10, time.Minute)(handlers.Register(authService, notificationWorker)))
	mux.Handle("POST /api/v1/auth/login", middleware.RateLimit(10, time.Minute)(handlers.Login(authService)))
	mux.Handle("GET /api/v1/me", protected(handlers.Me(userRepository)))
	mux.Handle("GET /api/v1/organization", protected(handlers.Organization(organizationRepository)))
	mux.Handle("POST /api/v1/services", protected(handlers.CreateService(serviceRepository)))
	mux.Handle("GET /api/v1/services", protected(handlers.ListServices(serviceRepository)))
	mux.Handle("GET /api/v1/services/{id}", protected(handlers.GetService(serviceRepository)))
	mux.Handle("PUT /api/v1/services/{id}", protected(handlers.UpdateService(serviceRepository)))
	mux.Handle("DELETE /api/v1/services/{id}", protected(handlers.DeleteService(serviceRepository)))
	mux.Handle("POST /api/v1/customers", protected(handlers.CreateCustomer(customerRepository)))
	mux.Handle("GET /api/v1/customers", protected(handlers.ListCustomers(customerRepository)))
	mux.Handle("GET /api/v1/customers/{id}", protected(handlers.GetCustomer(customerRepository)))
	mux.Handle("PUT /api/v1/customers/{id}", protected(handlers.UpdateCustomer(customerRepository)))
	mux.Handle("DELETE /api/v1/customers/{id}", protected(handlers.DeleteCustomer(customerRepository)))
	mux.Handle("POST /api/v1/bookings", protected(handlers.CreateBooking(bookingRepository, notificationWorker)))
	mux.Handle("GET /api/v1/bookings", protected(handlers.ListBookings(bookingRepository)))
	mux.Handle("GET /api/v1/bookings/{id}", protected(handlers.GetBooking(bookingRepository)))
	mux.Handle("PUT /api/v1/bookings/{id}", protected(handlers.UpdateBooking(bookingRepository)))
	mux.Handle("DELETE /api/v1/bookings/{id}", protected(handlers.DeleteBooking(bookingRepository)))
	mux.Handle("POST /api/v1/appointments", protected(handlers.CreateAppointment(bookingRepository, notificationWorker)))
	mux.Handle("GET /api/v1/appointments", protected(handlers.ListAppointments(bookingRepository)))
	mux.Handle("GET /api/v1/appointments/{id}", protected(handlers.GetAppointment(bookingRepository)))
	mux.Handle("PUT /api/v1/appointments/{id}", protected(handlers.UpdateAppointment(bookingRepository)))
	mux.Handle("DELETE /api/v1/appointments/{id}", protected(handlers.CancelAppointment(bookingRepository)))
	mux.Handle("POST /api/v1/technicians", protected(handlers.CreateTechnician(technicianRepository)))
	mux.Handle("GET /api/v1/technicians", protected(handlers.ListTechnicians(technicianRepository)))
	mux.Handle("GET /api/v1/technicians/{id}", protected(handlers.GetTechnician(technicianRepository)))
	mux.Handle("PUT /api/v1/technicians/{id}", protected(handlers.UpdateTechnician(technicianRepository)))
	mux.Handle("DELETE /api/v1/technicians/{id}", protected(handlers.DeactivateTechnician(technicianRepository)))
	mux.Handle("POST /api/v1/invoices", protected(handlers.CreateInvoice(invoiceRepository, notificationWorker)))
	mux.Handle("GET /api/v1/invoices", protected(handlers.ListInvoices(invoiceRepository)))
	mux.Handle("GET /api/v1/invoices/{id}", protected(handlers.GetInvoice(invoiceRepository)))
	mux.Handle("PUT /api/v1/invoices/{id}", protected(handlers.UpdateInvoice(invoiceRepository)))
	mux.Handle("DELETE /api/v1/invoices/{id}", protected(handlers.CancelInvoice(invoiceRepository)))
	mux.Handle("POST /api/v1/invoices/{id}/payments", protected(handlers.RecordPayment(invoiceRepository, notificationWorker)))
	mux.Handle("GET /api/v1/notifications", protected(handlers.ListNotifications(notificationRepository)))
	mux.Handle("PUT /api/v1/notifications/{id}/read", protected(handlers.MarkNotificationRead(notificationRepository)))
	mux.Handle("GET /api/v1/analytics/summary", protected(handlers.AnalyticsSummary(analyticsRepository)))
	return mux
}
