# Watchtower - Bug Bounty Asset Discovery & Monitoring

Watchtower is a high-performance Go application for monitoring HackerOne bug bounty programs, discovering domains, checking their health status, and tracking new assets over time.

## Features

- 🔍 **HackerOne Integration**: Automatically fetches all available bug bounty programs
- 🌐 **Domain Discovery**: Uses subfinder for comprehensive subdomain enumeration
- ✅ **Health Checking**: Verifies if domains are up or down with concurrent workers
- 💾 **Database Storage**: SQLite database for persistent storage
- 📊 **Web Dashboard**: Beautiful web interface to view results, stats, and new domains
- ⏰ **Daily Automation**: Runs scans automatically every 24 hours
- 🚀 **High Performance**: Concurrent processing with worker pools
- 🔔 **Status Change Tracking**: Alerts when domains change from DOWN to UP
- 🎯 **RDP/VDP Filters**: Filter programs by type (Remote Disclosure / Vulnerability Disclosure)
- 💰 **Bounty Filters**: Filter programs that offer bounties
- 🔬 **ProjectDiscovery Integration**: Uses httpx for enhanced domain information (title, technologies, status codes)
- 📈 **Real-time Updates**: Auto-refreshing web interface with live results

## Prerequisites

1. **Go 1.21+** installed
2. **Subfinder** installed and configured:
   ```bash
   go install -v github.com/projectdiscovery/subfinder/v2/cmd/subfinder@latest
   ```
3. **httpx** (optional but recommended) for enhanced domain information:
   ```bash
   go install -v github.com/projectdiscovery/httpx/cmd/httpx@latest
   ```
4. **HackerOne API Token**: Get your token from [HackerOne Settings](https://hackerone.com/settings/api_token/edit)

## Installation

### Option 1: Docker (Recommended) 🐳

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd watchtower
   ```

2. Set up environment:
   ```bash
   cp .env.example .env
   # Edit .env and set your HACKERONE_TOKEN
   ```

3. Build and run with Docker:
   ```bash
   make docker-setup    # First time setup
   make docker-up       # Start the application
   ```

   Or manually:
   ```bash
   docker-compose build
   docker-compose up -d
   ```

4. Access the web interface at http://localhost:8080

5. View logs:
   ```bash
   make docker-logs
   # or
   docker-compose logs -f watchtower
   ```

**Note**: Dependencies (subfinder, httpx) are automatically installed when the container starts!

### Option 2: Local Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd watchtower
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Set up your HackerOne API token:
   ```bash
   export HACKERONE_TOKEN="your-token-here"
   # OR create a file:
   echo "your-token-here" > .hackerone_token
   ```

4. Install required tools:
   ```bash
   make install-subfinder
   make install-httpx  # Optional but recommended
   ```

5. (Optional) Configure subfinder with API keys for better results:
   ```bash
   subfinder -pc
   # Follow the prompts to add API keys (Shodan, VirusTotal, etc.)
   ```

## Configuration

Environment variables (all optional):

- `HACKERONE_TOKEN`: Your HackerOne API token (required)
- `DATABASE_PATH`: Path to SQLite database (default: `./watchtower.db`)
- `WEB_PORT`: Web server port (default: `8080`)
- `HEALTH_CHECK_TIMEOUT`: Timeout for health checks (default: `10s`)
- `HEALTH_CHECK_WORKERS`: Number of concurrent health check workers (default: `50`)
- `SCAN_INTERVAL`: Interval between scans (default: `24h`)

## Usage

### With Docker:

```bash
# Start the application
make docker-up
# or
docker-compose up -d

# View logs
make docker-logs
# or
docker-compose logs -f watchtower

# Stop the application
make docker-down
# or
docker-compose down

# Restart
make docker-restart
# or
docker-compose restart watchtower
```

### Local Run:

```bash
go run main.go
# or
make run
```

The application will:
1. Run an initial scan immediately
2. Start the web server on port 8080 (or your configured port)
3. Schedule daily scans automatically

**Note**: With Docker, dependencies (subfinder, httpx) are automatically installed on container start.

### Access the Web Interface:

Open your browser and navigate to:
- **Dashboard**: http://localhost:8080
- **Domains**: http://localhost:8080/domains
- **Programs**: http://localhost:8080/programs
- **Status Changes**: http://localhost:8080/status-changes (shows when domains go from DOWN to UP)
- **Filters**: http://localhost:8080/filters (RDP/VDP/Bounty filters)

### API Endpoints:

- `GET /api/v1/stats` - Get statistics
- `GET /api/v1/domains/new?limit=100` - Get new domains
- `GET /api/v1/domains?program=handle&limit=100` - Get domains by program
- `GET /api/v1/programs` - Get all programs
- `GET /api/v1/programs/rdp` - Get RDP (Remote Disclosure) programs
- `GET /api/v1/programs/vdp` - Get VDP (Vulnerability Disclosure) programs
- `GET /api/v1/programs/bounties` - Get programs offering bounties
- `GET /api/v1/status-changes?limit=50` - Get domain status changes
- `GET /api/v1/status-changes/unnotified?limit=50` - Get unnotified status changes

## Project Structure

```
watchtower/
├── main.go                 # Application entry point
├── go.mod                  # Go dependencies
├── internal/
│   ├── config/            # Configuration management
│   ├── database/          # Database layer (SQLite)
│   ├── hackerone/         # HackerOne API client
│   ├── discovery/         # Domain discovery service
│   ├── healthcheck/       # Health check service
│   ├── scheduler/         # Scan scheduler
│   └── server/            # Web server and API
├── web/
│   ├── templates/         # HTML templates
│   └── static/           # CSS and static assets
└── README.md
```

## How It Works

1. **Program Fetching**: Connects to HackerOne API and fetches all available programs
2. **Scope Extraction**: Gets the scope (domains) for each program
3. **Subdomain Discovery**: Uses subfinder to discover subdomains for each base domain
4. **Health Checking**: Concurrently checks if each domain is up or down
5. **Database Storage**: Saves all discovered domains with their status
6. **New Asset Detection**: Tracks which domains are newly discovered
7. **Web Dashboard**: Provides a UI to view results and statistics

## Performance

- Concurrent health checking with configurable worker pool
- Efficient database operations with indexes
- Rate limiting to respect API limits
- Optimized for handling thousands of domains

## Database Schema

- **programs**: Stores HackerOne program information
- **domains**: Stores discovered domains with status and metadata

## Troubleshooting

### Subfinder not found:
```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### HackerOne API errors:
- Verify your token is correct
- Check API rate limits
- Ensure you have access to the programs

### Health checks timing out:
- Increase `HEALTH_CHECK_TIMEOUT`
- Reduce `HEALTH_CHECK_WORKERS` if you have network issues

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
