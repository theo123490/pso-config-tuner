FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /controller ./cmd/controller

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /controller /app/controller
COPY configs/ /app/configs/
EXPOSE 8080
ENTRYPOINT ["/app/controller"]
