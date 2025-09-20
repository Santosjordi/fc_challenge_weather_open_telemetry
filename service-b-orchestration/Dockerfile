# Builder stage
FROM golang:latest AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/service-b-orchestration ./cmd/main.go

# Final runtime image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/service-b-orchestration .
RUN chmod +x ./service-b-orchestration
USER appuser
EXPOSE 8081
CMD ["./service-b-orchestration"]
