package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

type ZipCode struct {
	CEP string `json:"cep"`
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
			semconv.ServiceName("service-a-input"),
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

// isValidZipCode valida se o CEP tem 8 dígitos
func isValidZipCode(cep string) bool {
	re := regexp.MustCompile(`^[0-9]{8}$`)
	return re.MatchString(cep)
}

// handler principal
func handler(w http.ResponseWriter, r *http.Request) {
	// Cria um span que engloba a operação completa
	ctx, span := otel.Tracer("service-a").Start(r.Context(), "service-a-handler")
	defer span.End()

	if r.Method != "POST" {
		http.Error(w, "apenas POST é permitido", http.StatusMethodNotAllowed)
		return
	}

	var zipCode ZipCode
	if err := json.NewDecoder(r.Body).Decode(&zipCode); err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("error.message", "falha ao decodificar o body da requisição"))
		http.Error(w, "falha ao decodificar o body da requisição", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if !isValidZipCode(zipCode.CEP) {
		span.SetAttributes(attribute.String("validation.status", "failed"))
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	span.SetAttributes(attribute.String("validation.status", "success"))

	// Chama o Serviço B, propagando o contexto do trace
	serviceBURL := os.Getenv("SERVICE_B_URL")
	if serviceBURL == "" {
		serviceBURL = "http://localhost:8081"
	}

	// Cria um span para a chamada HTTP para o Serviço B
	_, callSpan := otel.Tracer("service-a").Start(ctx, "call-service-b")

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/%s", serviceBURL, zipCode.CEP), nil)
	if err != nil {
		callSpan.RecordError(err)
		callSpan.End()
		http.Error(w, "falha ao criar a requisição para o Serviço B", http.StatusInternalServerError)
		return
	}

	// O otelhttp.Transport lida com a injeção do contexto de trace nos headers
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}
	resp, err := client.Do(req)
	if err != nil {
		callSpan.RecordError(err)
		callSpan.End()
		http.Error(w, "falha ao chamar o Serviço B", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	callSpan.End()

	// Lê o corpo da resposta
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		http.Error(w, "falha ao ler a resposta do Serviço B", http.StatusInternalServerError)
		return
	}

	// Copia a resposta do Serviço B para o cliente
	w.WriteHeader(resp.StatusCode)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(body); err != nil {
		log.Printf("falha ao escrever a resposta: %v", err)
	}
}

func main() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Configura o provedor de trace e se certifica de que ele seja desligado corretamente
	tp, err := initTracerProvider()
	if err != nil {
		log.Fatalf("falha ao configurar o TracerProvider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatalf("falha ao desligar o TracerProvider: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/zipcode", handler)

	// O otelhttp.NewHandler lida com a criação de spans para as requisições HTTP de entrada
	handler := otelhttp.NewHandler(mux, "service-a-input")

	log.Println("Serviço A está rodando na porta :8080...")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("falha ao rodar o servidor: %v", err)
	}

	select {
	case <-sigCh:
		log.Println("Shutting down gracefully...")
	case <-ctx.Done():
		log.Println("Shutting down due to other reason...")
	}

	// Timeout context for gracefull shutdown
	_, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
}
