BINARY  := freshMango
PKG     := ./...
COVER   := coverage.out

.DEFAULT_GOAL := help

## Build / Run ----------------------------------------------------------------

.PHONY: build
build: ## Compile the binary
	go build -o $(BINARY) .

.PHONY: install
install: ## Install the binary to $GOPATH/bin
	go install .

## Quality ---------------------------------------------------------------------

.PHONY: test
test: ## Run all tests
	go test $(PKG)

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	go test -coverprofile=$(COVER) $(PKG)
	go tool cover -func=$(COVER)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run $(PKG)

.PHONY: vet
vet: ## Run go vet
	go vet $(PKG)

.PHONY: check
check: vet lint test ## Run vet + lint + tests (full quality gate)

## Housekeeping ----------------------------------------------------------------

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY) $(COVER)

## Help ------------------------------------------------------------------------

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
