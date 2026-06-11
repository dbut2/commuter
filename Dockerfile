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

CMD ["/bin/server"]
