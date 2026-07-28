FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN go build -o ws-server .

FROM alpine:latest
WORKDIR /
COPY --from=build /app/ws-server /ws-server
EXPOSE 8082
CMD ["/ws-server"]
