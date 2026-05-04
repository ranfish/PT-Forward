.PHONY: build run test vet lint clean reset-password docker frontend

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY  := pt-forward
LDFLAGS := -s -w -X main.version=$(VERSION)

frontend:
	cd web && npm install && npm run build
	rm -rf frontend/dist
	cp -r web/dist frontend/dist

build: frontend
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/pt-forward/

build-api:
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/pt-forward/

run: build
	./$(BINARY)

test:
	go test -v -race -count=1 ./...

vet:
	go vet ./...

lint: vet
	@echo "lint OK"

clean:
	rm -f $(BINARY)
	rm -rf data/ logs/

reset-password: build-api
	./$(BINARY) -reset-password

docker:
	docker build -t pt-forward:$(VERSION) .

golangci-lint:
	golangci-lint run ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .
	goimports -w .
