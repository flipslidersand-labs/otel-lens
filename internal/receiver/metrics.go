package receiver

import (
	"context"
	"fmt"
	"time"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/flipslidersand/otel-lens/internal/store"
)

type metricsServer struct {
	collectormetrics.UnimplementedMetricsServiceServer
	st     *store.ClickHouseStore
	logger *zap.Logger
}

func (s *metricsServer) Export(ctx context.Context, req *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	for _, rm := range req.ResourceMetrics {
		svcName := resourceAttrFromMetrics(rm)
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				samples := extractSamples(m, svcName)
				for _, sample := range samples {
					if err := s.st.InsertMetric(ctx, sample); err != nil {
						s.logger.Error("insert metric", zap.String("name", sample.Name), zap.Error(err))
					}
				}
			}
		}
	}
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

func RegisterMetricsService(srv *grpc.Server, st *store.ClickHouseStore, logger *zap.Logger) {
	collectormetrics.RegisterMetricsServiceServer(srv, &metricsServer{st: st, logger: logger})
}

func resourceAttrFromMetrics(rm *metricsv1.ResourceMetrics) string {
	if rm.Resource == nil {
		return ""
	}
	for _, a := range rm.Resource.Attributes {
		if a.Key == "service.name" {
			return a.Value.GetStringValue()
		}
	}
	return ""
}

// extractSamples flattens all data points from any metric kind into MetricSample slice.
func extractSamples(m *metricsv1.Metric, svcName string) []store.MetricSample {
	var out []store.MetricSample
	switch d := m.Data.(type) {
	case *metricsv1.Metric_Gauge:
		for _, dp := range d.Gauge.DataPoints {
			out = append(out, toSample(m.Name, svcName, dp.TimeUnixNano, dpValue(dp), dp.Attributes))
		}
	case *metricsv1.Metric_Sum:
		for _, dp := range d.Sum.DataPoints {
			out = append(out, toSample(m.Name, svcName, dp.TimeUnixNano, dpValue(dp), dp.Attributes))
		}
	case *metricsv1.Metric_Histogram:
		for _, dp := range d.Histogram.DataPoints {
			out = append(out, toSample(m.Name+".count", svcName, dp.TimeUnixNano, float64(dp.Count), dp.Attributes))
			if dp.Count > 0 {
				out = append(out, toSample(m.Name+".sum", svcName, dp.TimeUnixNano, dp.GetSum(), dp.Attributes))
			}
		}
	}
	return out
}

func dpValue(dp *metricsv1.NumberDataPoint) float64 {
	switch v := dp.Value.(type) {
	case *metricsv1.NumberDataPoint_AsDouble:
		return v.AsDouble
	case *metricsv1.NumberDataPoint_AsInt:
		return float64(v.AsInt)
	default:
		return 0
	}
}

func toSample(name, svcName string, tsNano uint64, value float64, attrs []*commonv1.KeyValue) store.MetricSample {
	ts := time.Unix(0, int64(tsNano)).UTC()
	labels := make(map[string]string, len(attrs))
	for _, a := range attrs {
		labels[a.Key] = fmt.Sprint(a.Value.Value)
	}
	return store.MetricSample{
		Name:        name,
		ServiceName: svcName,
		Timestamp:   ts,
		Value:       value,
		Labels:      labels,
	}
}
