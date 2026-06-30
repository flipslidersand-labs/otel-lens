package correlator

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/flipslidersand/otel-lens/internal/store"
)

// Candidate is a ranked correlation result.
type Candidate struct {
	Description string
	Score       float64  // |Pearson r|, 0.0–1.0
	Evidence    []string // human-readable supporting details
	TraceIDs    []string
}

// Score computes Pearson correlation between the error-rate timeseries and each
// metric timeseries, then merges with per-service error counts from traces.
// It returns candidates ranked by Score descending.
func Score(ctx context.Context, st *store.ClickHouseStore, from, to time.Time, errorThreshold float64) ([]Candidate, error) {
	// 1. Error-rate timeseries (per minute)
	errorBuckets, err := st.QueryErrorRate(ctx, from, to, "")
	if err != nil {
		return nil, fmt.Errorf("error rate query: %w", err)
	}
	if len(errorBuckets) == 0 {
		return nil, nil
	}

	// Build time → error_rate map
	errRateByTime := make(map[time.Time]float64, len(errorBuckets))
	for _, b := range errorBuckets {
		errRateByTime[b.Bucket.Truncate(time.Minute)] = b.ErrorRatePct
	}

	// 2. Metric timeseries
	metricBuckets, err := st.QueryMetricTimeseries(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("metric timeseries query: %w", err)
	}

	// Group by (metric_name, service_name) → []value aligned to error-rate time axis
	type key struct{ metric, service string }
	seriesMap := map[key][]float64{}
	for _, b := range metricBuckets {
		k := key{b.MetricName, b.ServiceName}
		seriesMap[k] = append(seriesMap[k], b.AvgValue)
	}

	// Build aligned error-rate vector (sorted buckets)
	sortedTimes := make([]time.Time, 0, len(errRateByTime))
	for t := range errRateByTime {
		sortedTimes = append(sortedTimes, t)
	}
	sort.Slice(sortedTimes, func(i, j int) bool { return sortedTimes[i].Before(sortedTimes[j]) })

	errVec := make([]float64, len(sortedTimes))
	for i, t := range sortedTimes {
		errVec[i] = errRateByTime[t]
	}

	// 3. Compute Pearson r for each metric series
	var candidates []Candidate
	seenMetrics := map[key]bool{}

	for _, b := range metricBuckets {
		k := key{b.MetricName, b.ServiceName}
		if seenMetrics[k] {
			continue
		}
		seenMetrics[k] = true

		// Build metric value vector aligned to the same time axis
		metricByTime := make(map[time.Time]float64)
		for _, mb := range metricBuckets {
			if mb.MetricName == k.metric && mb.ServiceName == k.service {
				metricByTime[mb.Bucket.Truncate(time.Minute)] = mb.AvgValue
			}
		}
		metVec := make([]float64, len(sortedTimes))
		for i, t := range sortedTimes {
			metVec[i] = metricByTime[t] // 0 if missing
		}

		r := pearson(errVec, metVec)
		if math.IsNaN(r) {
			continue
		}
		absR := math.Abs(r)
		if absR < 0.3 {
			continue // ignore weak correlations
		}

		direction := "positively"
		if r < 0 {
			direction = "negatively"
		}
		candidates = append(candidates, Candidate{
			Description: fmt.Sprintf("%s/%s %s correlated with error rate (r=%.2f)", k.service, k.metric, direction, r),
			Score:       absR,
			Evidence: []string{
				fmt.Sprintf("metric: %s, service: %s", k.metric, k.service),
				fmt.Sprintf("Pearson r = %.4f over %d minute buckets", r, len(sortedTimes)),
			},
		})
	}

	// 4. Add per-service error-count ranking from traces
	spans, err := st.QueryTraces(ctx, from, to, "")
	if err != nil {
		return nil, fmt.Errorf("traces query: %w", err)
	}
	errorsByService := map[string]struct {
		errors, total int
		traceIDs      []string
	}{}
	for _, sp := range spans {
		e := errorsByService[sp.ServiceName]
		e.total++
		if sp.StatusCode == 2 {
			e.errors++
			if len(e.traceIDs) < 5 {
				e.traceIDs = append(e.traceIDs, sp.TraceID)
			}
		}
		errorsByService[sp.ServiceName] = e
	}
	for svc, stats := range errorsByService {
		if stats.errors == 0 {
			continue
		}
		rate := float64(stats.errors) / float64(stats.total)
		if rate < errorThreshold/100 {
			continue
		}
		candidates = append(candidates, Candidate{
			Description: fmt.Sprintf("service %q has high error rate (%.1f%%, %d/%d spans)",
				svc, rate*100, stats.errors, stats.total),
			Score:    rate,
			Evidence: []string{fmt.Sprintf("%d error spans out of %d total", stats.errors, stats.total)},
			TraceIDs: stats.traceIDs,
		})
	}

	// 5. Sort by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	return candidates, nil
}

// pearson computes the Pearson correlation coefficient of two equal-length slices.
func pearson(x, y []float64) float64 {
	n := len(x)
	if n == 0 || n != len(y) {
		return math.NaN()
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}
	fn := float64(n)
	num := fn*sumXY - sumX*sumY
	den := math.Sqrt((fn*sumX2 - sumX*sumX) * (fn*sumY2 - sumY*sumY))
	if den == 0 {
		return math.NaN()
	}
	return num / den
}
