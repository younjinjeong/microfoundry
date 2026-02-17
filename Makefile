BINARY_NAME := mf
BUILD_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build test lint clean tidy fmt e2e-install e2e e2e-headed e2e-ui e2e-report

all: tidy fmt build test

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/mf

test:
	go test ./... -v -count=1

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

tidy:
	go mod tidy

clean:
	rm -rf $(BUILD_DIR)

docker-build:
	docker build -t microfoundry/mf:$(VERSION) .

docker-run:
	docker run --rm -it microfoundry/mf:$(VERSION)

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

monitoring-install:
	bash deploy/monitoring/install.sh

# E2E testing with Playwright
e2e-install:
	cd test && npm install && npx playwright install chromium

e2e: build
	cd test && npx playwright test

e2e-headed: build
	cd test && npx playwright test --headed

e2e-ui: build
	cd test && npx playwright test --ui

e2e-report:
	cd test && npx playwright show-report
