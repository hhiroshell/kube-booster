# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source
RUN go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o controller cmd/controller/main.go

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/controller .

USER 65532:65532

ENTRYPOINT ["/controller"]
