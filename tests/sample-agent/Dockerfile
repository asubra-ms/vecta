# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go build -o agent main.go

# Stage 2: Final lightweight image
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/agent .
CMD ["./agent"]

