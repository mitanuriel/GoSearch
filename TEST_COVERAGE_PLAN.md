# Test Coverage Improvement Plan

**Current Coverage:** 52.8%  
**Target:** 80% (to pass SonarCloud quality gate)  
**Gap:** Need ~27% more coverage

## Priority 1: Quick Wins (High Impact, Easy to Test)

### 1. Root Handlers (88.9% → 100%)
- **File:** `root.go`
- **Functions:** `rootHandler`, `aboutHandler`
- **Missing:** Error handling paths for template execution
- **Effort:** Low
- **Impact:** +0.5%

### 2. Session Metrics (83.3% → 100%)
- **File:** `session_metrics.go`
- **Function:** `getAuthStatus`
- **Missing:** Error path when store.Get fails
- **Effort:** Very Low
- **Impact:** +0.3%

### 3. Search Handler (95.2% → 100%)
- **File:** `search.go`
- **Function:** `searchHandler`
- **Missing:** Template execution error path
- **Effort:** Low
- **Impact:** +0.4%

### 4. Weather Functions (84-86% → 100%)
- **File:** `weather.go`
- **Functions:** `weatherHandler`, `fetchWeatherData`
- **Missing:** Error paths and edge cases
- **Effort:** Low
- **Impact:** +1.5%

## Priority 2: Medium Impact (Partial Coverage)

### 5. User Handlers (64-85% coverage)
- **File:** `user.go`
- **Functions:**
  - `apiLogin` (64.9%)
  - `registerHandler` (70.0%)
  - `apiRegisterHandler` (80.8%)
  - `logoutHandler` (81.8%)
  - `login` (85.7%)
- **Missing:** Various error paths, validation failures
- **Effort:** Medium
- **Impact:** +5-8%

### 6. Password Reset Functions (66-82% coverage)
- **File:** `password_reset.go`
- **Functions:**
  - `renderResetPasswordError` (66.7%)
  - `checkPasswordResetRequired` (68.2%)
  - `setupPasswordResetTable` (70.3%)
  - `passwordResetMiddleware` (72.2%)
  - `apiResetPasswordHandler` (79.5%)
  - `verifySetup` (80.0%)
  - `resetPasswordHandler` (82.1%)
- **Missing:** Error paths, edge cases, validation failures
- **Effort:** Medium-High
- **Impact:** +6-10%

### 7. Search Elasticsearch (53.6% → 80%+)
- **File:** `search.go`
- **Function:** `searchPagesInEs`
- **Missing:** Elasticsearch error paths, JSON decode errors
- **Effort:** Medium
- **Impact:** +2-3%

### 8. Scraping Functions (25-94% coverage)
- **File:** `scraping.go`
- **Functions:**
  - `tryScrapeInLanguages` (25.0%)
  - `StartScraping` (47.1%)
  - `scrapeWikipedia` (94.4%)
- **Missing:** Error handling, language fallbacks, HTTP failures
- **Effort:** High (requires mocking HTTP requests)
- **Impact:** +3-5%

## Priority 3: Infrastructure (Currently 0% - Skip for Now)

These are hard to test and low ROI:
- `main.go` - main() function (0%)
- `databaseConfig.go` - initialization functions (0%)
- `elasticsearch.go` - initElasticsearch (0%)
- `prometheus.go` - monitoring functions (0%)
- Database backup functions (0%)
- Cron scheduler (0%)

**Note:** These are typically excluded from coverage requirements as they're integration/startup code.

## Recommended Approach

### Phase 1: Quick Wins (Target: +3-5% coverage)
1. Complete root handlers tests
2. Complete session metrics tests
3. Complete search handler tests
4. Complete weather tests

**Estimated effort:** 2-3 hours  
**Expected coverage:** ~56-58%

### Phase 2: User & Auth (Target: +8-12% coverage)
1. Improve user handler tests
2. Improve password reset tests
3. Add missing error path tests

**Estimated effort:** 4-6 hours  
**Expected coverage:** ~64-70%

### Phase 3: Advanced Functions (Target: +10-15% coverage)
1. Improve search Elasticsearch tests
2. Improve scraping tests with HTTP mocking

**Estimated effort:** 6-8 hours  
**Expected coverage:** ~74-85%

## Files to Create/Modify

- `src/backend/root_test.go` - Enhance existing tests
- `src/backend/session_metrics_test.go` - Enhance existing tests
- `src/backend/search_test.go` - Enhance existing tests
- `src/backend/weather_test.go` - Enhance existing tests
- `src/backend/user_test.go` - Add missing test cases
- `src/backend/password_reset_test.go` - Add missing test cases
- `src/backend/scraping_test.go` - Add comprehensive mocking

## Success Metrics

- **Minimum Target:** 60% coverage (Phase 1 + Phase 2 partial)
- **Goal Target:** 80% coverage (All phases)
- **SonarCloud:** Pass quality gate
- **CI/CD:** All tests pass with increased coverage
