# Test Coverage Improvement Plan

**Current Coverage:** 53.8% (Updated: 2026-01-03)  
**Target:** 80% (to pass SonarCloud quality gate)  
**Gap:** Need ~26.2% more coverage

## Progress Log
- 2026-01-03: 52.3% → 53.8% (+1.5%) - Completed scraping.go Phase 3 tests
  - StartScraping: 47.1% → 100%
  - tryScrapeInLanguages: 25.0% → 100%

## Priority 1: Quick Wins (High Impact, Easy to Test)

### 1. Root Handlers (88.9% → 100%)
- **File:** `root.go`
- **Functions:** `rootHandler`, `aboutHandler`
- **Missing:** Error handling paths for template execution
- **Effort:** Low
- **Impact:** +0.5%
- **Status:** TODO

### 2. Session Metrics (83.3% → ~90%)
- **File:** `session_metrics.go`
- **Function:** `getAuthStatus`
- **Missing:** Error path when store.Get fails
- **Effort:** Very Low
- **Impact:** +0.3%
- **Status:** ✅ DONE (added TestGetAuthStatus_StoreGetError)

### 3. Search Handler (95.2% → 100%)
- **File:** `search.go`
- **Function:** `searchHandler`
- **Missing:** Template execution error path
- **Effort:** Low
- **Impact:** +0.4%
- **Status:** TODO

### 4. Weather Functions (84-86% → 100%)
- **File:** `weather.go`
- **Functions:** `weatherHandler`, `fetchWeatherData`
- **Missing:** Error paths and edge cases
- **Effort:** Low
- **Impact:** +1.5%
- **Status:** TODO

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
- **Status:** TODO

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
  - `tryScrapeInLanguages` (25.0% → 100% ✅)
  - `StartScraping` (47.1% → 100% ✅)
  - `scrapeWikipedia` (94.4%)
- **Missing:** None (main functions covered)
- **Effort:** High (requires mocking HTTP requests)
- **Impact:** +3-5%
- **Status:** ✅ DONE (added comprehensive HTTP integration tests)

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
1. Complete root handlers tests - TODO
2. Complete session metrics tests - ✅ DONE
3. Complete search handler tests - TODO
4. Complete weather tests - TODO

**Estimated effort:** 2-3 hours  
**Expected coverage:** ~56-58%  
**Status:** 1/4 complete

### Phase 2: User & Auth (Target: +8-12% coverage)
1. Improve user handler tests - TODO
2. Improve password reset tests - TODO
3. Add missing error path tests - TODO

**Estimated effort:** 4-6 hours  
**Expected coverage:** ~64-70%  
**Status:** Not started

### Phase 3: Advanced Functions (Target: +10-15% coverage)
1. Improve search Elasticsearch tests - TODO
2. Improve scraping tests with HTTP mocking - ✅ DONE

**Estimated effort:** 6-8 hours  
**Expected coverage:** ~74-85%  
**Status:** 1/2 complete

## Next Steps (Priority Order)

1. **Quick Wins Remaining** (Highest ROI):
   - Root handlers (+0.5%)
   - Search handler (+0.4%)
   - Weather functions (+1.5%)
   - **Total potential:** +2.4% → ~56.2% coverage

2. **User Handlers** (Medium ROI):
   - apiLogin, registerHandler, apiRegisterHandler error paths
   - **Total potential:** +5-8% → ~61-64% coverage

3. **Password Reset** (Medium ROI):
   - Multiple functions with partial coverage
   - **Total potential:** +6-10% → ~67-74% coverage

4. **Search Elasticsearch** (Lower ROI):
   - searchElasticsearch function (53.6%)
   - **Total potential:** +2-3% → ~69-77% coverage

## Files to Create/Modify

- `src/backend/root_test.go` - Enhance existing tests
- ~~`src/backend/session_metrics_test.go`~~ - ✅ Enhanced
- `src/backend/search_test.go` - Enhance existing tests
- `src/backend/weather_test.go` - Enhance existing tests
- `src/backend/user_test.go` - Add missing test cases
- ~~`src/backend/scraping_test.go`~~ - ✅ Enhanced with comprehensive tests
- `src/backend/password_reset_test.go` - Add missing test cases
- `src/backend/scraping_test.go` - Add comprehensive mocking

## Success Metrics

- **Minimum Target:** 60% coverage (Phase 1 + Phase 2 partial)
- **Goal Target:** 80% coverage (All phases)
- **SonarCloud:** Pass quality gate
- **CI/CD:** All tests pass with increased coverage
