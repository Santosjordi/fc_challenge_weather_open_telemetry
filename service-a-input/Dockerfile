# Stage: builder
FROM golang:latest AS builder
WORKDIR /app

# Copy modules first for caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build static binary
COPY . .
# Use static build (CGO disabled) and target linux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/service-a ./cmd/main.go

# Final runtime image (small)
FROM alpine:latest
# add ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app
# copy binary from 'builder' stage
COPY --from=builder /app/service-a .

# ensure binary is executable (usually preserved)
RUN chmod +x /app/service-a

USER appuser
EXPOSE 8080
CMD ["./service-a"]
