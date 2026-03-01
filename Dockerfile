FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /operator ./cmd/operator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /preinstall ./cmd/preinstall

FROM gcr.io/distroless/static-debian12
COPY --from=builder /operator /operator
COPY --from=builder /preinstall /preinstall
COPY deploy/ /deploy/
USER nonroot:nonroot
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD ["/operator", "health"]
ENTRYPOINT ["/operator"]
