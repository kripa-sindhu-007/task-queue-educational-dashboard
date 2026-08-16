.PHONY: build test test-integration bench vet fmt tidy run loadgen chaos

build:
	go build ./...

# Unit tests (miniredis-backed; no external services required).
test:
	go test ./... -count=1

# Integration tests (testcontainers-backed real Redis). Added in Phase 1 (P1.9);
# gated behind the `integration` build tag so `make test` stays fast and hermetic.
test-integration:
	go test ./... -tags=integration -count=1

bench:
	go test ./... -run=^$$ -bench=. -benchmem

vet:
	go vet ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

run:
	go run ./cmd/server

# Load generator (P3.5). Pass flags via ARGS, e.g.:
#   make loadgen ARGS="-ramp 100:2000 -duration 90s"
loadgen:
	go run ./cmd/loadgen $(ARGS)

# Chaos harness (P4.5). Random worker + scheduler kills under load with a cron
# job running, then asserts zero-loss + cron integrity from Redis ground truth.
# Config via env, e.g.:  make chaos CHAOS_DURATION=10m CHAOS_INTERVAL=15
chaos:
	cd .. && ./scripts/chaos.sh
