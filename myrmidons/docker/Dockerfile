# Myrmidons Docker Image

# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev make ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source
COPY . .

# Build with security flags
ENV CGO_ENABLED=1
RUN go build -ldflags="-w -s -extldflags '-fno-PIC'" -o myrmidons ./cmd/server

# Runtime stage
FROM gcr.io/distroless/base-debian13:nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/myrmidons /app/myrmidons

WORKDIR /app

# Run as non-root
USER nonroot

EXPOSE 4000

ENTRYPOINT ["/app/myrmidons"]
