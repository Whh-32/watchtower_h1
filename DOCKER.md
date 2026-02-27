# Docker Setup Guide

This guide explains how to run Watchtower using Docker.

## Quick Start

1. **Set up environment:**
   ```bash
   cp .env.example .env
   # Edit .env and set your HACKERONE_TOKEN
   ```

2. **Build and start:**
   ```bash
   docker-compose up -d
   ```

3. **Access the web interface:**
   Open http://localhost:8080 in your browser

## Docker Compose Commands

```bash
# Build the image
docker-compose build

# Start in background
docker-compose up -d

# View logs
docker-compose logs -f watchtower

# Stop
docker-compose down

# Restart
docker-compose restart watchtower

# Access container shell
docker-compose exec watchtower sh
```

## Environment Variables

Set these in `.env` file or as environment variables:

- `HACKERONE_TOKEN` - **Required**: Your HackerOne API token
- `DATABASE_PATH` - Database file path (default: `/app/data/watchtower.db`)
- `WEB_PORT` - Web server port (default: `8080`)
- `HEALTH_CHECK_TIMEOUT` - Health check timeout (default: `10s`)
- `HEALTH_CHECK_WORKERS` - Concurrent workers (default: `50`)
- `SCAN_INTERVAL` - Scan interval (default: `24h`)

## Data Persistence

The database is stored in `./data/` directory which is mounted as a volume. This ensures your data persists even if you remove the container.

## Dependencies Installation

The Docker container automatically installs:
- **subfinder** - For subdomain discovery
- **httpx** - For domain enrichment

These are installed when the container starts via the entrypoint script.

## Troubleshooting

### Container won't start

1. Check logs:
   ```bash
   docker-compose logs watchtower
   ```

2. Verify HackerOne token is set:
   ```bash
   docker-compose exec watchtower env | grep HACKERONE_TOKEN
   ```

### Dependencies not found

The entrypoint script installs dependencies on startup. Check logs to see if installation succeeded:
```bash
docker-compose logs watchtower | grep -i "installing\|subfinder\|httpx"
```

### Database issues

If you need to reset the database:
```bash
docker-compose down
rm -rf data/
docker-compose up -d
```

### Port already in use

Change the port in `docker-compose.yml`:
```yaml
ports:
  - "8081:8080"  # Use port 8081 instead
```

## Building from Source

To rebuild the image:
```bash
docker-compose build --no-cache
```

## Production Deployment

For production, consider:
1. Using environment variables instead of `.env` file
2. Setting up proper secrets management
3. Using a reverse proxy (nginx, traefik)
4. Setting up monitoring and logging
5. Using Docker secrets for sensitive data
