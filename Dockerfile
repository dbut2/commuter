FROM golang:1.26-alpine AS builder

WORKDIR /commuter

COPY go/go.mod go/go.sum ./

RUN go mod download

COPY go/ ./

RUN go build -o /app .

FROM alpine AS final

COPY --from=builder /app /app
COPY config.yaml /config.yaml

ENV PORT=8080
EXPOSE ${PORT}
CMD ["/app"]
