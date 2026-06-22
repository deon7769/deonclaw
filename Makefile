.PHONY: build test fmt ship provider-call-chain-smoke proactive-runtime-smoke

build:
	go build -o bin/deonctl ./cmd/deonctl

test:
	go test ./...

fmt:
	gofmt -w .

provider-call-chain-smoke:
	bash scripts/provider-call-chain-fixture-smoke.sh

proactive-runtime-smoke:
	bash scripts/proactive-runtime-fixture-smoke.sh

ship:
	@if [ -n "$(MSG)" ]; then \
		bash scripts/validated-commit-push.sh -m "$(MSG)"; \
	else \
		bash scripts/validated-commit-push.sh; \
	fi
