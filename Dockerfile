FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bridage-server ./cmd/bridage-server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bridage-server /usr/local/bin/bridage-server
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/bridage-server"]
