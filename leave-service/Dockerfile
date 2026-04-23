# Stage 1: Build
FROM golang:1.21 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o leave-service .

# Stage 2: Minimal runtime image
FROM scratch

COPY --from=builder /app/leave-service /leave-service

EXPOSE 9090

CMD ["/leave-service"]
