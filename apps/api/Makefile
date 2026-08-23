# recruitmate Go API 开发命令
# 依赖：Go 1.27+

.PHONY: dev build test vet lint migrate-up migrate-down seed run clean

dev:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

run: build
	./bin/server

test:
	go test ./...

vet:
	go vet ./...

# golangci-lint 需本地安装：brew install golangci-lint
lint:
	golangci-lint run

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

seed:
	go run ./cmd/seed

clean:
	rm -rf bin
