.PHONY: fmt
fmt:
	@echo "🧹 Formatting Go code..."
	@gofmt -l -w `find . -type f -name '*.go' -not -path "./vendor/*"`
	@golines --max-len=120 --base-formatter=gofmt --shorten-comments --ignore-generated  --ignored-dirs=vendor -w .
	@echo "✅ Code formatted successfully"

.PHONY: lint
lint:
	golangci-lint run

.PHONY: run_server
run_server:
	go run cmd/main.go

.PHONY: build
build:
	go build -o matching-service .

.PHONY: run_containers
run_containers:
	docker compose -f ./deploy/docker-compose.yml up -d

.PHONY: stop_containers
stop_containers:
	docker compose -f ./deploy/docker-compose.yml down

.PHONY: docs
docs:
	swag init -g ./cmd/main.go -o=./docs --generatedTime=false --parseDependency --parseInternal

.PHONY: check_containers
check_containers:
	docker network inspect zesty-network --format '{{range .Containers}}{{.Name}} - {{.IPv4Address}}{{println}}{{end}}'