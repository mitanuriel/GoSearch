# OWASP ZAP Workflow Fix

## Problem
The OWASP ZAP security scan started failing after merging PR #55 (Dependabot Go module updates). The error showed:
```
curl: (7) Failed to connect to localhost port 8080 after 0 ms: Couldn't connect to server
```

## Investigation: Was it the Dependabot Changes?

### What Dependabot Changed (PR #55)
```diff
- golang.org/x/crypto v0.38.0 → v0.46.0 (major jump!)
- golang.org/x/text v0.25.0 → v0.32.0
- golang.org/x/net v0.39.0 → v0.47.0
- golang.org/x/sys v0.33.0 → v0.39.0
```

### Analysis
**Verdict: The Go module updates are probably NOT the root cause.**

**Why:**
1. The app builds successfully with new versions
2. `golang.org/x/crypto` is only used for bcrypt password hashing (not during startup)
3. The error shows the app **never started** - it's not a runtime crash
4. The Go crypto library doesn't have initialization requirements that would prevent startup

**More likely:** The ZAP workflow was already fragile, and this exposed existing issues.

## Root Causes Identified

### 1. Missing Environment Variables
- `WEATHER_API_KEY` was not set (app may fail if weather endpoints are accessed)
- `SCRAPING_ENABLED` was not explicitly disabled

### 2. Poor Application Startup Handling
- Used `go run ./src/backend/. & sleep 20` which:
  - Doesn't capture the process ID
  - Doesn't redirect logs anywhere
  - Has a fixed 20-second wait (may be too short)
  - Doesn't check if app actually started

### 3. No Debugging Information
- No logs were captured to see why the app failed
- No way to troubleshoot startup issues

### 4. Insufficient Retry Logic
- Only 5 retries with 5-second delays
- If app takes longer, scan fails

## Solutions Implemented

### 1. Added Missing Environment Variables
```yaml
echo "WEATHER_API_KEY=test-api-key-not-needed-for-zap-scan" >> $GITHUB_ENV
echo "SCRAPING_ENABLED=0" >> $GITHUB_ENV
```

### 2. Improved Application Startup
```yaml
- name: Run the application
  run: |
    cd src/backend
    nohup go run . > ../../app.log 2>&1 &
    echo $! > ../../app.pid
    cd ../..
    echo "Application started with PID $(cat app.pid)"
    echo "Waiting 30 seconds for application to fully start..."
    sleep 30
```

**Changes:**
- `nohup` keeps process running even if terminal closes
- Redirect stdout and stderr to `app.log`
- Save process ID to `app.pid` for later cleanup
- Increased wait time to 30 seconds for DB/ES initialization

### 3. Added Application Log Checking
```yaml
- name: Check application logs
  if: always()
  run: |
    echo "=== Application Logs ==="
    cat app.log || echo "No logs available"
    echo "=== End of Logs ==="
```

This step runs even if previous steps fail, so we can always see logs.

### 4. Enhanced Health Check
```yaml
- name: Ensure the application is running
  run: |
    if ! curl --retry 10 --retry-connrefused --retry-delay 3 http://localhost:8080; then
      echo "Application failed to start. Checking logs:"
      cat app.log
      exit 1
    fi
    echo "Application is running successfully!"
```

**Changes:**
- Increased retries from 5 to 10
- Reduced delay from 5s to 3s (more responsive)
- Display logs on failure
- Clear success/failure messages

### 5. Proper Cleanup
```yaml
- name: Stop application
  if: always()
  run: |
    if [ -f app.pid ]; then
      PID=$(cat app.pid)
      echo "Stopping application with PID $PID"
      kill $PID || true
      rm app.pid
    fi
```

Ensures the app process is killed after the scan completes or fails.

## Expected Outcome

With these changes:
1. ✅ Application logs will be visible for debugging
2. ✅ All required environment variables are set
3. ✅ App gets proper time to initialize DB and Elasticsearch
4. ✅ Better retry logic with more attempts
5. ✅ Clear failure messages with logs
6. ✅ Clean process management

## Testing

To verify the fix works:
1. Commit and push these changes to `develop`
2. The ZAP workflow will run automatically
3. Check the "Run the application" step logs to see the PID
4. Check the "Check application logs" step to see app startup
5. The "Ensure the application is running" step should now succeed
6. ZAP scan should proceed normally

## Potential Future Issues

If the scan still fails, check:
- Elasticsearch connection (may need more time to start)
- Database schema initialization
- Port 8080 conflicts
- Memory/resource limitations in GitHub Actions
