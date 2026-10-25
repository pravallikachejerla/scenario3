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

// RunRetainAll processes a batch job.
//
// SOURCE CONDITION (STAB-05): it loads every record's full payload
// into a slice up front and keeps every processed record's payload
// referenced in that same slice until the entire batch finishes,
// rather than streaming or chunking records and releasing each one's
// payload as soon as it has been processed. Peak retained-object
// count equals the batch size regardless of how many records have
// already been durably processed.
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

// RunStreaming is the corrected primitive: it processes one record at
// a time and never accumulates more than windowSize payloads at once.
// A repair is expected to route batch execution through something
// with this shape (or equivalent chunked/streamed processing) instead
// of RunRetainAll.
func RunStreaming(records []Record, windowSize int, process func(Record) error, probe RetentionProbe) error {
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

var _ = domain.BatchJob{}
