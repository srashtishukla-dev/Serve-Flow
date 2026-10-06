package analytics

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Metrics struct {
	Customers             int64       `json:"customers"`
	Services              int64       `json:"services"`
	Appointments          int64       `json:"appointments"`
	UpcomingAppointments  int64       `json:"upcoming_appointments"`
	CompletedAppointments int64       `json:"completed_appointments"`
	CancelledAppointments int64       `json:"cancelled_appointments"`
	NoShowAppointments    int64       `json:"no_show_appointments"`
	TotalTechnicians      int64       `json:"total_technicians"`
	ActiveTechnicians     int64       `json:"active_technicians"`
	TotalInvoices         int64       `json:"total_invoices"`
	PaidInvoices          int64       `json:"paid_invoices"`
	PendingInvoices       int64       `json:"pending_invoices"`
	TotalPayments         int64       `json:"total_payments"`
	TotalInvoicedAmount   json.Number `json:"total_invoiced_amount"`
	TotalRevenue          json.Number `json:"total_revenue"`
	OutstandingBalance    json.Number `json:"outstanding_balance"`
}

type MonthlyTrend struct {
	Month        string      `json:"month"`
	Revenue      json.Number `json:"revenue"`
	Appointments int64       `json:"appointments"`
}

type Trends struct {
	StartMonth string         `json:"start_month"`
	EndMonth   string         `json:"end_month"`
	Months     []MonthlyTrend `json:"months"`
}

type Summary struct {
	Metrics            Metrics              `json:"metrics"`
	Trends             Trends               `json:"trends"`
	PopularServices    []ServiceInsight     `json:"popular_services"`
	TopCustomers       []CustomerInsight    `json:"top_customers"`
	TechnicianWorkload []TechnicianWorkload `json:"technician_workload"`
}

type Repository struct {
	pool *pgxpool.Pool
}

type ServiceInsight struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Bookings int64  `json:"bookings"`
}

type CustomerInsight struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Bookings        int64  `json:"bookings"`
	LastBookingDate string `json:"last_booking_date"`
}

type TechnicianWorkload struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Bookings          int64  `json:"bookings"`
	ActiveBookings    int64  `json:"active_bookings"`
	CompletedBookings int64  `json:"completed_bookings"`
}

type DailyActivity struct {
	Date                  string      `json:"date"`
	Revenue               json.Number `json:"revenue"`
	Appointments          int64       `json:"appointments"`
	CompletedAppointments int64       `json:"completed_appointments"`
	CancelledAppointments int64       `json:"cancelled_appointments"`
	NoShowAppointments    int64       `json:"no_show_appointments"`
}

type Activity struct {
	Range     string          `json:"range"`
	StartDate string          `json:"start_date"`
	EndDate   string          `json:"end_date"`
	Days      []DailyActivity `json:"days"`
}

const (
	RangeToday        = "today"
	RangeLast7Days    = "last_7_days"
	RangeLast30Days   = "last_30_days"
	RangeCurrentMonth = "current_month"
)

var ErrInvalidRange = errors.New("range must be today, last_7_days, last_30_days, or current_month")

// DashboardSummary is the organization-scoped overview for the admin dashboard.
// A pending payment is an invoice that is issued or partially paid with a balance due;
// completed payments are recorded payment entries, since payments have no status column.
type DashboardSummary struct {
	TotalCustomers        int64       `json:"total_customers"`
	TotalTechnicians      int64       `json:"total_technicians"`
	TotalServices         int64       `json:"total_services"`
	TotalBookings         int64       `json:"total_bookings"`
	UpcomingAppointments  int64       `json:"upcoming_appointments"`
	CompletedAppointments int64       `json:"completed_appointments"`
	CancelledAppointments int64       `json:"cancelled_appointments"`
	PendingPayments       int64       `json:"pending_payments"`
	CompletedPayments     int64       `json:"completed_payments"`
	TotalRevenue          json.Number `json:"total_revenue"`
}

func (repository *Repository) DashboardSummary(ctx context.Context, organizationID string) (DashboardSummary, error) {
	var result DashboardSummary
	var revenue string
	err := repository.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM customers WHERE organization_id = $1::uuid),
			(SELECT COUNT(*) FROM technicians WHERE organization_id = $1::uuid),
			(SELECT COUNT(*) FROM services WHERE organization_id = $1::uuid),
			(SELECT COUNT(*) FROM bookings WHERE organization_id = $1::uuid),
			(SELECT COUNT(*) FROM bookings
			  WHERE organization_id = $1::uuid AND status IN ('BOOKED','ASSIGNED')
			    AND (booking_date > CURRENT_DATE OR (booking_date = CURRENT_DATE AND start_time >= LOCALTIME))),
			(SELECT COUNT(*) FROM bookings WHERE organization_id = $1::uuid AND status = 'COMPLETED'),
			(SELECT COUNT(*) FROM bookings WHERE organization_id = $1::uuid AND status = 'CANCELLED'),
			(SELECT COUNT(*) FROM invoices
			  WHERE organization_id = $1::uuid AND status IN ('ISSUED', 'PARTIALLY_PAID')),
			(SELECT COUNT(*) FROM payments WHERE organization_id = $1::uuid),
			(SELECT COALESCE(SUM(amount), 0)::text FROM payments WHERE organization_id = $1::uuid)
	`, organizationID).Scan(
		&result.TotalCustomers, &result.TotalTechnicians, &result.TotalServices, &result.TotalBookings,
		&result.UpcomingAppointments, &result.CompletedAppointments, &result.CancelledAppointments,
		&result.PendingPayments, &result.CompletedPayments, &revenue,
	)
	if err != nil {
		return DashboardSummary{}, err
	}
	result.TotalRevenue = json.Number(revenue)
	return result, nil
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Summary(ctx context.Context, organizationID string) (Summary, error) {
	var result Summary
	var totalRevenue, outstandingBalance, totalInvoicedAmount string
	err := repository.pool.QueryRow(ctx, `
		WITH customer_counts AS (
			SELECT COUNT(*) AS total
			FROM customers
			WHERE organization_id = $1::uuid
		), service_counts AS (
			SELECT COUNT(*) AS total
			FROM services
			WHERE organization_id = $1::uuid
		), appointment_counts AS (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE status = 'COMPLETED') AS completed,
			       COUNT(*) FILTER (WHERE status = 'CANCELLED') AS cancelled,
			       COUNT(*) FILTER (WHERE status = 'NO_SHOW') AS no_show,
			       COUNT(*) FILTER (
				       WHERE status IN ('BOOKED','ASSIGNED')
				         AND (booking_date > CURRENT_DATE OR (booking_date = CURRENT_DATE AND start_time >= LOCALTIME))
			       ) AS upcoming
			FROM bookings
			WHERE organization_id = $1::uuid
		), technician_counts AS (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE status = 'ACTIVE') AS active
			FROM technicians
			WHERE organization_id = $1::uuid
		), invoice_balances AS (
			SELECT invoice.id, invoice.status, invoice.total,
			       GREATEST(invoice.total - COALESCE(SUM(payment.amount), 0), 0) AS balance_due
			FROM invoices AS invoice
			LEFT JOIN payments AS payment
			  ON payment.invoice_id = invoice.id AND payment.organization_id = invoice.organization_id
			WHERE invoice.organization_id = $1::uuid
			GROUP BY invoice.id
		), invoice_counts AS (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE status = 'PAID') AS paid,
			       COUNT(*) FILTER (WHERE status IN ('ISSUED', 'PARTIALLY_PAID')) AS pending,
			       COALESCE(SUM(balance_due) FILTER (WHERE status IN ('ISSUED', 'PARTIALLY_PAID')), 0)::text AS outstanding,
			       COALESCE(SUM(total) FILTER (WHERE status IN ('ISSUED', 'PARTIALLY_PAID', 'PAID')), 0)::text AS invoiced
			FROM invoice_balances
		), payment_totals AS (
			SELECT COUNT(*) AS total,
			       COALESCE(SUM(amount), 0)::text AS revenue
			FROM payments
			WHERE organization_id = $1::uuid
		)
		SELECT customer_counts.total, service_counts.total,
		       appointment_counts.total, appointment_counts.upcoming, appointment_counts.completed,
		       appointment_counts.cancelled, appointment_counts.no_show,
		       technician_counts.total, technician_counts.active,
		       invoice_counts.total, invoice_counts.paid, invoice_counts.pending, invoice_counts.outstanding, invoice_counts.invoiced,
		       payment_totals.total, payment_totals.revenue
		FROM customer_counts
		CROSS JOIN service_counts
		CROSS JOIN appointment_counts
		CROSS JOIN technician_counts
		CROSS JOIN invoice_counts
		CROSS JOIN payment_totals
	`, organizationID).Scan(
		&result.Metrics.Customers,
		&result.Metrics.Services,
		&result.Metrics.Appointments,
		&result.Metrics.UpcomingAppointments,
		&result.Metrics.CompletedAppointments,
		&result.Metrics.CancelledAppointments,
		&result.Metrics.NoShowAppointments,
		&result.Metrics.TotalTechnicians,
		&result.Metrics.ActiveTechnicians,
		&result.Metrics.TotalInvoices,
		&result.Metrics.PaidInvoices,
		&result.Metrics.PendingInvoices,
		&outstandingBalance,
		&totalInvoicedAmount,
		&result.Metrics.TotalPayments,
		&totalRevenue,
	)
	if err != nil {
		return Summary{}, err
	}
	result.Metrics.TotalRevenue = json.Number(totalRevenue)
	result.Metrics.OutstandingBalance = json.Number(outstandingBalance)
	result.Metrics.TotalInvoicedAmount = json.Number(totalInvoicedAmount)

	rows, err := repository.pool.Query(ctx, `
		WITH months AS (
			SELECT generate_series(
				date_trunc('month', CURRENT_DATE) - INTERVAL '5 months',
				date_trunc('month', CURRENT_DATE),
				INTERVAL '1 month'
			)::date AS month_start
		), month_bounds AS (
			SELECT MIN(month_start) AS start_date, MAX(month_start) + INTERVAL '1 month' AS end_date
			FROM months
		), monthly_revenue AS (
			SELECT date_trunc('month', payment_date)::date AS month_start, SUM(amount)::text AS revenue
			FROM payments
			WHERE organization_id = $1::uuid
			  AND payment_date >= (SELECT start_date FROM month_bounds)
			  AND payment_date < (SELECT end_date FROM month_bounds)
			GROUP BY 1
		), monthly_appointments AS (
			SELECT date_trunc('month', booking_date)::date AS month_start,
			       COUNT(*) FILTER (WHERE status <> 'CANCELLED') AS appointments
			FROM bookings
			WHERE organization_id = $1::uuid
			  AND booking_date >= (SELECT start_date FROM month_bounds)
			  AND booking_date < (SELECT end_date FROM month_bounds)
			GROUP BY 1
		)
		SELECT to_char(months.month_start, 'YYYY-MM'),
		       COALESCE(monthly_revenue.revenue, '0.00'),
		       COALESCE(monthly_appointments.appointments, 0)
		FROM months
		LEFT JOIN monthly_revenue USING (month_start)
		LEFT JOIN monthly_appointments USING (month_start)
		ORDER BY months.month_start
	`, organizationID)
	if err != nil {
		return Summary{}, err
	}
	defer rows.Close()
	result.Trends.Months = make([]MonthlyTrend, 0, 6)
	for rows.Next() {
		var month MonthlyTrend
		var revenue string
		if err := rows.Scan(&month.Month, &revenue, &month.Appointments); err != nil {
			return Summary{}, err
		}
		month.Revenue = json.Number(revenue)
		result.Trends.Months = append(result.Trends.Months, month)
	}
	if err := rows.Err(); err != nil {
		return Summary{}, err
	}
	if len(result.Trends.Months) > 0 {
		result.Trends.StartMonth = result.Trends.Months[0].Month
		result.Trends.EndMonth = result.Trends.Months[len(result.Trends.Months)-1].Month
	}
	result.PopularServices, err = repository.popularServices(ctx, organizationID)
	if err != nil {
		return Summary{}, err
	}
	result.TopCustomers, err = repository.topCustomers(ctx, organizationID)
	if err != nil {
		return Summary{}, err
	}
	result.TechnicianWorkload, err = repository.technicianWorkload(ctx, organizationID)
	if err != nil {
		return Summary{}, err
	}
	return result, nil
}

func (repository *Repository) Activity(ctx context.Context, organizationID, dateRange string) (Activity, error) {
	switch dateRange {
	case RangeToday, RangeLast7Days, RangeLast30Days, RangeCurrentMonth:
	default:
		return Activity{}, ErrInvalidRange
	}
	result := Activity{Range: dateRange, Days: make([]DailyActivity, 0)}
	rows, err := repository.pool.Query(ctx, `
		WITH bounds AS (
			SELECT CASE $2
			           WHEN 'today' THEN CURRENT_DATE
			           WHEN 'last_7_days' THEN CURRENT_DATE - 6
			           WHEN 'last_30_days' THEN CURRENT_DATE - 29
			           ELSE date_trunc('month', CURRENT_DATE)::date
			       END AS start_date,
			       CURRENT_DATE AS end_date
		), days AS (
			SELECT generate_series(start_date, end_date, INTERVAL '1 day')::date AS activity_date
			FROM bounds
		), daily_payments AS (
			SELECT payment_date, SUM(amount)::text AS revenue
			FROM payments
			WHERE organization_id = $1::uuid
			  AND payment_date BETWEEN (SELECT start_date FROM bounds) AND (SELECT end_date FROM bounds)
			GROUP BY payment_date
		), daily_appointments AS (
			SELECT booking_date,
			       COUNT(*) AS appointments,
			       COUNT(*) FILTER (WHERE status = 'COMPLETED') AS completed,
			       COUNT(*) FILTER (WHERE status = 'CANCELLED') AS cancelled,
			       COUNT(*) FILTER (WHERE status = 'NO_SHOW') AS no_show
			FROM bookings
			WHERE organization_id = $1::uuid
			  AND booking_date BETWEEN (SELECT start_date FROM bounds) AND (SELECT end_date FROM bounds)
			GROUP BY booking_date
		)
		SELECT days.activity_date::text,
		       COALESCE(daily_payments.revenue, '0.00'),
		       COALESCE(daily_appointments.appointments, 0),
		       COALESCE(daily_appointments.completed, 0),
		       COALESCE(daily_appointments.cancelled, 0),
		       COALESCE(daily_appointments.no_show, 0)
		FROM days
		LEFT JOIN daily_payments ON daily_payments.payment_date = days.activity_date
		LEFT JOIN daily_appointments ON daily_appointments.booking_date = days.activity_date
		ORDER BY days.activity_date
	`, organizationID, dateRange)
	if err != nil {
		return Activity{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var day DailyActivity
		var revenue string
		if err := rows.Scan(&day.Date, &revenue, &day.Appointments, &day.CompletedAppointments, &day.CancelledAppointments, &day.NoShowAppointments); err != nil {
			return Activity{}, err
		}
		day.Revenue = json.Number(revenue)
		result.Days = append(result.Days, day)
	}
	if err := rows.Err(); err != nil {
		return Activity{}, err
	}
	if len(result.Days) > 0 {
		result.StartDate = result.Days[0].Date
		result.EndDate = result.Days[len(result.Days)-1].Date
	}
	return result, nil
}

func (repository *Repository) popularServices(ctx context.Context, organizationID string) ([]ServiceInsight, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT service.id::text, service.name, COUNT(booking.id)
		FROM services AS service
		JOIN bookings AS booking
		  ON booking.service_id = service.id AND booking.organization_id = service.organization_id
		 AND booking.status <> 'CANCELLED'
		WHERE service.organization_id = $1::uuid
		GROUP BY service.id, service.name
		ORDER BY COUNT(booking.id) DESC, service.name
		LIMIT 5
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ServiceInsight, 0)
	for rows.Next() {
		var item ServiceInsight
		if err := rows.Scan(&item.ID, &item.Name, &item.Bookings); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) topCustomers(ctx context.Context, organizationID string) ([]CustomerInsight, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT customer.id::text, customer.name, COUNT(booking.id), MAX(booking.booking_date)::text
		FROM customers AS customer
		JOIN bookings AS booking
		  ON booking.customer_id = customer.id AND booking.organization_id = customer.organization_id
		 AND booking.status <> 'CANCELLED'
		WHERE customer.organization_id = $1::uuid
		GROUP BY customer.id, customer.name
		ORDER BY COUNT(booking.id) DESC, MAX(booking.booking_date) DESC, customer.name
		LIMIT 5
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CustomerInsight, 0)
	for rows.Next() {
		var item CustomerInsight
		if err := rows.Scan(&item.ID, &item.Name, &item.Bookings, &item.LastBookingDate); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) technicianWorkload(ctx context.Context, organizationID string) ([]TechnicianWorkload, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT technician.id::text, technician.name,
		       COUNT(booking.id),
		       COUNT(booking.id) FILTER (WHERE booking.status IN ('BOOKED', 'ASSIGNED', 'IN_PROGRESS')),
		       COUNT(booking.id) FILTER (WHERE booking.status = 'COMPLETED')
		FROM technicians AS technician
		LEFT JOIN bookings AS booking
		  ON booking.technician_id = technician.id AND booking.organization_id = technician.organization_id
		WHERE technician.organization_id = $1::uuid
		GROUP BY technician.id, technician.name
		ORDER BY COUNT(booking.id) FILTER (WHERE booking.status IN ('BOOKED', 'ASSIGNED', 'IN_PROGRESS')) DESC,
		         COUNT(booking.id) DESC, technician.name
		LIMIT 10
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TechnicianWorkload, 0)
	for rows.Next() {
		var item TechnicianWorkload
		if err := rows.Scan(&item.ID, &item.Name, &item.Bookings, &item.ActiveBookings, &item.CompletedBookings); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
