// Package observability provides structured logging and Prometheus
// metrics shared across the platform.
package observability

import (
	"encoding/json"
	"io"
	"os"
	"time"
)

type Logger struct {
	out io.Writer
}

func NewLogger() *Logger { return &Logger{out: os.Stdout} }

type logLine struct {
	Timestamp     string `json:"timestamp"`
	Level         string `json:"level"`
	Message       string `json:"message"`
	RequestID     string `json:"request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

func (l *Logger) Log(level, msg, requestID, correlationID, tenantID string, fields map[string]any) {
	line := logLine{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Level: level, Message: msg,
		RequestID: requestID, CorrelationID: correlationID, TenantID: tenantID, Fields: fields,
	}
	b, _ := json.Marshal(line)
	l.out.Write(append(b, '\n'))
}

func (l *Logger) Info(msg string, fields map[string]any)  { l.Log("info", msg, "", "", "", fields) }
func (l *Logger) Error(msg string, fields map[string]any) { l.Log("error", msg, "", "", "", fields) }
