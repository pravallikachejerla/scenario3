package public_test

import (
	"strings"
	"testing"

	"github.com/soma-genesis/scenario-3-order-stability/internal/security"
)

func TestRedact_RemovesCardNumber(t *testing.T) {
	in := `{"order_id":"ord-1","card_number":"4111111111111111"}`
	out := security.Redact(in)
	if strings.Contains(out, "4111111111111111") {
		t.Fatalf("expected card number to be redacted, got %q", out)
	}
	if !strings.Contains(out, "order_id") {
		t.Fatalf("expected non-sensitive fields to survive redaction, got %q", out)
	}
}
