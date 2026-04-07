FROM golang:1.22-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/backend ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -o /bin/migrate ./cmd/migrate && \
    CGO_ENABLED=0 GOOS=linux go build -o /bin/seed ./cmd/seed

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app
COPY --from=builder /bin/backend /usr/local/bin/backend
COPY --from=builder /bin/migrate /usr/local/bin/migrate
COPY --from=builder /bin/seed /usr/local/bin/seed
COPY --from=builder /app/migrations /app/migrations
COPY --from=builder /app/seed /app/seed

EXPOSE 8080
CMD ["backend"]
