package technicians

import (
	"errors"
	"testing"
)

func TestInputValidate(t *testing.T) {
	input := Input{Name: "  Priya Singh ", Email: " priya@example.test ", Phone: " 9876543210 "}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid input was rejected: %v", err)
	}
	if input.Name != "Priya Singh" || input.Email != "priya@example.test" || input.Phone != "9876543210" {
		t.Fatalf("input was not trimmed: %+v", input)
	}

	testCases := []struct {
		name  string
		input Input
		want  error
	}{
		{name: "missing name", input: Input{Email: "person@example.test"}, want: ErrNameRequired},
		{name: "blank name", input: Input{Name: "   "}, want: ErrNameRequired},
		{name: "invalid email", input: Input{Name: "Technician", Email: "not-an-email"}, want: ErrEmailInvalid},
		{name: "long email", input: Input{Name: "Technician", Email: string(make([]byte, 255))}, want: ErrEmailTooLong},
		{name: "long phone", input: Input{Name: "Technician", Phone: string(make([]byte, 51))}, want: ErrPhoneTooLong},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.input.Validate(); !errors.Is(err, testCase.want) {
				t.Fatalf("Validate() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestValidID(t *testing.T) {
	if !ValidID("00000000-0000-0000-0000-000000000001") {
		t.Fatal("valid UUID was rejected")
	}
	if ValidID("not-a-uuid") {
		t.Fatal("invalid UUID was accepted")
	}
}
