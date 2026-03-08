package test

import (
	"RTTH/internal/structs"
	"testing"
)

func TestClientTransaction_Validate(t *testing.T) {
	tests := []struct {
		name    string
		txn     structs.ClientTransaction
		wantErr string
	}{
		{
			name:    "Valid Transaction",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "Valid Data", Timestamp: 12345},
			wantErr: "",
		},
		{
			name:    "Missing ClientID",
			txn:     structs.ClientTransaction{ClientID: 0, Payload: "Valid Data", Timestamp: 12345},
			wantErr: "transaction ID is required",
		},
		{
			name:    "Empty Payload",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "   ", Timestamp: 12345},
			wantErr: "payload cannot be empty",
		},
		{
			name:    "Missing Timestamp",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "Valid Data", Timestamp: 0},
			wantErr: "Timestamp is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.txn.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != tt.wantErr {
					t.Errorf("expected error '%s', got '%v'", tt.wantErr, err)
				}
			}
		})
	}
}