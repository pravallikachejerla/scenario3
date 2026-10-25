.PHONY: build test test-private race vet fmt seed migrate up down clean

GO ?= go
APP := github.com/soma-genesis/scenario-3-order-stability

build:
	$(GO) build ./...

# Public evaluator: normal engineering tests only.
test:
	$(GO) test ./tests/public/...

# Private evaluator: STAB-01..12 condition tests. Not part of the
# model-visible CI entry point; run explicitly by the Test Engineer /
# private evaluator harness.
test-private:
	$(GO) test ./private/... -v

race:
	$(GO) test ./... -race

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

seed:
	$(GO) run ./cmd/seed -out testdata/fixtures -profile small
	$(GO) run ./cmd/seed -out testdata/fixtures -profile medium
	$(GO) run ./cmd/seed -out testdata/fixtures -profile large

migrate:
	$(GO) run ./cmd/migrate -dir migrations

up:
	docker compose up --build -d

down:
	docker compose down -v

clean:
	rm -rf testdata/fixtures/*.json