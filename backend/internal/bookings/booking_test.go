package bookings

import (
	"errors"
	"testing"
)

func TestAppointmentInputValidate(t *testing.T) {
	input := AppointmentInput{
		CustomerID:      "00000000-0000-0000-0000-000000000001",
		ServiceID:       "00000000-0000-0000-0000-000000000002",
		AppointmentDate: "2030-01-15", StartTime: "10:00", EndTime: "11:00",
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid appointment without a technician was rejected: %v", err)
	}

	testCases := []struct {
		name  string
		input AppointmentInput
		want  error
	}{
		{name: "missing customer", input: AppointmentInput{ServiceID: "00000000-0000-0000-0000-000000000002", AppointmentDate: "2030-01-15", StartTime: "10:00", EndTime: "11:00"}, want: ErrCustomerIDRequired},
		{name: "invalid service", input: AppointmentInput{CustomerID: input.CustomerID, ServiceID: "invalid", AppointmentDate: "2030-01-15", StartTime: "10:00", EndTime: "11:00"}, want: ErrServiceIDRequired},
		{name: "invalid technician", input: AppointmentInput{CustomerID: input.CustomerID, ServiceID: input.ServiceID, TechnicianID: "invalid", AppointmentDate: "2030-01-15", StartTime: "10:00", EndTime: "11:00"}, want: ErrTechnicianIDInvalid},
		{name: "invalid date", input: AppointmentInput{CustomerID: input.CustomerID, ServiceID: input.ServiceID, AppointmentDate: "2030-2-15", StartTime: "10:00", EndTime: "11:00"}, want: ErrBookingDateInvalid},
		{name: "invalid time", input: AppointmentInput{CustomerID: input.CustomerID, ServiceID: input.ServiceID, AppointmentDate: "2030-01-15", StartTime: "25:00", EndTime: "26:00"}, want: ErrBookingTimeInvalid},
		{name: "end equals start", input: AppointmentInput{CustomerID: input.CustomerID, ServiceID: input.ServiceID, AppointmentDate: "2030-01-15", StartTime: "10:00", EndTime: "10:00"}, want: ErrBookingEndBeforeStart},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.input.Validate(); !errors.Is(err, testCase.want) {
				t.Fatalf("Validate() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestAppointmentUpdateRejectsUnknownStatus(t *testing.T) {
	input := AppointmentUpdateInput{
		AppointmentInput: AppointmentInput{
			CustomerID:      "00000000-0000-0000-0000-000000000001",
			ServiceID:       "00000000-0000-0000-0000-000000000002",
			AppointmentDate: "2030-01-15", StartTime: "10:00", EndTime: "11:00",
		},
		Status: "SCHEDULED",
	}
	if err := input.Validate(); !errors.Is(err, ErrBookingStatusInvalid) {
		t.Fatalf("Validate() error = %v, want %v", err, ErrBookingStatusInvalid)
	}
}
