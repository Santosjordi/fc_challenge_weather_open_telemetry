# Project TODO List

This file outlines the necessary corrections and missing requirements to fully align the project with the `ASSIGNMENT.md` specifications.

## Service B

### 1. Correct the Success Response Format

-   **File:** `service-b/cmd/main.go`
-   **Issue:** The successful response (`200 OK`) is missing the `city` field.
-   **Requirement:** The response body should be `{ "city": "São Paulo", "temp_C": 28.5, "temp_F": 28.5, "temp_K": 28.5 }`.
-   **Action:** Modify the `TempResponse` struct to include a `City` field and populate it with the `Localidade` from the ViaCEP API response.

### 2. Implement OpenTelemetry Tracing

-   **File:** `service-b/cmd/main.go`
-   **Issue:** The OpenTelemetry tracer provider is defined (`initTracerProvider`) but is never initialized or used. The incoming trace context from Service A is not being propagated, and no new spans are being created.
-   **Requirement:** Implement distributed tracing and create spans for external API calls.
-   **Actions:**
    -   Call `initTracerProvider()` in the `main` function.
    -   Wrap the main HTTP handler with `otelhttp.NewHandler` to automatically handle incoming trace contexts.
    -   Create a new span that wraps the entire handler logic to trace the orchestration process.
    -   Create a child span specifically for the `ViaCEP` API call.
    -   Create another child span for the `WeatherAPI` call.

## Docker

-   **File:** `docker-compose.yml`
-   **Issue:** The `otel-collector` service is defined but is missing the configuration file (`otel-collector-config.yml`) required to run.
-   **Requirement:** The project must be runnable via `docker-compose`.
-   **Action:** Create the `otel-collector-config.yml` file with the necessary configuration to receive OTLP traces and export them to Zipkin.
