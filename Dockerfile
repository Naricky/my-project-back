FROM golang:1.13.7-alpine3.11 as builder
WORKDIR /app
COPY . .
RUN go build -o rl main.go

FROM alpine:3.11
RUN apk add --update bash ca-certificates
WORKDIR /app
COPY --from=builder /app/rl .

ENTRYPOINT ["/app/rl"]
