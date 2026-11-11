OAPI_CODEGEN := github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1

.PHONY: generate build run test docker-up docker-down

generate:
	go run $(OAPI_CODEGEN) -config api/oapi-codegen.yaml api/openapi.yaml

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

docker-up:
	docker compose up --build

docker-down:
	docker compose down