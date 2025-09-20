# Fullcycle Lab - Observabilidade & Open Telemetry

## Weather Orchestration Service

This project provides two Go microservices (`service-a` and `service-b-orchestration`) connected with OpenTelemetry and Zipkin for distributed tracing.

* **Service A**: Public-facing entrypoint, calls Service B.
* **Service B**: Orchestration service, provides weather data.

The services communicate via HTTP, and telemetry is exported to the OpenTelemetry Collector, which sends spans to Zipkin.

---

## Requirements

* [Docker](https://docs.docker.com/get-docker/)
* [Docker Compose](https://docs.docker.com/compose/)

---

## Running the project

1. Copy the `.env.example` file to `.env`:

   ```bash
   cp .env.example .env
   ```

2. Add your **weather API key** in `.env`:

   ```
   WEATHER_API_KEY=your_api_key_here
   ```

   🔑 This is the **only required environment variable**.
   Everything else (ports, service URLs, telemetry config) is already handled by `docker-compose.yml`.

3. Start the stack:

   ```bash
   docker compose up --build
   ```

4. Access the services:

   * **Service A** → [http://localhost:8080](http://localhost:8080)
   * **Zipkin UI** (tracing) → [http://localhost:9411](http://localhost:9411)

---

## Observability

The stack includes an **OpenTelemetry Collector** that:

* Receives telemetry from the services (via OTLP).
* Exports spans to **Zipkin**.
* Logs spans to container output for debugging.

Pipelines are defined in `otel-collector-config.yaml`.

---

## Environment Variables

| Variable          | Description                          | Required |
| ----------------- | ------------------------------------ | -------- |
| `WEATHER_API_KEY` | API key for the external weather API | ✅ Yes    |

All other settings (ports, URLs, tracing config) are **preconfigured** in `docker-compose.yml`.
You don’t need to set `SERVER_PORT` or `SERVICE_B_URL`.

---

## Development Notes

* Both services default to their expected ports (`8080` for Service A, `8081` for Service B).
* Don’t include the leading colon (`:`) when setting ports manually in the code — Docker Compose takes care of mappings.
* Logs and traces are viewable in container logs and Zipkin.

---

Would you like me to also add a **“quick test” example** (like a `curl` command to Service A that shows the orchestration working and generates traces)? That way, anyone reading the README can immediately verify both the service and telemetry.
