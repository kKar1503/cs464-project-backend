# Build stage
FROM golang:1.25.7-alpine AS builder

WORKDIR /app

COPY shared ./shared
COPY db ./db
COPY services/deck/go.mod services/deck/go.sum* ./services/deck/

WORKDIR /app/services/deck
RUN go mod download

COPY services/deck/. ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/deck .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/bin/deck .

EXPOSE 8005

CMD ["./deck"]