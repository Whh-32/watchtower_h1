.PHONY: build run install clean test deps docker-build docker-up docker-down docker-logs docker-restart

# Build the application
build:
	go build -o watchtower main.go

# Run the application
run:
	go run main.go

# Install dependencies
deps:
	go mod download
	go mod tidy

# Install subfinder (required dependency)
install-subfinder:
	go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest

# Install httpx (optional but recommended)
install-httpx:
	go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest

# Clean build artifacts
clean:
	rm -f watchtower
	rm -f *.db *.db-shm *.db-wal

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...

# Docker commands
docker-build:
	docker-compose build

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f watchtower

docker-restart:
	docker-compose restart watchtower

docker-shell:
	docker-compose exec watchtower sh

# Build and run with Docker
docker-run: docker-build docker-up
	@echo "Watchtower is running!"
	@echo "Access the web interface at: http://localhost:8080"
	@echo "View logs with: make docker-logs"

# Setup everything
setup: deps install-subfinder install-httpx
	@echo "Setup complete! Don't forget to set HACKERONE_TOKEN environment variable"

# Full Docker setup
docker-setup:
	@if [ ! -f .env ]; then \
		echo "Creating .env file from .env.example..."; \
		cp .env.example .env; \
		echo "⚠️  Please edit .env and set your HACKERONE_TOKEN"; \
	fi
	docker-compose build
	@echo "✅ Docker setup complete!"
	@echo "📝 Edit .env file and set HACKERONE_TOKEN"
	@echo "🚀 Then run: make docker-up"
