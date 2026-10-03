package invoices

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/serveflow/serveflow/backend/internal/bookings"
	"github.com/serveflow/serveflow/backend/internal/customers"
)

const (
	StatusDraft               = "DRAFT"
	StatusIssued              = "ISSUED"
	StatusPartiallyPaid       = "PARTIALLY_PAID"
	StatusPaid                = "PAID"
	StatusCancelled           = "CANCELLED"
	maxMoneyCents       int64 = 999999999999
)

var (
	ErrNotFound              = errors.New("invoice not found")
	ErrConflict              = errors.New("invoice cannot be changed in its current state")
	ErrRelatedRecordNotFound = errors.New("customer or appointment not found in this organization")
	ErrCustomerRequired      = errors.New("customer_id is required")
	ErrCustomerInvalid       = errors.New("customer_id must be a valid UUID")
	ErrAppointmentInvalid    = errors.New("appointment_id must be a valid UUID")
	ErrItemsRequired         = errors.New("at least one invoice item is required")
	ErrItemsTooMany          = errors.New("an invoice may contain at most 100 items")
	ErrDescriptionRequired   = errors.New("item description is required")
	ErrDescriptionTooLong    = errors.New("item description must be 200 characters or fewer")
	ErrQuantityInvalid       = errors.New("quantity must be a whole number between 1 and 1000000")
	ErrMoneyInvalid          = errors.New("amount must be non-negative with at most two decimal places")
	ErrDateInvalid           = errors.New("issue_date and due_date must be valid dates in YYYY-MM-DD format")
	ErrDueDateBeforeIssue    = errors.New("due_date cannot be before issue_date")
	ErrNotesTooLong          = errors.New("notes must be 2000 characters or fewer")
	ErrPaymentAmountInvalid  = errors.New("payment amount must be greater than zero")
	ErrPaymentMethodInvalid  = errors.New("payment_method must be CASH, CARD, UPI, BANK_TRANSFER, or OTHER")
	ErrPaymentDateInvalid    = errors.New("payment_date must be a valid date in YYYY-MM-DD format")
	ErrReferenceTooLong      = errors.New("reference must be 120 characters or fewer")
	ErrPaymentNotesTooLong   = errors.New("payment notes must be 1000 characters or fewer")
	ErrOverpayment           = errors.New("payment cannot exceed the remaining invoice balance")
	ErrStatusInvalid         = errors.New("invoice status is invalid")
	invoiceIDRegex           = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	moneyRegex               = regexp.MustCompile(`^\d+(\.\d{1,2})?$`)
)

type Money string

func (money *Money) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return ErrMoneyInvalid
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*money = Money(value)
		return nil
	}
	*money = Money(string(data))
	return nil
}

type ItemInput struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	UnitPrice   Money  `json:"unit_price"`
}

type Input struct {
	CustomerID    string      `json:"customer_id"`
	AppointmentID string      `json:"appointment_id,omitempty"`
	IssueDate     string      `json:"issue_date"`
	DueDate       string      `json:"due_date"`
	Items         []ItemInput `json:"items"`
	Tax           Money       `json:"tax,omitempty"`
	Notes         string      `json:"notes,omitempty"`
}

type UpdateInput Input

type PaymentInput struct {
	Amount        Money  `json:"amount"`
	PaymentDate   string `json:"payment_date"`
	PaymentMethod string `json:"payment_method"`
	Reference     string `json:"reference,omitempty"`
	Notes         string `json:"notes,omitempty"`
}

type ItemTotal struct {
	Amount string
}

type Totals struct {
	Items    []ItemTotal
	Subtotal string
	Tax      string
	Total    string
}

type Invoice struct {
	ID            string      `json:"id"`
	InvoiceNumber string      `json:"invoice_number"`
	CustomerID    string      `json:"customer_id"`
	CustomerName  string      `json:"customer_name,omitempty"`
	AppointmentID string      `json:"appointment_id,omitempty"`
	Status        string      `json:"status"`
	IssueDate     string      `json:"issue_date"`
	DueDate       string      `json:"due_date"`
	Subtotal      json.Number `json:"subtotal"`
	Tax           json.Number `json:"tax"`
	Total         json.Number `json:"total"`
	AmountPaid    json.Number `json:"amount_paid"`
	BalanceDue    json.Number `json:"balance_due"`
	Notes         string      `json:"notes,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type Item struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	Quantity    int         `json:"quantity"`
	UnitPrice   json.Number `json:"unit_price"`
	Amount      json.Number `json:"amount"`
}

type Payment struct {
	ID            string      `json:"id"`
	InvoiceID     string      `json:"invoice_id"`
	Amount        json.Number `json:"amount"`
	PaymentDate   string      `json:"payment_date"`
	PaymentMethod string      `json:"payment_method"`
	Reference     string      `json:"reference,omitempty"`
	Notes         string      `json:"notes,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
}

type Details struct {
	Invoice  Invoice            `json:"invoice"`
	Customer customers.Customer `json:"customer"`
	Items    []Item             `json:"items"`
	Payments []Payment          `json:"payments"`
}

func (input *Input) Validate() (Totals, error) {
	input.CustomerID = strings.TrimSpace(input.CustomerID)
	input.AppointmentID = strings.TrimSpace(input.AppointmentID)
	input.Notes = strings.TrimSpace(input.Notes)
	if input.CustomerID == "" {
		return Totals{}, ErrCustomerRequired
	}
	if !customers.ValidID(input.CustomerID) {
		return Totals{}, ErrCustomerInvalid
	}
	if input.AppointmentID != "" && !bookings.ValidID(input.AppointmentID) {
		return Totals{}, ErrAppointmentInvalid
	}
	if !validDate(input.IssueDate) || !validDate(input.DueDate) {
		return Totals{}, ErrDateInvalid
	}
	if input.DueDate < input.IssueDate {
		return Totals{}, ErrDueDateBeforeIssue
	}
	if len(input.Items) == 0 {
		return Totals{}, ErrItemsRequired
	}
	if len(input.Items) > 100 {
		return Totals{}, ErrItemsTooMany
	}
	if utf8.RuneCountInString(input.Notes) > 2000 {
		return Totals{}, ErrNotesTooLong
	}

	taxCents, err := parseMoney(input.Tax, true)
	if err != nil {
		return Totals{}, err
	}
	input.Tax = Money(formatMoney(taxCents))

	totals := Totals{Items: make([]ItemTotal, len(input.Items))}
	var subtotalCents int64
	for index := range input.Items {
		item := &input.Items[index]
		item.Description = strings.TrimSpace(item.Description)
		if item.Description == "" {
			return Totals{}, ErrDescriptionRequired
		}
		if utf8.RuneCountInString(item.Description) > 200 {
			return Totals{}, ErrDescriptionTooLong
		}
		if item.Quantity < 1 || item.Quantity > 1000000 {
			return Totals{}, ErrQuantityInvalid
		}
		unitPriceCents, err := parseMoney(item.UnitPrice, false)
		if err != nil {
			return Totals{}, err
		}
		if unitPriceCents > 0 && int64(item.Quantity) > maxMoneyCents/unitPriceCents {
			return Totals{}, ErrMoneyInvalid
		}
		amountCents := unitPriceCents * int64(item.Quantity)
		if amountCents > maxMoneyCents-subtotalCents {
			return Totals{}, ErrMoneyInvalid
		}
		subtotalCents += amountCents
		item.UnitPrice = Money(formatMoney(unitPriceCents))
		totals.Items[index] = ItemTotal{Amount: formatMoney(amountCents)}
	}
	if taxCents > maxMoneyCents-subtotalCents {
		return Totals{}, ErrMoneyInvalid
	}
	totals.Subtotal = formatMoney(subtotalCents)
	totals.Tax = formatMoney(taxCents)
	totals.Total = formatMoney(subtotalCents + taxCents)
	return totals, nil
}

func (input *PaymentInput) Validate() (int64, error) {
	amountCents, err := parseMoney(input.Amount, false)
	if err != nil || amountCents == 0 {
		return 0, ErrPaymentAmountInvalid
	}
	if !validDate(input.PaymentDate) {
		return 0, ErrPaymentDateInvalid
	}
	input.PaymentMethod = strings.ToUpper(strings.TrimSpace(input.PaymentMethod))
	switch input.PaymentMethod {
	case "CASH", "CARD", "UPI", "BANK_TRANSFER", "OTHER":
	default:
		return 0, ErrPaymentMethodInvalid
	}
	input.Reference = strings.TrimSpace(input.Reference)
	if utf8.RuneCountInString(input.Reference) > 120 {
		return 0, ErrReferenceTooLong
	}
	input.Notes = strings.TrimSpace(input.Notes)
	if utf8.RuneCountInString(input.Notes) > 1000 {
		return 0, ErrPaymentNotesTooLong
	}
	input.Amount = Money(formatMoney(amountCents))
	return amountCents, nil
}

func ValidID(id string) bool {
	return invoiceIDRegex.MatchString(id)
}

func ValidStatus(status string) bool {
	switch status {
	case StatusDraft, StatusIssued, StatusPartiallyPaid, StatusPaid, StatusCancelled:
		return true
	default:
		return false
	}
}

func StatusAfterPayment(totalCents, paidCents int64) string {
	if paidCents == totalCents {
		return StatusPaid
	}
	return StatusPartiallyPaid
}

func jsonMoney(value string) json.Number {
	return json.Number(value)
}

func parseMoney(money Money, emptyIsZero bool) (int64, error) {
	raw := strings.TrimSpace(string(money))
	if raw == "" && emptyIsZero {
		return 0, nil
	}
	if !moneyRegex.MatchString(raw) {
		return 0, ErrMoneyInvalid
	}
	value, ok := new(big.Rat).SetString(raw)
	if !ok || value.Sign() < 0 {
		return 0, ErrMoneyInvalid
	}
	cents := new(big.Rat).Mul(value, big.NewRat(100, 1))
	if !cents.IsInt() || !cents.Num().IsInt64() || cents.Num().Cmp(big.NewInt(maxMoneyCents)) > 0 {
		return 0, ErrMoneyInvalid
	}
	return cents.Num().Int64(), nil
}

func formatMoney(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

func validDate(value string) bool {
	date, err := time.Parse("2006-01-02", value)
	return err == nil && date.Format("2006-01-02") == value
}
