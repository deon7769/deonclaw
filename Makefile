.PHONY: build test fmt ship

build:
	go build -o bin/deonctl ./cmd/deonctl

test:
	go test ./...

fmt:
	gofmt -w .

ship:
	@if [ -n "$(MSG)" ]; then \
		bash scripts/validated-commit-push.sh -m "$(MSG)"; \
	else \
		bash scripts/validated-commit-push.sh; \
	fi
