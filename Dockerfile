FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./db ./db
COPY ./go ./go
COPY ./web ./web

RUN go build -o /bin/server ./go

FROM alpine AS final

WORKDIR /app

COPY --from=builder /bin/server /bin/server

ARG PORT=8080
EXPOSE ${PORT}

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s CMD wget -qO /dev/null "http://127.0.0.1:${HTTP_PORT:-8080}/health" || exit 1

CMD ["/bin/server"]
