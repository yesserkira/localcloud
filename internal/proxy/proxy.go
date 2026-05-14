package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/localcloud-dev/localcloud/internal/adapters"
	"github.com/localcloud-dev/localcloud/internal/id"
	"github.com/localcloud-dev/localcloud/internal/redaction"
	"github.com/localcloud-dev/localcloud/internal/timeline"
)

// Proxy is a reverse proxy that captures HTTP traffic as timeline events.
type Proxy struct {
	serviceName       string
	targetURL         *url.URL
	correlationHeader string
	runID             string
	listenAddr        string
	actualAddr        string

	redactionPolicy *redaction.Policy
	bodyMaxBytes    int
	server          *http.Server
	sink            adapters.EventSink
	logger          *slog.Logger
}

// Config for the HTTP proxy.
type Config struct {
	ServiceName       string
	TargetBaseURL     string
	ListenAddr        string
	RunID             string
	CorrelationHeader string
	RedactionPolicy   *redaction.Policy
	BodyMaxBytes      int
}

// New creates a new capture proxy.
func New(cfg Config, logger *slog.Logger) (*Proxy, error) {
	target, err := url.Parse(cfg.TargetBaseURL)
	if err != nil {
		return nil, fmt.Errorf("proxy: invalid target URL %q: %w", cfg.TargetBaseURL, err)
	}

	if cfg.CorrelationHeader == "" {
		cfg.CorrelationHeader = "x-localcloud-correlation-id"
	}
	if cfg.BodyMaxBytes <= 0 {
		cfg.BodyMaxBytes = 8192
	}
	if cfg.RedactionPolicy == nil {
		cfg.RedactionPolicy = redaction.DefaultPolicy()
	}

	return &Proxy{
		serviceName:       cfg.ServiceName,
		targetURL:         target,
		correlationHeader: cfg.CorrelationHeader,
		runID:             cfg.RunID,
		listenAddr:        cfg.ListenAddr,
		redactionPolicy:   cfg.RedactionPolicy,
		bodyMaxBytes:      cfg.BodyMaxBytes,
		logger:            logger,
	}, nil
}

// Start begins serving and capturing HTTP requests.
func (p *Proxy) Start(ctx context.Context, sink adapters.EventSink) error {
	p.sink = sink

	rp := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = p.targetURL.Scheme
			req.URL.Host = p.targetURL.Host
			req.Host = p.targetURL.Host
		},
		ModifyResponse: p.captureResponse,
		ErrorHandler:   p.handleError,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Inject correlation ID if not present
		if r.Header.Get(p.correlationHeader) == "" {
			r.Header.Set(p.correlationHeader, id.Correlation())
		}

		// Store start time for duration calculation
		ctx := context.WithValue(r.Context(), ctxKeyStart, time.Now())
		ctx = context.WithValue(ctx, ctxKeyCorrelation, r.Header.Get(p.correlationHeader))
		ctx = context.WithValue(ctx, ctxKeyEventID, id.Event())

		// Capture request body
		var bodyBuf bytes.Buffer
		if r.Body != nil {
			limited := io.LimitReader(r.Body, int64(p.bodyMaxBytes)+1)
			io.Copy(&bodyBuf, limited)
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(bodyBuf.Bytes()), r.Body))
		}
		ctx = context.WithValue(ctx, ctxKeyReqBody, bodyBuf.Bytes())

		r = r.WithContext(ctx)
		rp.ServeHTTP(w, r)
	})

	p.server = &http.Server{
		Addr:    p.listenAddr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("proxy: listen %s: %w", p.listenAddr, err)
	}
	p.actualAddr = ln.Addr().String()

	p.logger.Info("proxy listening", "addr", p.actualAddr, "target", p.targetURL.String())
	go p.server.Serve(ln)
	return nil
}

// Addr returns the actual address the proxy is listening on.
// Useful when started with port 0 (random port).
func (p *Proxy) Addr() string { return p.actualAddr }

// Stop gracefully shuts down the proxy.
func (p *Proxy) Stop(ctx context.Context) error {
	if p.server == nil {
		return nil
	}
	return p.server.Shutdown(ctx)
}

func (p *Proxy) captureResponse(resp *http.Response) error {
	ctx := resp.Request.Context()
	start, _ := ctx.Value(ctxKeyStart).(time.Time)
	correlationID, _ := ctx.Value(ctxKeyCorrelation).(string)
	eventID, _ := ctx.Value(ctxKeyEventID).(string)
	reqBodyBytes, _ := ctx.Value(ctxKeyReqBody).([]byte)

	duration := time.Since(start).Milliseconds()
	req := resp.Request

	// Read response body preview
	var respBody bytes.Buffer
	if resp.Body != nil {
		limited := io.LimitReader(resp.Body, int64(p.bodyMaxBytes)+1)
		io.Copy(&respBody, limited)
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(respBody.Bytes()), resp.Body))
	}

	// Build request headers map
	reqHeaders := make(map[string]string)
	for k, v := range req.Header {
		reqHeaders[k] = strings.Join(v, ", ")
	}
	p.redactionPolicy.RedactHeaders(reqHeaders)

	// Build response headers map
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}
	p.redactionPolicy.RedactHeaders(respHeaders)

	// Redact request body
	reqBodyPreview := string(reqBodyBytes)
	reqBodyRedacted := false
	if len(reqBodyPreview) > p.bodyMaxBytes {
		reqBodyPreview = reqBodyPreview[:p.bodyMaxBytes]
	}
	redactedReqBody, n := p.redactionPolicy.RedactJSONBody(reqBodyPreview)
	if n > 0 {
		reqBodyPreview = redactedReqBody
		reqBodyRedacted = true
	}

	// Redact response body
	respBodyPreview := string(respBody.Bytes())
	respBodyRedacted := false
	if len(respBodyPreview) > p.bodyMaxBytes {
		respBodyPreview = respBodyPreview[:p.bodyMaxBytes]
	}
	redactedRespBody, n := p.redactionPolicy.RedactJSONBody(respBodyPreview)
	if n > 0 {
		respBodyPreview = redactedRespBody
		respBodyRedacted = true
	}

	// Determine status
	status := timeline.StatusOK
	if resp.StatusCode >= 500 {
		status = timeline.StatusError
	} else if resp.StatusCode >= 400 {
		status = timeline.StatusWarning
	}

	// Replayable: safe methods + POST are replayable with warning
	method := req.Method
	replayable := true
	replayWarning := ""
	if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
		replayWarning = fmt.Sprintf("%s is an unsafe method — replay may create side effects", method)
	}

	event := timeline.TimelineEvent{
		ID:        eventID,
		RunID:     p.runID,
		Timestamp: start,
		Source:    timeline.SourceHTTPProxy,
		Service:  p.serviceName,
		Action:   timeline.ActionHTTPRequest,
		Summary:  fmt.Sprintf("%s %s → %d", method, req.URL.Path, resp.StatusCode),
		Status:   status,
		DurationMs: &duration,
		CorrelationID: strPtr(correlationID),
		Request: &timeline.RequestData{
			Method:        method,
			Scheme:        req.URL.Scheme,
			Host:          req.URL.Host,
			Path:          req.URL.Path,
			Query:         req.URL.RawQuery,
			Headers:       reqHeaders,
			BodyPreview:   reqBodyPreview,
			BodySHA256:    sha256Hex(reqBodyBytes),
			BodyRedacted:  reqBodyRedacted,
			Replayable:    replayable,
			ReplayWarning: replayWarning,
		},
		Response: &timeline.ResponseData{
			StatusCode:   resp.StatusCode,
			Headers:      respHeaders,
			BodyPreview:  respBodyPreview,
			BodySHA256:   sha256Hex(respBody.Bytes()),
			BodyRedacted: respBodyRedacted,
		},
	}

	go func() {
		if err := p.sink.Emit(context.Background(), event); err != nil {
			p.logger.Error("proxy: emit event failed", "err", err)
		}
	}()

	return nil
}

func (p *Proxy) handleError(w http.ResponseWriter, r *http.Request, err error) {
	p.logger.Error("proxy: upstream error", "path", r.URL.Path, "err", err)
	http.Error(w, "Bad Gateway", http.StatusBadGateway)
}

// Context keys
type ctxKey int

const (
	ctxKeyStart ctxKey = iota
	ctxKeyCorrelation
	ctxKeyEventID
	ctxKeyReqBody
)

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func sha256Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
