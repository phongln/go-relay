PACKAGES := ./relay/... ./middleware/... ./mockrelay/...

GOBIN := $(shell go env GOPATH)/bin
GOLANGCI_LINT := $(GOBIN)/golangci-lint
GOIMPORTS := $(GOBIN)/goimports

.PHONY: fmt lint lint-fix test vet check tools

tools:
	cd tools && go install github.com/golangci/golangci-lint/cmd/golangci-lint
	cd tools && go install golang.org/x/tools/cmd/goimports

fmt: tools
	gofmt -s -w .
	$(GOIMPORTS) -w -local github.com/phongln/go-relay .

lint: tools
	$(GOLANGCI_LINT) run --timeout=5m $(PACKAGES)

lint-fix: tools
	$(GOLANGCI_LINT) run --fix --timeout=5m $(PACKAGES)

vet:
	go vet $(PACKAGES)

test:
	go test -race -count=1 -timeout=60s ./...

check: fmt lint-fix vet test
