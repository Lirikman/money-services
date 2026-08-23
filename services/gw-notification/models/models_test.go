package models_test

import (
	"testing"

	"github.com/Lirikman/money_services/services/gw-notification/models"
)

func TestLargeTransfer_IsLarge(t *testing.T) {

	tests := []struct {
		name   string
		amount float64
		want   bool
	}{
		{
			name:   "below threshold",
			amount: 29999,
			want:   false,
		},
		{
			name:   "threshold",
			amount: 30000,
			want:   true,
		},
		{
			name:   "above threshold",
			amount: 50000,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transaction := models.LargeTransfer{Amount: tt.amount}
			got := transaction.IsLarge()
			if got != tt.want {
				t.Fatalf("IsLarge() = %v, want %v", got, tt.want)
			}
		})
	}
}
