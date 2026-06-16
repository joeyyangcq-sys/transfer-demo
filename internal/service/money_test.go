package service

import (
	"errors"
	"testing"

	"github.com/joeyyang/transfer-demo/internal/domain"
)

func TestValidateAmount(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"0", true},
		{"-1", true},
		{"100.23344", false},
		{"0.000000000000000001", false}, // 18 dp, ok
		{"0.0000000000000000001", true}, // 19 dp, too precise
		{"100000000000000000000", true}, // 10^20, out of range
	}
	for _, tc := range cases {
		err := validateAmount(dec(tc.in))
		if tc.wantErr && !errors.Is(err, domain.ErrInvalidAmount) {
			t.Errorf("validateAmount(%s) = %v, want ErrInvalidAmount", tc.in, err)
		}
		if !tc.wantErr && err != nil {
			t.Errorf("validateAmount(%s) = %v, want nil", tc.in, err)
		}
	}
}

func TestValidateInitialBalance(t *testing.T) {
	if err := validateInitialBalance(dec("0")); err != nil {
		t.Errorf("zero balance should be allowed, got %v", err)
	}
	if err := validateInitialBalance(dec("-0.01")); !errors.Is(err, domain.ErrInvalidAmount) {
		t.Errorf("negative balance: got %v, want ErrInvalidAmount", err)
	}
}
