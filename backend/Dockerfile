FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.19

LABEL org.opencontainers.image.source="https://github.com/kripa-sindhu-007/weekend_project_1"
LABEL org.opencontainers.image.description="Task Queue Backend — Go-based distributed task processing server"
LABEL org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache ca-certificates
COPY --from=builder /server /server
EXPOSE 8080
CMD ["/server"]
