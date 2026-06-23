# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY hack/grpc-demo-server/ hack/grpc-demo-server/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
    -o grpc-demo-server \
    github.com/hhiroshell/kube-booster/hack/grpc-demo-server

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/grpc-demo-server .

USER 65532:65532

EXPOSE 50051

ENTRYPOINT ["/grpc-demo-server"]
