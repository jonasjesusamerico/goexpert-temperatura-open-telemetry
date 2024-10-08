package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
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

type ClimaResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

func main() {
	initTracer()

	http.HandleFunc("/clima", handleClima)
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func getLocalizacao(ctx context.Context, cep string) (string, error) {
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	url := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get location")
	}

	var data map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return "", err
	}

	location, ok := data["localidade"].(string)
	if !ok {
		return "", fmt.Errorf("localidade not found")
	}

	return location, nil
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func initTracer() {
	ctx := context.Background()
	client := otlptracehttp.NewClient(
		otlptracehttp.WithEndpoint("otel-collector:4317"),
		otlptracehttp.WithInsecure(),
	)

	exporter, err := otlptrace.New(ctx, client)

	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}

	zipkinExporter, err := zipkin.New("http://zipkin:9411/api/v2/spans")

	if err != nil {
		log.Fatalf("failed to create zipkin exporter: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithBatcher(zipkinExporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("servico-b"),
		)),
	)
	otel.SetTracerProvider(tp)
}

func getClima(ctx context.Context, location string) (float64, error) {
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	url := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=e5bd00e528e346ff8a840254213009&q&q=%s", url.QueryEscape(location))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("erro na criação de clima request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("erro na execução da requisição de clima: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("falha ao buscar clima, status code: %d", resp.StatusCode)
	}

	var result struct {
		Current struct {
			TempC float64 `json:"temp_c"`
		} `json:"current"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("error parsing clima response: %v", err)
	}

	return result.Current.TempC, nil
}

func handleClima(w http.ResponseWriter, r *http.Request) {
	cep := r.URL.Query().Get("cep")
	if len(cep) != 8 || !isNumeric(cep) {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	ctx := r.Context()
	tracer := otel.Tracer("servico-b")
	ctx, span := tracer.Start(ctx, "handleClima")
	defer span.End()

	location, err := getLocalizacao(ctx, cep)
	if err != nil {
		log.Printf("Error getting location: %v", err)
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	clima, err := getClima(ctx, location)
	if err != nil {
		log.Printf("Error getting clima: %v", err)
		http.Error(w, "error fetching clima", http.StatusInternalServerError)
		return
	}

	response := ClimaResponse{
		City:  location,
		TempC: clima,
		TempF: clima*1.8 + 32,
		TempK: clima + 273,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
