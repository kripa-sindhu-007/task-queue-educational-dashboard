FROM golang:1.22-alpine AS builder

WORKDIR /app

# Cache the dependency layer: only re-downloads when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server && \
    CGO_ENABLED=0 go build -o /worker ./cmd/worker

FROM alpine:3.19

LABEL org.opencontainers.image.source="https://github.com/kripa-sindhu-007/weekend_project_1"
LABEL org.opencontainers.image.description="Task Queue Backend — Go-based distributed task processing server + worker"
LABEL org.opencontainers.image.licenses="MIT"

RUN apk add --no-cache ca-certificates
# Both binaries ship in one image; docker-compose picks which to run per service.
COPY --from=builder /server /server
COPY --from=builder /worker /worker
EXPOSE 8080
CMD ["/server"]

