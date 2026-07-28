FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go build -o worker .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/worker /app/worker
CMD ["./worker"]
