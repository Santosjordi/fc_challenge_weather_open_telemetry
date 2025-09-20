package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"service-b/config"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type ViaCEPResponse struct {
	Localidade string `json:"localidade"`
	Erro       bool   `json:"erro"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type TempResponse struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type app struct {
	cfg *config.Config
}

// initTracerProvider configura o provedor de tracer para enviar traces para o OTLP.
func initTracerProvider() (*sdktrace.TracerProvider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	// Cria um novo cliente exportador OTLP que se conecta ao OTEL Collector
	ctx := context.Background()
	exporter, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(), // needed if collector is not using TLS
	)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar o exporter OTLP: %w", err)
	}

	// Define os atributos do recurso, como o nome do serviço
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("service-b-orchestration"),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar o recurso: %w", err)
	}

	// Cria o TracerProvider com o exportador e o recurso
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Define o TracerProvider global
	otel.SetTracerProvider(tp)
	// Define o propagador de contexto para HTTP
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}

// Function to handle the main logic
func (a *app) handler(w http.ResponseWriter, r *http.Request) {
	// The otelhttp handler wrapper automatically extracts the trace context.
	// Create a new span for the orchestration handler
	tracer, ctx := otel.Tracer("service-b-tracer"), r.Context()
	ctx, span := tracer.Start(ctx, "orchestration-handler")
	defer span.End()

	// Get the ZIP code from the URL path
	cep := r.URL.Path[1:]

	log.Printf("Received request for CEP: %s", cep)

	// Validate ZIP code format
	if !regexp.MustCompile(`^[0-9]{8}$`).MatchString(cep) {
		log.Printf("Invalid zipcode format: %s", cep)
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	// Fetch city from ViaCEP
	viaCepURL := fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep)

	// Create a span for the ViaCEP API call
	ctx, viaCepSpan := tracer.Start(ctx, "call-viacep-api")
	req, _ := http.NewRequestWithContext(ctx, "GET", viaCepURL, nil)
	resp, err := http.DefaultClient.Do(req)
	viaCepSpan.End()

	if err != nil {
		log.Printf("Error fetching from ViaCEP: %v", err)
		http.Error(w, "error fetching from ViaCEP", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var viaCepData ViaCEPResponse
	if err := json.Unmarshal(body, &viaCepData); err != nil {
		log.Printf("Error unmarshalling ViaCEP response: %v", err)
		http.Error(w, "error unmarshalling ViaCEP response", http.StatusInternalServerError)
		return
	}
	if viaCepData.Erro || viaCepData.Localidade == "" {
		log.Printf("CEP not found or missing localidade: %s", cep)
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	// Fetch temperature from WeatherAPI
	// URL encode the city name to handle spaces and special characters
	escapedCity := url.QueryEscape(viaCepData.Localidade)
	weatherAPIKey := a.cfg.WeatherAPIKey
	weatherURL := fmt.Sprintf("http://api.weatherapi.com/v1/current.json?key=%s&q=%s", weatherAPIKey, escapedCity)

	// Create a span for the WeatherAPI call
	ctx, weatherSpan := tracer.Start(ctx, "call-weather-api")
	req, _ = http.NewRequestWithContext(ctx, "GET", weatherURL, nil)
	resp, err = http.DefaultClient.Do(req)
	weatherSpan.End()

	if err != nil {
		log.Printf("Error fetching from WeatherAPI: %v", err)
		http.Error(w, "error fetching from WeatherAPI", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var weatherData WeatherAPIResponse
	if err := json.Unmarshal(body, &weatherData); err != nil {
		log.Printf("Error unmarshalling WeatherAPI response: %v", err)
		http.Error(w, "error unmarshalling WeatherAPI response", http.StatusInternalServerError)
		return
	}

	tempC := weatherData.Current.TempC
	tempF := tempC*1.8 + 32
	tempK := tempC + 273

	// Construct and send the final response
	response := TempResponse{
		City:  viaCepData.Localidade,
		TempC: tempC,
		TempF: tempF,
		TempK: tempK,
	}

	// log the response to the console
	log.Printf("Response for CEP %s: %+v", cep, response)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func main() {
	// Graceful shutdown setup
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Configure and register the OpenTelemetry tracer provider.
	tp, err := initTracerProvider()
	if err != nil {
		log.Fatalf("failed to initialize tracer provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("error shutting down tracer provider: %v", err)
		}
	}()

	// Load the configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	application := &app{cfg: cfg}

	// Use otelhttp.NewHandler to wrap the mux and automatically create spans for incoming requests.
	mux := http.NewServeMux()
	mux.HandleFunc("/", application.handler)
	handler := otelhttp.NewHandler(mux, "service-b-orchestration")

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8081" // Default port
	}
	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}

	// Create server
	srv := &http.Server{
		Addr:    port,
		Handler: handler,
	}

	// Run server in a goroutine
	go func() {
		log.Printf("Service B is running on port %s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not start server: %v", err)
		}
	}()

	// Wait for signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}
}
