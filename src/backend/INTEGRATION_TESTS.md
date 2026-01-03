# Integration Tests

This directory contains integration tests for the GoSearch application. Integration tests verify that different components work together correctly with real infrastructure (database, Elasticsearch, etc.).

## Test Categories

### 1. Main Server Integration Tests (`integration_main_test.go`)
Tests for:
- Database connection (`connectDB`, `initDB`)
- Password reset table setup
- Health endpoint
- Metrics endpoint
- Static file serving
- Environment variables

### 2. Elasticsearch Integration Tests (`integration_elasticsearch_test.go`)
Tests for:
- Elasticsearch initialization
- Elasticsearch connection
- Index existence checks
- Page synchronization to Elasticsearch

### 3. Cron & Backup Integration Tests (`integration_cron_test.go`)
Tests for:
- Table checking functionality
- Connection string parsing
- Database backup creation
- Backup file verification
- Old backup cleanup
- Cron job wrappers

## Running Integration Tests

### Prerequisites

Integration tests require:
1. **PostgreSQL database** running and accessible
2. **Elasticsearch** running (for ES tests)
3. **Environment variables** properly set
4. **pg_dump** utility installed (for backup tests)

### Required Environment Variables

```bash
# Database connection
export CONN_STR="postgres://user:password@localhost:5432/gosearch?sslmode=disable"

# Session secret
export SESSION_SECRET="your-secret-key-here"

# Elasticsearch (optional, for ES tests)
export ES_HOST="localhost"
export ES_USERNAME="elastic"
export ES_PASSWORD="changeme"

# Server URL (optional, for endpoint tests)
export SERVER_URL="http://localhost:8080"

# Backup path (optional, for backup tests)
export BACKUP_PATH="/tmp/gosearch_backups"
```

### Running All Integration Tests

```bash
# Run all integration tests
go test -tags=integration -v ./src/backend/...

# Run with timeout
go test -tags=integration -v -timeout 30m ./src/backend/...
```

### Running Specific Integration Test Files

```bash
# Run only main integration tests
go test -tags=integration -v ./src/backend/ -run TestConnectDB

# Run only Elasticsearch tests
go test -tags=integration -v ./src/backend/ -run TestInitElasticsearch

# Run only cron/backup tests
go test -tags=integration -v ./src/backend/ -run TestCheckTables
```

### Running Integration Tests with Docker

If you're using Docker Compose for your infrastructure:

```bash
# Start infrastructure
docker-compose up -d postgres elasticsearch

# Wait for services to be ready
sleep 10

# Set environment variables from docker-compose
export CONN_STR="postgres://postgres:postgres@localhost:5432/gosearch?sslmode=disable"
export ES_HOST="localhost"
export ES_USERNAME="elastic"
export ES_PASSWORD="changeme"

# Run integration tests
go test -tags=integration -v ./src/backend/...

# Clean up
docker-compose down
```

## Excluding Integration Tests from Normal Test Runs

Integration tests are automatically excluded from normal test runs because they use the build tag `// +build integration`.

When you run:
```bash
go test ./src/backend/...
```

Integration tests will NOT be executed. Only unit tests will run.

## Coverage with Integration Tests

To include integration tests in coverage reports:

```bash
# Run both unit and integration tests with coverage
go test -tags=integration -coverprofile=coverage_integration.out ./src/backend/...

# View coverage report
go tool cover -html=coverage_integration.out
```

## Test Behavior

### Skipping Tests

Integration tests will automatically skip if:
- Required environment variables are not set
- Required infrastructure (database, Elasticsearch) is not available
- Running in short mode (`-short` flag)

Example:
```bash
# Skip integration tests even with tags
go test -tags=integration -short ./src/backend/...
```

### Test Output

Integration tests provide detailed logging about:
- Connection attempts
- Infrastructure availability
- Test operations performed
- Reasons for skipping tests

## Continuous Integration

For CI/CD pipelines, you can:

1. **Run only unit tests** (default):
```bash
go test ./src/backend/...
```

2. **Run unit + integration tests** (requires infrastructure):
```bash
# Setup infrastructure (e.g., via docker-compose or cloud services)
# Set environment variables
# Run tests
go test -tags=integration -v ./src/backend/...
```

3. **Separate test stages**:
```yaml
# Example GitHub Actions
test-unit:
  runs-on: ubuntu-latest
  steps:
    - run: go test ./src/backend/...

test-integration:
  runs-on: ubuntu-latest
  services:
    postgres: ...
    elasticsearch: ...
  steps:
    - run: go test -tags=integration ./src/backend/...
```

## Troubleshooting

### "CONN_STR not set, skipping test"
Set the `CONN_STR` environment variable with your database connection string.

### "Database not available"
Ensure PostgreSQL is running and accessible at the connection string specified.

### "Elasticsearch not available"
Ensure Elasticsearch is running and the `ES_HOST`, `ES_USERNAME`, and `ES_PASSWORD` environment variables are set correctly.

### "pg_dump not found"
Install PostgreSQL client tools:
```bash
# Ubuntu/Debian
sudo apt-get install postgresql-client

# macOS
brew install postgresql
```

### "Server not running"
For endpoint tests, ensure the GoSearch server is running at the URL specified in `SERVER_URL` (default: http://localhost:8080).

## Best Practices

1. **Run unit tests frequently** during development
2. **Run integration tests** before commits/PRs
3. **Use Docker Compose** for consistent test infrastructure
4. **Clean up** test data after integration tests
5. **Mock external services** in unit tests, test real integrations in integration tests
6. **Set appropriate timeouts** for integration tests (they take longer than unit tests)

## Contributing

When adding new infrastructure code:
1. Write unit tests with mocks (if possible)
2. Write integration tests to verify real integration
3. Use the `// +build integration` tag
4. Check for required environment variables
5. Skip gracefully if infrastructure is unavailable
6. Document any new environment variables needed
