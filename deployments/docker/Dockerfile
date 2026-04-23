FROM golang:1.24-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/op-bot .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/op-bot /app/op-bot
COPY schema /app/schema
COPY docs /app/docs

EXPOSE 8080
ENTRYPOINT ["/app/op-bot"]
