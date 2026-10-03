package analytics

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Metrics struct {
	Customers             int64       `json:"customers"`
	Services              int64       `json:"services"`
	Appointments          int64       `json:"appointments"`
	UpcomingAppointments  int64       `json:"upcoming_appointments"`
	CompletedAppointments int64       `json:"completed_appointments"`
	ActiveTechnicians     int64       `json:"active_technicians"`
	TotalInvoices         int64       `json:"total_invoices"`
	PaidInvoices          int64       `json:"paid_invoices"`
	PendingInvoices       int64       `json:"pending_invoices"`
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
	Metrics Metrics `json:"metrics"`
	Trends  Trends  `json:"trends"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Summary(ctx context.Context, organizationID string) (Summary, error) {
	var result Summary
	var totalRevenue, outstandingBalance string
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
			       COUNT(*) FILTER (
				       WHERE status = 'BOOKED'
				         AND (booking_date > CURRENT_DATE OR (booking_date = CURRENT_DATE AND start_time >= LOCALTIME))
			       ) AS upcoming
			FROM bookings
			WHERE organization_id = $1::uuid
		), technician_counts AS (
			SELECT COUNT(*) FILTER (WHERE status = 'ACTIVE') AS active
			FROM technicians
			WHERE organization_id = $1::uuid
		), invoice_balances AS (
			SELECT invoice.id, invoice.status,
			       GREATEST(invoice.total - COALESCE(SUM(payment.amount), 0), 0) AS balance_due
			FROM invoices AS invoice
			LEFT JOIN payments AS payment
			  ON payment.invoice_id = invoice.id AND payment.organization_id = invoice.organization_id
			WHERE invoice.organization_id = $1::uuid
			GROUP BY invoice.id
		), invoice_counts AS (
			SELECT COUNT(*) AS total,
			       COUNT(*) FILTER (WHERE status = 'PAID') AS paid,
			       COUNT(*) FILTER (WHERE status IN ('DRAFT', 'ISSUED', 'PARTIALLY_PAID')) AS pending,
			       COALESCE(SUM(balance_due) FILTER (WHERE status IN ('DRAFT', 'ISSUED', 'PARTIALLY_PAID')), 0)::text AS outstanding
			FROM invoice_balances
		), payment_totals AS (
			SELECT COALESCE(SUM(amount), 0)::text AS revenue
			FROM payments
			WHERE organization_id = $1::uuid
		)
		SELECT customer_counts.total, service_counts.total,
		       appointment_counts.total, appointment_counts.upcoming, appointment_counts.completed,
		       technician_counts.active,
		       invoice_counts.total, invoice_counts.paid, invoice_counts.pending, invoice_counts.outstanding,
		       payment_totals.revenue
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
		&result.Metrics.ActiveTechnicians,
		&result.Metrics.TotalInvoices,
		&result.Metrics.PaidInvoices,
		&result.Metrics.PendingInvoices,
		&outstandingBalance,
		&totalRevenue,
	)
	if err != nil {
		return Summary{}, err
	}
	result.Metrics.TotalRevenue = json.Number(totalRevenue)
	result.Metrics.OutstandingBalance = json.Number(outstandingBalance)

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
	return result, nil
}
