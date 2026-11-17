FROM golang:1.25-bookworm AS build

ENV TZ=Asia/Jakarta

# Ensure IPv4 precedence to avoid "network is unreachable" on environments without IPv6 routing
RUN sed -i 's/^#precedence ::ffff:0:0\/96  100/precedence ::ffff:0:0\/96  100/' /etc/gai.conf 2>/dev/null || echo "precedence ::ffff:0:0/96 100" >> /etc/gai.conf

ENV GOPROXY=https://proxy.golang.org,https://goproxy.io,direct

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/api

FROM alpine:3.20

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=build /server /usr/local/bin/server
RUN chown appuser:appgroup /usr/local/bin/server

USER appuser

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/health || exit 1

EXPOSE 8080

ENTRYPOINT ["server"]
