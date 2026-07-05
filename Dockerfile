FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/server /app/server

ENV GIN_MODE=release
ENV PORT=7860
ENV DATA_DIR=/data

EXPOSE 7860

CMD ["/app/server"]
