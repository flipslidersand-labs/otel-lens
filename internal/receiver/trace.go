package receiver

import (
	"context"
	"fmt"
	"net"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/flipslidersand/otel-lens/internal/store"
)

type traceServer struct {
	collectortrace.UnimplementedTraceServiceServer
	st     *store.ClickHouseStore
	logger *zap.Logger
}

func (s *traceServer) Export(ctx context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	for _, rs := range req.ResourceSpans {
		svcName := resourceAttr(rs, "service.name")
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				if err := s.st.InsertSpan(ctx, toSpan(span, svcName)); err != nil {
					s.logger.Error("insert span", zap.String("trace_id", fmt.Sprintf("%x", span.TraceId)), zap.Error(err))
				}
			}
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func Serve(addr string, st *store.ClickHouseStore, logger *zap.Logger) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	srv := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(srv, &traceServer{st: st, logger: logger})
	logger.Info("OTLP gRPC receiver listening", zap.String("addr", addr))
	return srv.Serve(lis)
}

func resourceAttr(rs *tracev1.ResourceSpans, key string) string {
	if rs.Resource == nil {
		return ""
	}
	for _, a := range rs.Resource.Attributes {
		if a.Key == key {
			return a.Value.GetStringValue()
		}
	}
	return ""
}

func toSpan(s *tracev1.Span, svcName string) store.Span {
	start := time.Unix(0, int64(s.StartTimeUnixNano)).UTC()
	end := time.Unix(0, int64(s.EndTimeUnixNano)).UTC()

	attrs := make(map[string]string, len(s.Attributes))
	for _, a := range s.Attributes {
		attrs[a.Key] = fmt.Sprint(a.Value.Value)
	}

	return store.Span{
		TraceID:      fmt.Sprintf("%x", s.TraceId),
		SpanID:       fmt.Sprintf("%x", s.SpanId),
		ParentSpanID: fmt.Sprintf("%x", s.ParentSpanId),
		ServiceName:  svcName,
		Operation:    s.Name,
		StartTime:    start,
		EndTime:      end,
		DurationMs:   float64(end.Sub(start).Microseconds()) / 1000.0,
		StatusCode:   uint8(s.Status.GetCode()),
		Attributes:   attrs,
	}
}
