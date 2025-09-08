package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"service-b/config"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var viaCepURLBase = "https://viacep.com.br"
var weatherAPIURLBase = "http://api.weatherapi.com"

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
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

type app struct {
	cfg *config.Config
}

// initTracerProvider configura o provedor de tracer para enviar traces para o OTLP.
func initTracerProvider() (*sdktrace.TracerProvider, error) {
	// Cria um novo cliente exportador OTLP que se conecta ao OTEL Collector
	ctx := context.Background()
	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar o exporter OTLP: %w", err)
	}

	// Define os atributos do recurso, como o nome do serviço
	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("service-b"),
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
	// Get the ZIP code from the URL path
	cep := r.URL.Path[1:]

	log.Printf("Received request for CEP: %s", cep)

	// Validate ZIP code format
	if !regexp.MustCompile(`^\d{8}$`).MatchString(cep) {
		log.Printf("Invalid zipcode format: %s", cep)
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	// Fetch city from ViaCEP
	viaCepURL := fmt.Sprintf("%s/ws/%s/json/", viaCepURLBase, cep)
	resp, err := http.Get(viaCepURL)
	if err != nil {
		log.Printf("Error fetching from ViaCEP: %v", err)
		http.Error(w, "can not find zipcode", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Check the HTTP status code first
	if resp.StatusCode == http.StatusNotFound {
		log.Printf("CEP not found by ViaCEP: %s", cep)
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	var viaCepData ViaCEPResponse
	if err := json.Unmarshal(body, &viaCepData); err != nil {
		log.Printf("Error unmarshalling ViaCEP response: %v", err)
		http.Error(w, "can not find zipcode", http.StatusInternalServerError)
		return
	}
	if viaCepData.Localidade == "" {
		log.Printf("CEP not found or missing localidade: %s", cep)
		http.Error(w, "can not find zipcode", http.StatusNotFound)
		return
	}

	// Fetch temperature from WeatherAPI
	weatherAPIKey := a.cfg.WeatherAPIKey
	weatherURL := fmt.Sprintf("%s/v1/current.json?key=%s&q=%s", weatherAPIURLBase, weatherAPIKey, viaCepData.Localidade)
	resp, err = http.Get(weatherURL)
	if err != nil {
		log.Printf("Error fetching from WeatherAPI: %v", err)
		http.Error(w, "can not find temperature", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, _ = io.ReadAll(resp.Body)
	var weatherData WeatherAPIResponse
	if err := json.Unmarshal(body, &weatherData); err != nil {
		log.Printf("Error unmarshalling WeatherAPI response: %v", err)
		http.Error(w, "can not find temperature", http.StatusInternalServerError)
		return
	}

	tempC := weatherData.Current.TempC
	tempF := tempC*1.8 + 32
	tempK := tempC + 273.15 // Use 273.15 for more precision

	// Construct and send the final response
	response := TempResponse{
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
	// 1. Load the configuration file at startup
	cfg, err := config.LoadConfig() // or pass it as an argument
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	// 2. Create an instance of the app struct with the loaded config
	application := &app{cfg: cfg}

	// 3. Register the handler method
	http.HandleFunc("/", application.handler)
	// Register a specific handler for favicon.ico requests
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	// Cloud Run provides the PORT env variable
	port := os.Getenv("PORT")
	if port == "" {
		port = cfg.ServerPort // fallback to .env config
	}

	if !strings.HasPrefix(port, ":") {
		port = ":" + port
	}
	fmt.Printf("Server is running on port %s\n", port)
	http.ListenAndServe(port, nil)
}
