package main

import (
	"context"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	tracer trace.Tracer
	meter  metric.Meter
	logger *zap.Logger
)

func MonitorMemory(ctx context.Context) {
	for {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		gauge, err := meter.Int64Gauge("websocket_memory_usage_bytes")
		if err == nil {
			gauge.Record(ctx, int64(m.Alloc))
		}
		time.Sleep(5 * time.Second)
	}
}

func InitTelemetry() (func(), error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("websocket-server"),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	promExporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(promExporter),
	)
	otel.SetMeterProvider(meterProvider)

	// config := zap.NewProductionConfig()
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	// config.OutputPaths = []string{"./server.log"}
	logger, err = config.Build()
	if err != nil {
		return nil, err
	}

	tracer = otel.Tracer("websocket-server")
	meter = otel.Meter("websocket-server")

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tp.Shutdown(ctx)
		meterProvider.Shutdown(ctx)
		logger.Sync()
	}
	return shutdown, nil
}
