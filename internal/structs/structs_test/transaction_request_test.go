package structs_test

import (
	"RTTH/internal/structs"
	"testing"
)

func TestClientTransactionValidate_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		txn     structs.ClientTransaction
		wantErr string
	}{
		{
			name:    "valid transaction",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "Valid Data", Timestamp: 12345},
			wantErr: "",
		},
		{
			name:    "missing clientID",
			txn:     structs.ClientTransaction{ClientID: 0, Payload: "Valid Data", Timestamp: 12345},
			wantErr: "transaction ID is required",
		},
		{
			name:    "empty payload",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "", Timestamp: 12345},
			wantErr: "payload cannot be empty",
		},
		{
			name:    "whitespace payload",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "   ", Timestamp: 12345},
			wantErr: "payload cannot be empty",
		},
		{
			name:    "missing timestamp",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "Valid Data", Timestamp: 0},
			wantErr: "timestamp is required",
		},
		{
			name:    "error order clientID first",
			txn:     structs.ClientTransaction{ClientID: 0, Payload: "", Timestamp: 0},
			wantErr: "transaction ID is required",
		},
		{
			name:    "error order payload second",
			txn:     structs.ClientTransaction{ClientID: 1, Payload: "", Timestamp: 0},
			wantErr: "payload cannot be empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := tt.txn.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error %q, got nil", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestGetRequestFields_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		req    structs.GetRequest
		wantID int
	}{
		{
			name:   "explicit client ID",
			req:    structs.GetRequest{ClientID: 42},
			wantID: 42,
		},
		{
			name:   "zero value",
			req:    structs.GetRequest{},
			wantID: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.req.ClientID != tt.wantID {
				t.Fatalf("expected ClientID %d, got %d", tt.wantID, tt.req.ClientID)
			}
		})
	}
}
