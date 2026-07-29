package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/dto"
)

// RenderReportCSV serializes a ReportSummary into a multi-section CSV string.
// It is a pure transform of an already-computed aggregation DTO, so it stays unit-testable.
func RenderReportCSV(s dto.ReportSummary) string {
	var b strings.Builder

	writeLine(&b, "new-api report")
	writeLine(&b, "range,"+string(s.Range)+",granularity,"+string(s.Granularity))
	writeLine(&b, "period_start,"+strconv.FormatInt(s.PeriodStart, 10)+",period_end,"+strconv.FormatInt(s.PeriodEnd, 10))
	writeLine(&b, "role,"+string(s.Role))
	writeLine(&b, "")

	writeLine(&b, "section,key,value")
	writeLine(&b, "totals,quota,"+strconv.Itoa(s.Trend.Totals.Quota))
	writeLine(&b, "totals,requests,"+strconv.Itoa(s.Trend.Totals.Requests))
	writeLine(&b, "totals,tokens,"+strconv.Itoa(s.Trend.Totals.Tokens))
	writeLine(&b, "totals,failures,"+strconv.Itoa(s.Trend.Totals.Failures))
	writeLine(&b, "totals,avg_latency_ms,"+strconv.Itoa(s.Trend.Totals.AvgLatencyMs))
	writeLine(&b, "")

	writeLine(&b, "trend,bucket,quota,requests,tokens,failures,avg_latency_ms")
	for _, p := range s.Trend.Points {
		writeLine(&b, fmt.Sprintf("trend,%s,%d,%d,%d,%d,%d",
			p.BucketLabel, p.Quota, p.Requests, p.Tokens, p.Failures, p.AvgLatencyMs))
	}
	writeLine(&b, "")

	writeLine(&b, "models,model_name,quota,requests,tokens,failures,error_rate,avg_latency_ms")
	for _, m := range s.Models {
		writeLine(&b, fmt.Sprintf("models,%s,%d,%d,%d,%d,%s,%d",
			csvField(m.ModelName), m.Quota, m.Requests, m.Tokens, m.Failures,
			strconv.FormatFloat(m.ErrorRate, 'f', 4, 64), m.AvgLatencyMs))
	}
	writeLine(&b, "")

	writeLine(&b, "errors,model_name,failures,quota,share_percent")
	for _, e := range s.Errors {
		writeLine(&b, fmt.Sprintf("errors,%s,%d,%d,%d",
			csvField(e.ModelName), e.Failures, e.Quota, e.Share))
	}
	writeLine(&b, "")

	writeLine(&b, "model_performance,name,avg_latency_ms,p95_latency_ms,requests,tokens,throughput")
	for _, p := range s.ModelPerformance {
		writeLine(&b, fmt.Sprintf("model_performance,%s,%d,%d,%d,%d,%d",
			csvField(p.Name), p.AvgLatencyMs, p.P95LatencyMs, p.Requests, p.Tokens, p.Throughput))
	}
	writeLine(&b, "")

	writeLine(&b, "token_anatomy,model_name,prompt_tokens,completion_tokens,cache_tokens,total")
	for _, t := range s.TokenAnatomy {
		writeLine(&b, fmt.Sprintf("token_anatomy,%s,%d,%d,%d,%d",
			csvField(t.ModelName), t.PromptTokens, t.CompletionTokens, t.CacheTokens, t.Total))
	}
	writeLine(&b, "")

	if len(s.Channels) > 0 {
		writeLine(&b, "channels,channel_id,channel_name,quota,requests,failures,error_rate,avg_latency_ms")
		for _, ch := range s.Channels {
			writeLine(&b, fmt.Sprintf("channels,%d,%s,%d,%d,%d,%s,%d",
				ch.ChannelID, csvField(ch.ChannelName), ch.Quota, ch.Requests, ch.Failures,
				strconv.FormatFloat(ch.ErrorRate, 'f', 4, 64), ch.AvgLatencyMs))
		}
		writeLine(&b, "")
	}

	if s.StreamPerformance != nil {
		writeLine(&b, "stream_comparison,stream_avg_latency_ms,stream_requests,non_stream_avg_latency_ms,non_stream_requests")
		writeLine(&b, fmt.Sprintf("stream_comparison,%d,%d,%d,%d",
			s.StreamPerformance.StreamAvgLatencyMs, s.StreamPerformance.StreamRequests,
			s.StreamPerformance.NonStreamAvgLatencyMs, s.StreamPerformance.NonStreamRequests))
	}

	return b.String()
}

func writeLine(b *strings.Builder, line string) {
	b.WriteString(line)
	b.WriteString("\r\n")
}

// csvField quotes a field only when it contains characters that require quoting.
func csvField(s string) string {
	if s == "" {
		return ""
	}
	if strings.ContainsAny(s, ",\"\r\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
