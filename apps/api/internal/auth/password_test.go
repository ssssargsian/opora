package auth

import "testing"

func TestValidateNewPassword(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		password string
		valid    bool
	}{
		{name: "eight characters", password: "opora123", valid: true},
		{name: "unicode letters", password: "Опора123", valid: true},
		{name: "too short", password: "test123", valid: false},
		{name: "no digit", password: "opora-password", valid: false},
		{name: "no letter", password: "12345678", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateNewPassword(test.password)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateNewPassword() error = %v, valid = %v", err, test.valid)
			}
		})
	}
}
