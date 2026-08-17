.PHONY: build run test vet lint clean reset-password docker docker-multiarch frontend golangci-lint tidy fmt dict dict-check

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

lint:
	golangci-lint run ./...

check-gorm:
	@echo "Checking GORM error handling..."
	@! grep -rn '\.Find(' --include='*.go' internal/api/ internal/publish/ internal/rss/ internal/seeding/ internal/reseed/ | grep -v '_test.go' | grep -v '\.Error' | grep -v 'func ' | grep -v '//' || (echo "ERROR: Found .Find() calls without .Error check (see docs/33-编码规范.md §4.3)" && false)
	@echo "GORM check passed"

golangci-lint: lint

tidy:
	go mod tidy

fmt:
	gofmt -w .
	goimports -w .

docker:
	docker build --build-arg VERSION=$(VERSION) -t pt-forward:$(VERSION) -t pt-forward:latest .

docker-multiarch:
	docker buildx build --platform linux/amd64,linux/arm64 --build-arg VERSION=$(VERSION) -t pt-forward:$(VERSION) -t pt-forward:latest .

# §59.35 P3: 前端字典 codegen（生成物进 git，CI drift 检查）
GO ?= $(shell command -v go 2>/dev/null || echo /home/incast/.local/go/bin/go)

dict:
	$(GO) run ./cmd/gen-dict

dict-check: dict
	@git diff --exit-code web/src/generated/dict.ts || (echo "ERROR: web/src/generated/dict.ts 与 dict/*.json 不同步，运行 make dict 并提交" && exit 1)
	@echo "dict.ts in sync"
