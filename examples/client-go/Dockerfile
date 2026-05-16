FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /client ./examples/client-go

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /client /app/client
ENTRYPOINT ["/app/client"]
