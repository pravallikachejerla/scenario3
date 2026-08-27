// Package validation provides bounded, strict request validation
// shared by the HTTP layer.
package validation

import "fmt"

const MaxItemsPerOrder = 200
const MaxBatchSize = 5000

func RequireNonEmpty(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

func RequireBounded(field string, n, max int) error {
	if n <= 0 {
		return fmt.Errorf("%s must be positive", field)
	}
	if n > max {
		return fmt.Errorf("%s exceeds maximum of %d", field, max)
	}
	return nil
}
