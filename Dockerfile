# Builder Stage
FROM golang:1.25-alpine AS builder
ENV CGO_ENABLED=0 GOOS=linux
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /app/server ./cmd/main.go

# Production Stage
FROM alpine:latest AS production
WORKDIR /app
EXPOSE 8080
COPY --from=builder /app/server /app/server
ENV ENVIRONMENT=production 
ENV PORT=8080 
ENV ALLOWED_ORIGINS='https://hzclog.com,https://www.hzclog.com'
CMD ["./server"]
