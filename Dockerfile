FROM golang:1.24.1-alpine AS  builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o url-shortener ./cmd/url-shortener

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/url-shortener .
COPY config/prod.yaml ./config/
EXPOSE 8082
CMD ["./url-shortener"]