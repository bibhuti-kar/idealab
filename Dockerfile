FROM nvidia/cuda:12.6.0-devel-ubuntu22.04 AS builder

# Install Go
ENV GO_VERSION=1.22.10
RUN apt-get update -qq && apt-get install -y -qq wget git && \
    wget -q "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -o /operator ./cmd/operator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /preinstall ./cmd/preinstall

FROM nvidia/cuda:12.6.0-base-ubuntu22.04
RUN groupadd -r appuser && useradd -r -g appuser -u 1000 appuser
COPY --from=builder /operator /operator
COPY --from=builder /preinstall /preinstall
COPY deploy/ /deploy/
COPY scripts/ /scripts/
USER appuser
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD ["/operator", "health"]
ENTRYPOINT ["/operator"]
