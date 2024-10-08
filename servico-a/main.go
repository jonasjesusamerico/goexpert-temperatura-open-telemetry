package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type CEPRequest struct {
	CEP string `json:"cep"`
}

func main() {
	initTracer()

	http.HandleFunc("/cep", handleCEP)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func isNumero(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func initTracer() {
	ctx := context.Background()
	client := otlptracehttp.NewClient(otlptracehttp.WithEndpoint("otel-collector:4317"), otlptracehttp.WithInsecure())
	exporter, err := otlptrace.New(ctx, client)

	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	zipkinExporter, err := zipkin.New("http://zipkin:9411/api/v2/spans")
	if err != nil {
		log.Fatalf("failed to create zipkin exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithBatcher(zipkinExporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("servico-a"),
		)),
	)
	otel.SetTracerProvider(tp)
}

func handleCEP(w http.ResponseWriter, r *http.Request) {
	var req CEPRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "error decoding request", http.StatusBadRequest)
		return
	}

	log.Printf("Received CEP: %s", req.CEP)

	if len(req.CEP) != 8 || !isNumero(req.CEP) {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	ctx := r.Context()
	tracer := otel.Tracer("servico-a")
	ctx, span := tracer.Start(ctx, "handleCEP")
	defer span.End()

	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	serviceBUrl := fmt.Sprintf("http://servico-b:8081/clima?cep=%s", req.CEP)

	reqB, _ := http.NewRequestWithContext(ctx, "GET", serviceBUrl, nil)

	resp, err := client.Do(reqB)
	if err != nil {
		http.Error(w, "service B error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "invalid zipcode", resp.StatusCode)
		return
	}

	var climaResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&climaResp)
	if err != nil {
		http.Error(w, "error parsing response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(climaResp)
}
