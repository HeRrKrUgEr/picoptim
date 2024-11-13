# First stage: Build the Go application
FROM golang:1.23-alpine AS builder

# Install build dependencies (e.g., gcc, musl-dev)
RUN apk add --no-cache build-base

# Set the working directory
WORKDIR /app

# Copy Go modules manifest and download dependencies
COPY go.mod go.sum ./
ENV GOPROXY=https://proxy.golang.org
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the Go application with CGO_ENABLED=1
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o main .

# Use a minimal image for production
FROM alpine:latest
RUN apk --no-cache add ca-certificates imagemagick libheif


# Copy the executable from the builder stage
COPY --from=builder . .

WORKDIR /app

# Expose port 8080 and run the application
EXPOSE 8080
CMD ["./main"]