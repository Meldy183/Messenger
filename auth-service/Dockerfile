FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service ./auth-service/cmd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /auth-service /auth-service
ENTRYPOINT ["/auth-service"]
