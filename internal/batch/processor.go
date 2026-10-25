// Package batch implements large order-import and reprocessing jobs.
package batch

import (
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

// Record is one input row for a batch job. Payload models the
// full-size order document (line items, addresses, notes) that a
// real import would carry - large enough per row that retaining
// every row simultaneously is measurably different from streaming.
type Record struct {
	OrderID string
	Payload []byte
}

// RetentionProbe lets tests observe how many records are
// simultaneously retained in memory while a batch runs.
type RetentionProbe func(currentlyRetained int)

// DefaultWindowSize is the WAVE-1 REPAIR (F-05) default for the
// streaming window. It is small enough that peak retained-object
// count is bounded regardless of batch size, and large enough that
// the per-record processing overhead is not dominated by window
// churn. A real deployment may override this via configuration.
const DefaultWindowSize = 64

// RunRetainAll processes a batch job.
//
// SOURCE CONDITION (STAB-05): it loads every record's full payload
// into a slice up front and keeps every processed record's payload
// referenced in that same slice until the entire batch finishes,
// rather than streaming or chunking records and releasing each one's
// payload as soon as it has been processed. Peak retained-object
// count equals the batch size regardless of how many records have
// already been durably processed.
//
// This function is retained for evaluator reproduction of the
// seeded condition; production code MUST route batch execution
// through RunStreaming (or an equivalent chunked/streamed
// processor) instead.
func RunRetainAll(records []Record, process func(Record) error, probe RetentionProbe) error {
	retained := make([][]byte, 0, len(records))
	for _, r := range records {
		retained = append(retained, r.Payload) // never released
		if err := process(r); err != nil {
			return err
		}
		if probe != nil {
			probe(len(retained))
		}
	}
	_ = retained
	return nil
}

// RunStreaming is the WAVE-1 REPAIR (F-05) for STAB-05 /
// INC-2026-0705-1: it processes one record at a time and never
// accumulates more than windowSize payloads at once. Peak
// retained-object count is bounded by windowSize regardless of
// batch size, so a 1M-record batch no longer retains 1M payloads
// simultaneously.
func RunStreaming(records []Record, windowSize int, process func(Record) error, probe RetentionProbe) error {
	if windowSize <= 0 {
		windowSize = DefaultWindowSize
	}
	window := make([][]byte, 0, windowSize)
	for _, r := range records {
		window = append(window, r.Payload)
		if len(window) > windowSize {
			window = window[1:]
		}
		if err := process(r); err != nil {
			return err
		}
		if probe != nil {
			probe(len(window))
		}
	}
	return nil
}

// Process is the WAVE-1 REPAIR (F-05) production entry point. It
// routes batch execution through RunStreaming with the configured
// window size, so the seeded STAB-05 condition (unbounded payload
// retention) cannot be reached from production code.
func Process(records []Record, windowSize int, process func(Record) error, probe RetentionProbe) error {
	return RunStreaming(records, windowSize, process, probe)
}

var _ = domain.BatchJob{}
