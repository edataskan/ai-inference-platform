# Faz 4'te kullanılacak. Şimdiden hazır olsun.
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go build -o server .

FROM alpine:latest
WORKDIR /app
COPY --from=build /app/server /app/server
EXPOSE 8080
CMD ["./server"]
