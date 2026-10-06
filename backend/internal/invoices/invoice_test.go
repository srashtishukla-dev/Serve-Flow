package invoices

import (
	"errors"
	"testing"
)

func TestInputValidateCalculatesExactTotals(t *testing.T) {
	input := Input{
		CustomerID: "00000000-0000-0000-0000-000000000001",
		IssueDate:  "2030-01-01",
		DueDate:    "2030-01-31",
		Items: []ItemInput{
			{Description: " Service A ", Quantity: 2, UnitPrice: Money("500.00")},
			{Description: "Service B", Quantity: 1, UnitPrice: Money("300")},
		},
		Tax: Money("130"),
	}
	totals, err := input.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if totals.Items[0].Amount != "1000.00" || totals.Items[1].Amount != "300.00" || totals.Subtotal != "1300.00" || totals.Tax != "130.00" || totals.Total != "1430.00" {
		t.Fatalf("calculated totals = %+v", totals)
	}
	if input.Items[0].Description != "Service A" {
		t.Fatalf("description was not trimmed: %q", input.Items[0].Description)
	}
}

func TestInputValidateRejectsInvalidInvoiceData(t *testing.T) {
	valid := Input{
		CustomerID: "00000000-0000-0000-0000-000000000001",
		IssueDate:  "2030-01-01",
		DueDate:    "2030-01-31",
		Items:      []ItemInput{{Description: "Service", Quantity: 1, UnitPrice: Money("10.00")}},
	}
	testCases := []struct {
		name   string
		change func(*Input)
		want   error
	}{
		{"missing customer", func(input *Input) { input.CustomerID = "" }, ErrCustomerRequired},
		{"invalid customer", func(input *Input) { input.CustomerID = "bad" }, ErrCustomerInvalid},
		{"missing items", func(input *Input) { input.Items = nil }, ErrItemsRequired},
		{"invalid quantity", func(input *Input) { input.Items[0].Quantity = 0 }, ErrQuantityInvalid},
		{"negative price", func(input *Input) { input.Items[0].UnitPrice = Money("-1.00") }, ErrMoneyInvalid},
		{"invalid price precision", func(input *Input) { input.Items[0].UnitPrice = Money("1.001") }, ErrMoneyInvalid},
		{"invalid date", func(input *Input) { input.IssueDate = "2030-1-01" }, ErrDateInvalid},
		{"due date before issue", func(input *Input) { input.DueDate = "2029-12-31" }, ErrDueDateBeforeIssue},
		{"negative tax", func(input *Input) { input.Tax = Money("-1") }, ErrMoneyInvalid},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			input := valid
			input.Items = append([]ItemInput(nil), valid.Items...)
			testCase.change(&input)
			if _, err := input.Validate(); !errors.Is(err, testCase.want) {
				t.Fatalf("Validate() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestPaymentValidateAndStatusTransitions(t *testing.T) {
	input := PaymentInput{Amount: Money("500"), PaymentDate: "2030-01-10", PaymentMethod: " upi "}
	paidCents, err := input.Validate()
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if paidCents != 50000 || input.PaymentMethod != "UPI" || input.Amount != Money("500.00") {
		t.Fatalf("normalized payment = %+v, cents %d", input, paidCents)
	}
	if StatusAfterPayment(100000, 50000) != StatusPartiallyPaid || StatusAfterPayment(100000, 100000) != StatusPaid {
		t.Fatal("payment status transition is incorrect")
	}

	invalid := []struct {
		name  string
		input PaymentInput
		want  error
	}{
		{"zero amount", PaymentInput{Amount: Money("0"), PaymentDate: "2030-01-10", PaymentMethod: "CASH"}, ErrPaymentAmountInvalid},
		{"invalid date", PaymentInput{Amount: Money("1"), PaymentDate: "2030-1-10", PaymentMethod: "CASH"}, ErrPaymentDateInvalid},
		{"invalid method", PaymentInput{Amount: Money("1"), PaymentDate: "2030-01-10", PaymentMethod: "CHEQUE"}, ErrPaymentMethodInvalid},
	}
	for _, testCase := range invalid {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := testCase.input.Validate(); !errors.Is(err, testCase.want) {
				t.Fatalf("Validate() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
