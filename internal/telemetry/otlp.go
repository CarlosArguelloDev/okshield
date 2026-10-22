package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Default OTLP endpoints (AGENTS.md §3).
const (
	DefaultOTLPGRPCAddr = "127.0.0.1:4317"
	DefaultOTLPHTTPAddr = "127.0.0.1:4318"
)

// otlpLogsPayload models the standard OpenTelemetry JSON `/v1/logs` schema.
type otlpLogsPayload struct {
	ResourceLogs []struct {
		Resource struct {
			Attributes []struct {
				Key   string `json:"key"`
				Value struct {
					StringValue string `json:"stringValue"`
				} `json:"value"`
			} `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			LogRecords []struct {
				TimeUnixNano string `json:"timeUnixNano"`
				SeverityText string `json:"severityText"`
				Body         struct {
					StringValue string `json:"stringValue"`
				} `json:"body"`
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	} `json:"resourceLogs"`
}

var payloadPool = sync.Pool{
	New: func() any {
		return &otlpLogsPayload{}
	},
}

// Receiver is the embedded OTLP HTTP (:4318) and TCP/gRPC (:4317) receiver.
type Receiver struct {
	store     *Store
	grpcAddr  string
	httpAddr  string
	httpSrv   *http.Server
	grpcLn    net.Listener
	listening atomic.Bool
}

// NewReceiver creates an OTLP receiver writing into store.
func NewReceiver(store *Store, grpcAddr, httpAddr string) *Receiver {
	if grpcAddr == "" {
		grpcAddr = DefaultOTLPGRPCAddr
	}
	if httpAddr == "" {
		httpAddr = DefaultOTLPHTTPAddr
	}
	r := &Receiver{
		store:    store,
		grpcAddr: grpcAddr,
		httpAddr: httpAddr,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/logs", r.handleLogs)
	mux.HandleFunc("/v1/metrics", r.handleMetrics)
	r.httpSrv = &http.Server{
		Addr:              httpAddr,
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
	return r
}

// Start begins listening on HTTP (:4318) and TCP (:4317) in background
// goroutines until ctx is cancelled.
func (r *Receiver) Start(ctx context.Context) error {
	httpLn, err := net.Listen("tcp", r.httpAddr)
	if err != nil {
		return err
	}
	grpcLn, err := net.Listen("tcp", r.grpcAddr)
	if err == nil {
		r.grpcLn = grpcLn
		go r.acceptGRPC(ctx, grpcLn)
	}
	r.listening.Store(true)

	go func() {
		<-ctx.Done()
		r.listening.Store(false)
		shutCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = r.httpSrv.Shutdown(shutCtx)
		if r.grpcLn != nil {
			_ = r.grpcLn.Close()
		}
	}()
	go func() {
		_ = r.httpSrv.Serve(httpLn)
	}()
	return nil
}

// Handler returns the HTTP handler for in-process testing.
func (r *Receiver) Handler() http.Handler { return r.httpSrv.Handler }

// Stats returns the current receiver + store telemetry statistics.
func (r *Receiver) Stats() StoreStats {
	return r.store.Stats(r.grpcAddr, r.httpAddr, r.listening.Load())
}

func (r *Receiver) acceptGRPC(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		_ = conn.Close()
	}
}

func (r *Receiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer req.Body.Close()

	p := payloadPool.Get().(*otlpLogsPayload)
	p.ResourceLogs = p.ResourceLogs[:0]
	defer payloadPool.Put(p)

	if err := json.NewDecoder(io.LimitReader(req.Body, 256*1024)).Decode(p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	now := time.Now()
	for _, rl := range p.ResourceLogs {
		svc := "otlp"
		for _, attr := range rl.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.StringValue != "" {
				svc = attr.Value.StringValue
				break
			}
		}
		for _, sl := range rl.ScopeLogs {
			for _, lr := range sl.LogRecords {
				ts := now
				if ns, err := strconv.ParseInt(lr.TimeUnixNano, 10, 64); err == nil && ns > 0 {
					ts = time.Unix(0, ns)
				}
				sev := lr.SeverityText
				if sev == "" {
					sev = "INFO"
				}
				r.store.Append(LogRecord{
					Timestamp: ts,
					Source:    svc,
					Severity:  sev,
					Body:      lr.Body.StringValue,
				})
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}

func (r *Receiver) handleMetrics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(req.Body, 64*1024))
	_ = req.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"partialSuccess":{}}`))
}
