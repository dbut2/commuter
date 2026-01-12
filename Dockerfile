FROM golang:1.25-alpine AS builder

WORKDIR commuter

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY src src

RUN go build -o /app ./src

FROM golang:alpine AS final

COPY --from=builder /app /app

ENV PORT=8080
EXPOSE ${PORT}
CMD ["/app"]
