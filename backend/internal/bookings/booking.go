package bookings

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/serveflow/serveflow/backend/internal/customers"
	"github.com/serveflow/serveflow/backend/internal/pagination"
	"github.com/serveflow/serveflow/backend/internal/services"
	"github.com/serveflow/serveflow/backend/internal/technicians"
)

const (
	StatusBooked    = "BOOKED"
	StatusCancelled = "CANCELLED"
	StatusCompleted = "COMPLETED"
)

var (
	ErrNotFound                = errors.New("booking not found")
	ErrRelatedNotFound         = errors.New("customer, service, or technician not found in this organization")
	ErrConflict                = errors.New("booking time overlaps another active booking")
	ErrInvalidStatusTransition = errors.New("booking status can only change from BOOKED to COMPLETED or CANCELLED")
	ErrCustomerIDRequired      = errors.New("customer_id is required")
	ErrServiceIDRequired       = errors.New("service_id is required")
	ErrTechnicianIDInvalid     = errors.New("technician_id must be a valid UUID")
	ErrBookingDateInvalid      = errors.New("booking_date must be a valid date in YYYY-MM-DD format")
	ErrBookingTimeInvalid      = errors.New("start_time and end_time must be valid times in HH:MM format")
	ErrBookingEndBeforeStart   = errors.New("end_time must be after start_time")
	ErrBookingNotesTooLong     = errors.New("notes must be 2000 characters or fewer")
	ErrBookingStatusInvalid    = errors.New("status must be BOOKED, CANCELLED, or COMPLETED")
)

type Input struct {
	CustomerID   string `json:"customer_id"`
	ServiceID    string `json:"service_id"`
	TechnicianID string `json:"technician_id,omitempty"`
	BookingDate  string `json:"booking_date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Notes        string `json:"notes"`
}

type AppointmentInput struct {
	CustomerID      string `json:"customer_id"`
	ServiceID       string `json:"service_id"`
	TechnicianID    string `json:"technician_id,omitempty"`
	AppointmentDate string `json:"appointment_date"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	Notes           string `json:"notes"`
}

type AppointmentUpdateInput struct {
	AppointmentInput
	Status string `json:"status,omitempty"`
}

type UpdateInput struct {
	Input
	Status string `json:"status,omitempty"`
}

type StatusInput struct {
	Status string `json:"status"`
}

type Booking struct {
	ID           string `json:"id"`
	CustomerID   string `json:"customer_id"`
	ServiceID    string `json:"service_id"`
	TechnicianID string `json:"technician_id,omitempty"`
	BookingDate  string `json:"booking_date"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Status       string `json:"status"`
	Notes        string `json:"notes,omitempty"`
}

type Appointment struct {
	ID              string `json:"id"`
	CustomerID      string `json:"customer_id"`
	ServiceID       string `json:"service_id"`
	TechnicianID    string `json:"technician_id,omitempty"`
	AppointmentDate string `json:"appointment_date"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	Status          string `json:"status"`
	Notes           string `json:"notes,omitempty"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (input *Input) Validate() error {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.ServiceID = strings.TrimSpace(input.ServiceID)
	input.TechnicianID = strings.TrimSpace(input.TechnicianID)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.CustomerID == "" {
		return ErrCustomerIDRequired
	}
	if !customers.ValidID(input.CustomerID) {
		return ErrCustomerIDRequired
	}
	if input.ServiceID == "" {
		return ErrServiceIDRequired
	}
	if !services.ValidID(input.ServiceID) {
		return ErrServiceIDRequired
	}
	if input.TechnicianID != "" && !technicians.ValidID(input.TechnicianID) {
		return ErrTechnicianIDInvalid
	}
	date, err := time.Parse("2006-01-02", input.BookingDate)
	if err != nil || date.Format("2006-01-02") != input.BookingDate {
		return ErrBookingDateInvalid
	}
	start, err := time.Parse("15:04", input.StartTime)
	if err != nil || start.Format("15:04") != input.StartTime {
		return ErrBookingTimeInvalid
	}
	end, err := time.Parse("15:04", input.EndTime)
	if err != nil || end.Format("15:04") != input.EndTime {
		return ErrBookingTimeInvalid
	}
	if !end.After(start) {
		return ErrBookingEndBeforeStart
	}
	if utf8.RuneCountInString(input.Notes) > 2000 {
		return ErrBookingNotesTooLong
	}
	return nil
}

func (input *AppointmentInput) Validate() error {
	bookingInput := input.toBookingInput()
	if err := bookingInput.Validate(); err != nil {
		return err
	}
	input.CustomerID = bookingInput.CustomerID
	input.ServiceID = bookingInput.ServiceID
	input.TechnicianID = bookingInput.TechnicianID
	input.AppointmentDate = bookingInput.BookingDate
	input.StartTime = bookingInput.StartTime
	input.EndTime = bookingInput.EndTime
	input.Notes = bookingInput.Notes
	return nil
}

func (input *AppointmentUpdateInput) Validate() error {
	if err := input.AppointmentInput.Validate(); err != nil {
		return err
	}
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status != "" && !validStatus(input.Status) {
		return ErrBookingStatusInvalid
	}
	return nil
}

func (input AppointmentInput) toBookingInput() Input {
	return Input{
		CustomerID: input.CustomerID, ServiceID: input.ServiceID, TechnicianID: input.TechnicianID,
		BookingDate: input.AppointmentDate, StartTime: input.StartTime, EndTime: input.EndTime, Notes: input.Notes,
	}
}

func (input *UpdateInput) Validate() error {
	if err := input.Input.Validate(); err != nil {
		return err
	}
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status != "" && !validStatus(input.Status) {
		return ErrBookingStatusInvalid
	}
	return nil
}

func (input *StatusInput) Validate() error {
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status != StatusCompleted && input.Status != StatusCancelled {
		return ErrBookingStatusInvalid
	}
	return nil
}

func (repository *Repository) Create(ctx context.Context, organizationID string, input Input) (Booking, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return Booking{}, err
	}
	if err := relatedRecordsBelongToOrganization(ctx, tx, organizationID, input.CustomerID, input.ServiceID, input.TechnicianID); err != nil {
		return Booking{}, err
	}
	conflict, err := hasConflict(ctx, tx, organizationID, input, nil)
	if err != nil {
		return Booking{}, err
	}
	if conflict {
		return Booking{}, ErrConflict
	}

	booking, err := scanBooking(tx.QueryRow(ctx, `
		INSERT INTO bookings (organization_id, customer_id, service_id, booking_date, start_time, end_time, status, notes, technician_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::date, $5::time, $6::time, $7, $8, NULLIF($9, '')::uuid)
		RETURNING id::text, customer_id::text, service_id::text, booking_date::text,
		          to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
	`, organizationID, input.CustomerID, input.ServiceID, input.BookingDate, input.StartTime, input.EndTime, StatusBooked, input.Notes, input.TechnicianID))
	if err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (repository *Repository) CreateAppointment(ctx context.Context, organizationID string, input AppointmentInput) (Appointment, error) {
	booking, err := repository.Create(ctx, organizationID, input.toBookingInput())
	if err != nil {
		return Appointment{}, err
	}
	return appointmentFromBooking(booking), nil
}

func (repository *Repository) List(ctx context.Context, organizationID string) ([]Booking, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, customer_id::text, service_id::text, booking_date::text,
		       to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
		FROM bookings
		WHERE organization_id = $1::uuid
		ORDER BY booking_date, start_time, id
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Booking, 0)
	for rows.Next() {
		item, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) ListPage(ctx context.Context, organizationID string, query pagination.Query) ([]Booking, pagination.Metadata, error) {
	if query.Status != "" && !validStatus(query.Status) {
		return nil, pagination.Metadata{}, ErrBookingStatusInvalid
	}
	var total int
	if err := repository.pool.QueryRow(ctx, `
		SELECT count(*)
		FROM bookings AS booking
		WHERE booking.organization_id = $1::uuid
		  AND ($2 = '' OR booking.status = $2)
		  AND ($3 = '' OR booking.notes ILIKE '%' || $3 || '%'
		       OR EXISTS (SELECT 1 FROM customers customer WHERE customer.id = booking.customer_id AND customer.organization_id = booking.organization_id AND customer.name ILIKE '%' || $3 || '%'))
	`, organizationID, query.Status, query.Search).Scan(&total); err != nil {
		return nil, pagination.Metadata{}, err
	}
	orderBy := pagination.OrderBy(query, map[string]string{
		"booking_date": "booking_date", "created_at": "created_at", "status": "status",
	})
	rows, err := repository.pool.Query(ctx, `
		SELECT booking.id::text, booking.customer_id::text, booking.service_id::text, booking.booking_date::text,
		       to_char(booking.start_time, 'HH24:MI'), to_char(booking.end_time, 'HH24:MI'), booking.status, booking.notes, COALESCE(booking.technician_id::text, '')
		FROM bookings AS booking
		WHERE booking.organization_id = $1::uuid
		  AND ($2 = '' OR booking.status = $2)
		  AND ($3 = '' OR booking.notes ILIKE '%' || $3 || '%'
		       OR EXISTS (SELECT 1 FROM customers customer WHERE customer.id = booking.customer_id AND customer.organization_id = booking.organization_id AND customer.name ILIKE '%' || $3 || '%'))
		ORDER BY `+orderBy+`, booking.start_time ASC, booking.id ASC
		LIMIT $4 OFFSET $5
	`, organizationID, query.Status, query.Search, query.Limit, query.Offset)
	if err != nil {
		return nil, pagination.Metadata{}, err
	}
	defer rows.Close()
	items := make([]Booking, 0)
	for rows.Next() {
		item, err := scanBooking(rows)
		if err != nil {
			return nil, pagination.Metadata{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, pagination.Metadata{}, err
	}
	return items, pagination.NewMetadata(query, total), nil
}

func (repository *Repository) ListAppointmentsPage(ctx context.Context, organizationID string, query pagination.Query) ([]Appointment, pagination.Metadata, error) {
	bookings, metadata, err := repository.ListPage(ctx, organizationID, query)
	if err != nil {
		return nil, pagination.Metadata{}, err
	}
	appointments := make([]Appointment, len(bookings))
	for index, booking := range bookings {
		appointments[index] = appointmentFromBooking(booking)
	}
	return appointments, metadata, nil
}

func (repository *Repository) ListAppointments(ctx context.Context, organizationID string) ([]Appointment, error) {
	bookings, err := repository.List(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	appointments := make([]Appointment, len(bookings))
	for index, booking := range bookings {
		appointments[index] = appointmentFromBooking(booking)
	}
	return appointments, nil
}

func (repository *Repository) Today(ctx context.Context, organizationID string) ([]Booking, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, customer_id::text, service_id::text, booking_date::text,
		       to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
		FROM bookings
		WHERE organization_id = $1::uuid AND booking_date = CURRENT_DATE
		ORDER BY start_time, id
	`, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (repository *Repository) Upcoming(ctx context.Context, organizationID string) ([]Booking, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT id::text, customer_id::text, service_id::text, booking_date::text,
		       to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
		FROM bookings
		WHERE organization_id = $1::uuid
		  AND status = $2
		  AND (booking_date > CURRENT_DATE OR (booking_date = CURRENT_DATE AND start_time >= LOCALTIME))
		ORDER BY booking_date, start_time, id
		LIMIT 20
	`, organizationID, StatusBooked)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanBookings(rows)
}

func (repository *Repository) FindByID(ctx context.Context, organizationID, bookingID string) (Booking, error) {
	item, err := scanBooking(repository.pool.QueryRow(ctx, `
		SELECT id::text, customer_id::text, service_id::text, booking_date::text,
		       to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
		FROM bookings
		WHERE id = $1::uuid AND organization_id = $2::uuid
	`, bookingID, organizationID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	return item, err
}

func (repository *Repository) FindAppointmentByID(ctx context.Context, organizationID, appointmentID string) (Appointment, error) {
	booking, err := repository.FindByID(ctx, organizationID, appointmentID)
	if err != nil {
		return Appointment{}, err
	}
	return appointmentFromBooking(booking), nil
}

func (repository *Repository) Update(ctx context.Context, organizationID, bookingID string, input UpdateInput) (Booking, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return Booking{}, err
	}
	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status FROM bookings WHERE id = $1::uuid AND organization_id = $2::uuid FOR UPDATE
	`, bookingID, organizationID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if err := relatedRecordsBelongToOrganization(ctx, tx, organizationID, input.CustomerID, input.ServiceID, input.TechnicianID); err != nil {
		return Booking{}, err
	}

	status := input.Status
	if status == "" {
		status = currentStatus
	} else if !validStatus(status) {
		return Booking{}, ErrBookingStatusInvalid
	} else if status != currentStatus && (currentStatus != StatusBooked || (status != StatusCompleted && status != StatusCancelled)) {
		return Booking{}, ErrInvalidStatusTransition
	}
	if status == StatusBooked {
		conflict, err := hasConflict(ctx, tx, organizationID, input.Input, &bookingID)
		if err != nil {
			return Booking{}, err
		}
		if conflict {
			return Booking{}, ErrConflict
		}
	}

	booking, err := scanBooking(tx.QueryRow(ctx, `
		UPDATE bookings
		SET customer_id = $3::uuid, service_id = $4::uuid, booking_date = $5::date,
		    start_time = $6::time, end_time = $7::time, status = $8, notes = $9,
		    technician_id = NULLIF($10, '')::uuid,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
		RETURNING id::text, customer_id::text, service_id::text, booking_date::text,
		          to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
	`, bookingID, organizationID, input.CustomerID, input.ServiceID, input.BookingDate, input.StartTime, input.EndTime, status, input.Notes, input.TechnicianID))
	if err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (repository *Repository) UpdateAppointment(ctx context.Context, organizationID, appointmentID string, input AppointmentUpdateInput) (Appointment, error) {
	booking, err := repository.Update(ctx, organizationID, appointmentID, UpdateInput{
		Input: input.AppointmentInput.toBookingInput(), Status: input.Status,
	})
	if err != nil {
		return Appointment{}, err
	}
	return appointmentFromBooking(booking), nil
}

func (repository *Repository) ChangeStatus(ctx context.Context, organizationID, bookingID, status string) (Booking, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != StatusCompleted && status != StatusCancelled {
		return Booking{}, ErrBookingStatusInvalid
	}
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return Booking{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockOrganization(ctx, tx, organizationID); err != nil {
		return Booking{}, err
	}
	var currentStatus string
	err = tx.QueryRow(ctx, `
		SELECT status FROM bookings WHERE id = $1::uuid AND organization_id = $2::uuid FOR UPDATE
	`, bookingID, organizationID).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return Booking{}, ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if currentStatus != StatusBooked {
		return Booking{}, ErrInvalidStatusTransition
	}
	booking, err := scanBooking(tx.QueryRow(ctx, `
		UPDATE bookings
		SET status = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid AND organization_id = $2::uuid
		RETURNING id::text, customer_id::text, service_id::text, booking_date::text,
		          to_char(start_time, 'HH24:MI'), to_char(end_time, 'HH24:MI'), status, notes, COALESCE(technician_id::text, '')
	`, bookingID, organizationID, status))
	if err != nil {
		return Booking{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func (repository *Repository) Cancel(ctx context.Context, organizationID, bookingID string) error {
	_, err := repository.ChangeStatus(ctx, organizationID, bookingID, StatusCancelled)
	return err
}

func (repository *Repository) CancelAppointment(ctx context.Context, organizationID, appointmentID string) error {
	return repository.Cancel(ctx, organizationID, appointmentID)
}

func ValidID(id string) bool {
	return customers.ValidID(id)
}

func validStatus(status string) bool {
	return status == StatusBooked || status == StatusCancelled || status == StatusCompleted
}

func lockOrganization(ctx context.Context, tx pgx.Tx, organizationID string) error {
	var lockedID string
	err := tx.QueryRow(ctx, `SELECT id::text FROM organizations WHERE id = $1::uuid FOR UPDATE`, organizationID).Scan(&lockedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func relatedRecordsBelongToOrganization(ctx context.Context, tx pgx.Tx, organizationID, customerID, serviceID, technicianID string) error {
	var customerExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM customers WHERE id = $1::uuid AND organization_id = $2::uuid)
	`, customerID, organizationID).Scan(&customerExists); err != nil {
		return err
	}
	var serviceExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM services WHERE id = $1::uuid AND organization_id = $2::uuid)
	`, serviceID, organizationID).Scan(&serviceExists); err != nil {
		return err
	}
	if !customerExists || !serviceExists {
		return ErrRelatedNotFound
	}
	if technicianID != "" {
		var technicianExists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM technicians WHERE id = $1::uuid AND organization_id = $2::uuid)
		`, technicianID, organizationID).Scan(&technicianExists); err != nil {
			return err
		}
		if !technicianExists {
			return ErrRelatedNotFound
		}
	}
	return nil
}

func hasConflict(ctx context.Context, tx pgx.Tx, organizationID string, input Input, excludeID *string) (bool, error) {
	var ignoredID any
	if excludeID != nil {
		ignoredID = *excludeID
	}
	var technicianID any
	if input.TechnicianID != "" {
		technicianID = input.TechnicianID
	}
	var conflict bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM bookings
			WHERE organization_id = $1::uuid
			  AND booking_date = $2::date
			  AND status = $3
			  AND start_time < $5::time
			  AND end_time > $4::time
			  AND (($7::uuid IS NULL AND technician_id IS NULL) OR technician_id = $7::uuid)
			  AND ($6::uuid IS NULL OR id <> $6::uuid)
		)
	`, organizationID, input.BookingDate, StatusBooked, input.StartTime, input.EndTime, ignoredID, technicianID).Scan(&conflict)
	return conflict, err
}

func scanBooking(row pgx.Row) (Booking, error) {
	var booking Booking
	err := row.Scan(&booking.ID, &booking.CustomerID, &booking.ServiceID, &booking.BookingDate, &booking.StartTime, &booking.EndTime, &booking.Status, &booking.Notes, &booking.TechnicianID)
	if err != nil {
		return Booking{}, err
	}
	return booking, nil
}

func appointmentFromBooking(booking Booking) Appointment {
	return Appointment{
		ID: booking.ID, CustomerID: booking.CustomerID, ServiceID: booking.ServiceID, TechnicianID: booking.TechnicianID,
		AppointmentDate: booking.BookingDate, StartTime: booking.StartTime, EndTime: booking.EndTime,
		Status: booking.Status, Notes: booking.Notes,
	}
}

func scanBookings(rows pgx.Rows) ([]Booking, error) {
	items := make([]Booking, 0)
	for rows.Next() {
		item, err := scanBooking(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
