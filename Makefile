.PHONY: run build test vet tidy fmt docker

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

fmt:
	gofmt -w .

docker:
	docker build -t expense-tracker-api .
