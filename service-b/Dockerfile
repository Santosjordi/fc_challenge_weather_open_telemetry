# Use a lightweight Go image as the build stage
FROM golang:latest AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files to enable dependency caching
COPY go.mod go.sum ./

# Download the dependencies
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application, making it a static binary
# -o fc-weather-app specifies the output binary name
# ./cmd/main.go points to the main entry file
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o fc-weather-app ./cmd/main.go

# ---
# Use a minimal base image for the final, smaller production image
FROM alpine:latest

# Set the working directory
WORKDIR /root/

# Copy the built binary from the builder stage
# fc-weather-app from the builder is copied to the final image
COPY --from=builder /app/fc-weather-app .

# Expose the port the app will listen on
EXPOSE 8080

# Command to run the application
# The binary is executed when the container starts
CMD ["./fc-weather-app"]