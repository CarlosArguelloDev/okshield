package telemetry

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const sampleOTLPLogsJSON = `{
  "resourceLogs": [{
    "resource": {
      "attributes": [{"key": "service.name", "value": {"stringValue": "checkout-svc"}}]
    },
    "scopeLogs": [{
      "logRecords": [
        {"timeUnixNano": "1700000000000000000", "severityText": "ERROR", "body": {"stringValue": "Connection refused to 10.0.0.9:5432"}},
        {"timeUnixNano": "1700000001000000000", "severityText": "WARN",  "body": {"stringValue": "retry attempt=1"}}
      ]
    }]
  }]
}`

func TestOTLPReceiverIngestsLogsIntoRingBuffer(t *testing.T) {
	st := NewStore("", time.Hour)
	defer st.Close()
	rcv := NewReceiver(st, DefaultOTLPGRPCAddr, DefaultOTLPHTTPAddr)

	req := httptest.NewRequest(http.MethodPost, "/v1/logs", bytes.NewBufferString(sampleOTLPLogsJSON))
	rec := httptest.NewRecorder()
	rcv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	snap := st.Ring().Snapshot()
	if len(snap) != 2 {
		t.Fatalf("ring len = %d, want 2", len(snap))
	}
	if snap[0].Source != "checkout-svc" || snap[0].Severity != "ERROR" {
		t.Errorf("record[0] = %+v", snap[0])
	}
}
