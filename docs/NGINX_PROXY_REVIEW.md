# Nginx Proxy Configuration Review
## OAuth Server Failover System

**Date:** 2025-10-29
**Focus:** Failover reliability and speed
**Traffic Scale:** < 100 req/s
**Configuration:** `helm/templates/nginx-configmap.yaml`

---

## Executive Summary

The nginx configuration implements a Lua-based active health checking system for OAuth/API endpoint failover. While functional, several issues affect failover reliability, speed, and maintainability.

### Key Findings

- ⚠️ **Critical:** Conflicting failover mechanisms (nginx native + Lua custom)
- 🔴 **High:** 400+ lines of duplicate health check code
- ⚠️ **Reliability:** OIDC has no failover (single point of failure)
- ⏱️ **Speed:** 30-second cache TTL delays failover detection by up to 30 seconds
- 🔄 **Issue:** Race conditions possible between "/" and "/apis" location health checks

---

## Detailed Findings

### 1. Conflicting Failover Mechanisms ⚠️ CRITICAL

**Problem:**
Two different failover systems are configured simultaneously:

1. **Nginx native upstream failover** (lines 31-43):
   ```nginx
   upstream oauth_backend {
       server primary:443 fail_timeout=15s max_fails=3;
       server secondary:443 backup;
   }
   ```

2. **Lua-based dynamic routing** (lines 65-178):
   ```lua
   proxy_pass https://$chosen_upstream;  # Bypasses nginx upstream
   ```

**Impact:**
- Unpredictable failover behavior
- Nginx's built-in logic never activates because Lua routing bypasses it
- Confusion about which system is actually handling failover

**Resolution:**
Remove upstream blocks entirely since Lua handles all routing. The recommended configuration eliminates this conflict.

---

### 2. Duplicate Code - 400+ Lines 🔴 HIGH

**Problem:**
The `check_upstream_health()` function appears identically in:
- Lines 86-109 (location "/")
- Lines 206-229 (location "/apis")

Plus surrounding health check logic is duplicated throughout both locations.

**Impact:**
- Bug fixes require updating two places
- Risk of code divergence
- Difficult to maintain and test
- Increased ConfigMap size

**Resolution:**
Extract to shared function in `init_by_lua_block` (see recommended config).

**Example:**
```lua
init_by_lua_block {
    function check_upstream_health(oauth_host, api_host, port, name)
        -- Single implementation used by all locations
    end

    function get_healthy_upstream()
        -- Shared health check and upstream selection logic
    end
}
```

---

### 3. OIDC Single Point of Failure ⚠️ RELIABILITY

**Problem:**
OIDC endpoint has no health checking or failover:

```lua
-- Line 312 comment: "OIDC doesn't have failover yet"
chosen_upstream = primary_oidc_host .. ":" .. primary_oidc_port
```

**Impact:**
- If primary OIDC endpoint fails, all OIDC token validations fail
- No automatic recovery
- Different reliability than OAuth/API endpoints

**Resolution Options:**

**Option A:** Add OIDC failover
```yaml
# values.yaml
proxy:
  upstreams:
    secondary:
      oidc: "dowa-backup.apps.k8s.r4c-test.solution.sbt"
```

**Option B:** Document limitation clearly
```yaml
# values.yaml with comments explaining single OIDC endpoint
```

**Option C:** Return error instead of failing silently
```lua
if not oidc_healthy then
    ngx.status = 503
    return ngx.exit(503)
end
```

---

### 4. Slow Failover Detection ⏱️ SPEED

**Current Configuration:**
```lua
local cache_ttl = 30  -- Line 67
```

**Timeline of Failure:**
```
T=0s:    Primary goes down
T=0-30s: Cached "healthy" status used, requests continue to fail
T=30s:   Cache expires, health check detects failure
T=30s+:  Requests start routing to secondary
```

**Result:** Up to 30 seconds of failed requests before failover

**For < 100 req/s Traffic:**
- 10-second TTL provides 3x faster detection
- Overhead is minimal: 6 health checks/minute vs 2/minute
- Each check: 2 HTTPS requests (OAuth + API) with 3s timeout

**Recommended:**
```lua
local cache_ttl = 10  -- Faster detection, acceptable overhead
```

**To Configure:**
```yaml
# values.yaml
proxy:
  healthCheck:
    cacheTtl: 10  # seconds
```

---

### 5. Cache Sharing Race Conditions 🔄

**Problem:**
Both locations use shared cache with separate locks:

```lua
# Location "/" uses:
health_cache:add("health_check_lock", ...)

# Location "/apis" uses:
health_cache:add("health_check_lock_apis", ...)

# But both write to:
health_cache:set("primary_healthy", ...)
health_cache:set("secondary_healthy", ...)
```

**Potential Race Condition:**
```
T=0: Location "/" checks health → primary healthy → writes cache
T=5: Location "/apis" checks health → primary unhealthy → overwrites cache
T=6: Location "/" reads cache → sees unhealthy → unnecessary failover
```

**Resolution:**
Use background timer for health checks instead of per-request:

```lua
init_worker_by_lua_block {
    local function health_check_timer(premature)
        if premature then return end

        -- Perform health check
        -- Update cache

        -- Schedule next check
        ngx.timer.at(10, health_check_timer)
    end

    -- Start timer
    ngx.timer.at(0, health_check_timer)
}
```

**Benefits:**
- Single health checking process
- No lock contention
- Consistent cache state
- More efficient

---

### 6. Configuration Portability 🔧

**Hardcoded DNS Resolver:**
```nginx
resolver 169.254.20.10;  # Line 16
```

This is a Kubernetes-specific DNS resolver that won't work in other environments.

**Resolution:**
```yaml
# values.yaml
proxy:
  dnsResolver: "169.254.20.10"
```

```nginx
# nginx-configmap.yaml
resolver {{ .Values.proxy.dnsResolver }};
```

---

### 7. Health Check Security 🔒

**Current:**
```lua
ssl_verify = false  -- Lines 93, 100, 213, 220
```

**Risk:** Man-in-the-middle attacks on health checks

**Assessment:**
- Low risk for internal service-to-service communication
- May be necessary if using self-signed certificates
- Should be documented or made configurable

**Resolution:**
```yaml
# values.yaml
proxy:
  healthCheck:
    sslVerify: false  # Set true for production with proper certs
```

```lua
ssl_verify = {{ .Values.proxy.healthCheck.sslVerify | default false }}
```

---

### 8. Error Handling When Both Down

**Current Behavior:**
```lua
-- Lines 159-163, 279-283
else
    -- Both unhealthy, fallback to secondary anyway
    chosen_upstream = secondary_oauth_host .. ":" .. secondary_port
    ngx.log(ngx.WARN, "Both upstreams unhealthy, falling back to secondary")
end
```

**Result:** Returns secondary even when known to be unhealthy, causes proxy errors

**Alternative Approach:**
```lua
if not primary_healthy and secondary_healthy_cached ~= 1 then
    ngx.status = 503
    ngx.say('{"error": "All OAuth upstreams unavailable"}')
    ngx.header.content_type = "application/json"
    return ngx.exit(503)
end
```

**Tradeoff:**
- Current: Attempts secondary, may work if health check was wrong
- Alternative: Fails fast with clear error, prevents cascading failures

**Recommendation:** Make configurable via `values.yaml`

---

## What's Working Well ✅

1. **Comprehensive Logging** (lines 22-25)
   - Includes upstream selection, status, and addresses
   - Added custom `chosen_backend` field in recommended config

2. **Connection Timeouts** (lines 175-177, 352-355)
   - 5s connect timeout: Quick failure detection
   - 30s read/send timeout: Adequate for OAuth flows

3. **Modern TLS Configuration** (lines 50-52)
   - TLS 1.2 and 1.3 only
   - Strong cipher suites
   - Proper cipher ordering

4. **Lock Mechanism** (lines 117, 237)
   - Prevents thundering herd problem
   - Multiple requests don't trigger simultaneous health checks

5. **Dual Endpoint Validation** (lines 103-105)
   - Checks both OAuth AND API endpoints
   - Only marks healthy if both are responding
   - Appropriate for your failover requirements

6. **TokenReview Routing Logic** (lines 298-315)
   - Intelligent routing based on token type
   - `sha256~` prefix detection for OAuth tokens
   - Fallback to OIDC endpoint for other formats

---

## Implementation Recommendations

### Phase 1: Critical Fixes (Immediate - 2-4 hours)

#### 1.1 Remove Upstream Blocks
**File:** `helm/templates/nginx-configmap.yaml`
**Lines:** 31-43

**Action:** Delete these lines entirely
```nginx
# DELETE THESE LINES:
upstream oauth_backend { ... }
upstream oidc_api_backend { ... }
upstream oauth_api_backend { ... }
```

**Reason:** Conflicts with Lua-based routing

---

#### 1.2 Consolidate Health Check Code
**File:** `helm/templates/nginx-configmap.yaml`
**Lines:** 86-109 duplicated in 206-229

**Action:** Extract to `init_by_lua_block` (see recommended config)

**Testing:**
```bash
# After applying changes
kubectl logs -n <namespace> -l app=nginx-proxy --tail=50 -f | grep "health check"

# Should see:
# Primary health check - OAuth: true, API: true, Overall: true
```

---

#### 1.3 Reduce Cache TTL
**File:** `helm/values.yaml`

**Add:**
```yaml
proxy:
  healthCheck:
    cacheTtl: 10  # Changed from 30
```

**File:** `helm/templates/nginx-configmap.yaml`
**Line:** 67

**Change:**
```lua
local cache_ttl = {{ .Values.proxy.healthCheck.cacheTtl | default 10 }}
```

**Testing:**
```bash
# Monitor failover speed
# Disable primary upstream, observe logs for detection time
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep "Cache expired"

# Should see cache expiration every 10 seconds
```

---

### Phase 2: Reliability Improvements (Short-term - 4-8 hours)

#### 2.1 Add OIDC Failover
**File:** `helm/values.yaml`

**Add:**
```yaml
proxy:
  upstreams:
    secondary:
      oidc: "dowa-backup.apps.k8s.r4c-test.solution.sbt"
  failover:
    oidcFailoverEnabled: true
```

**File:** `helm/templates/nginx-configmap.yaml`

Extend health check to include OIDC endpoint validation

---

#### 2.2 Make DNS Resolver Configurable
**File:** `helm/values.yaml`

**Add:**
```yaml
proxy:
  dnsResolver: "169.254.20.10"
```

**File:** `helm/templates/nginx-configmap.yaml`
**Line:** 16

**Change:**
```nginx
resolver {{ .Values.proxy.dnsResolver | default "169.254.20.10" }};
```

---

#### 2.3 Implement Recommended Configuration
**File:** `helm/templates/nginx-configmap.yaml`

**Action:** Replace with recommended configuration

**Key Changes:**
- Single `get_healthy_upstream()` function
- Shared health check logic
- Better error handling
- Metrics endpoint
- Configurable options

---

### Phase 3: Enhancements (When needed)

#### 3.1 Background Health Check Timer
**Benefit:** Eliminates race conditions, more efficient

**Implementation:**
```lua
init_worker_by_lua_block {
    local function health_check_timer(premature)
        -- Run health checks every 10 seconds
    end
    ngx.timer.at(0, health_check_timer)
}
```

---

#### 3.2 Metrics and Observability
**Add:** Prometheus metrics endpoint

**Metrics to track:**
- `failover_total`: Count of failover events
- `primary_healthy`: Current primary health status (0/1)
- `secondary_healthy`: Current secondary health status (0/1)
- `health_check_duration`: Time to complete health checks
- `requests_by_backend`: Requests routed to primary vs secondary

**Usage:**
```bash
curl http://<nginx-proxy>/nginx-metrics
```

---

#### 3.3 Certificate Verification
**File:** `helm/values.yaml`

**Add:**
```yaml
proxy:
  healthCheck:
    sslVerify: true  # Enable for production
    caFile: /etc/ssl/certs/ca-bundle.crt  # Optional custom CA
```

---

## Testing Strategy

### 1. Health Check Validation

**Test normal operation:**
```bash
# Deploy configuration
kubectl apply -f helm/

# Monitor logs
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep "health check"

# Expected output:
# Primary health check - OAuth: true, API: true, Overall: true
# Using primary upstream
```

---

### 2. Primary Failover Test

**Simulate primary failure:**
```bash
# Option A: Scale down primary oauth-server
kubectl scale deployment oauth-console -n <primary-namespace> --replicas=0

# Option B: Block traffic with network policy
# Option C: Break service endpoint

# Monitor failover
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep -E "(unhealthy|failover)"

# Expected:
# Primary health check - OAuth: false, API: true, Overall: false
# Primary unhealthy, using secondary upstream

# Test requests still work
curl -k https://proxy-auth.apps.k8s.r4c-test.solution.sbt/healthz
```

**Verify timing:**
- Should detect failure within 10 seconds (new TTL)
- All subsequent requests route to secondary

---

### 3. Primary Recovery Test

**Restore primary:**
```bash
kubectl scale deployment oauth-console -n <primary-namespace> --replicas=1

# Wait for primary to be ready
kubectl wait --for=condition=ready pod -l app=oauth-console -n <primary-namespace>

# Monitor recovery
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep "health check"

# Expected within 10 seconds:
# Primary health check - OAuth: true, API: true, Overall: true
# Using primary upstream
```

---

### 4. Both Down Test

**Simulate both failing:**
```bash
# Scale down both
kubectl scale deployment oauth-console -n <primary-namespace> --replicas=0
kubectl scale deployment oauth-console -n <secondary-namespace> --replicas=0

# Test requests
curl -k https://proxy-auth.apps.k8s.r4c-test.solution.sbt/healthz

# Current behavior: Attempts secondary, returns proxy error
# Recommended behavior (if configured): Returns 503 with clear message
```

---

### 5. Load Test (Optional for Low Traffic)

**Simple load test:**
```bash
# Install hey (if needed)
go install github.com/rakyll/hey@latest

# Run load test during failover
hey -n 1000 -c 10 -q 10 \
    -H "Authorization: Bearer <token>" \
    https://proxy-auth.apps.k8s.r4c-test.solution.sbt/apis/user.openshift.io/v1/users/~

# During test, trigger failover:
kubectl scale deployment oauth-console -n <primary-namespace> --replicas=0

# Analyze results for errors during transition
```

---

### 6. TokenReview Routing Test

**Test OAuth token routing:**
```bash
# OAuth token (sha256~ prefix)
curl -k -X POST https://proxy-auth.apps.k8s.r4c-test.solution.sbt/apis/... \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "TokenReview",
    "spec": {"token": "sha256~abc123..."}
  }'

# Check logs
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep "TokenReview"
# Expected: "TokenReview with oauth token - routing to ... api backend"
```

**Test OIDC token routing:**
```bash
# OIDC token (no sha256~ prefix)
curl -k -X POST https://proxy-auth.apps.k8s.r4c-test.solution.sbt/apis/... \
  -H "Content-Type: application/json" \
  -d '{
    "kind": "TokenReview",
    "spec": {"token": "eyJhbGc..."}
  }'

# Check logs
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep "TokenReview"
# Expected: "TokenReview with oidc token - routing to oidc_api_backend/validate-token"
```

---

### 7. Cache TTL Verification

**Verify cache behavior:**
```bash
# Enable debug logging temporarily
kubectl set env deployment/nginx-proxy -n <namespace> NGINX_LOG_LEVEL=debug

# Monitor cache age
kubectl logs -n <namespace> -l app=nginx-proxy -f | grep "cache"

# Expected output every 10 seconds:
# Cache expired (10s), performing health check...
# Using cached health status (age: 3s) - Primary: true
# Using cached health status (age: 7s) - Primary: true
```

---

## Rollback Plan

If issues occur after deployment:

### Quick Rollback
```bash
# Revert to previous version
kubectl rollout undo deployment/nginx-proxy -n <namespace>

# Or apply old configmap
kubectl apply -f helm/templates/nginx-configmap.yaml.backup
kubectl rollout restart deployment/nginx-proxy -n <namespace>
```

### Gradual Rollback
```bash
# If using recommended config, revert specific values:

# 1. Increase cache TTL back to 30s
helm upgrade oauth-server ./helm \
  --set proxy.healthCheck.cacheTtl=30 \
  --reuse-values

# 2. Disable metrics endpoint
helm upgrade oauth-server ./helm \
  --set proxy.observability.metricsEnabled=false \
  --reuse-values
```

---

## Monitoring Checklist

After deployment, monitor these metrics:

- [ ] Health check logs appear every 10 seconds
- [ ] No error logs related to health checking
- [ ] Failover events trigger within 10 seconds of primary failure
- [ ] Recovery to primary occurs within 10 seconds of restoration
- [ ] Request success rate remains high during failovers
- [ ] No increase in 5xx errors
- [ ] Memory usage stable (watch for Lua memory leaks)
- [ ] CPU usage within expected range

---

## Additional Considerations

### For Higher Traffic in Future

If traffic exceeds 100 req/s:

1. **Increase cache TTL**: 20-30 seconds to reduce health check overhead
2. **Implement background health checks**: Use `init_worker_by_lua_block` timer
3. **Add connection pooling**: Configure keepalive connections
4. **Enable metrics**: Track failover frequency and backend health
5. **Consider native nginx upstream**: If dynamic routing not needed

### Security Hardening

1. **Enable SSL verification**: Use proper certificates for health checks
2. **Implement rate limiting**: Prevent health check abuse
3. **Restrict metrics endpoint**: Add authentication if exposed externally
4. **Review token inspection**: Ensure token type detection is secure

### Operational Improvements

1. **Alerts**: Set up monitoring alerts for:
   - Frequent failovers (> 5 per hour)
   - Both upstreams down
   - Health check failures

2. **Runbooks**: Document procedures for:
   - Manual failover
   - Recovering from both-down scenario
   - Troubleshooting health check issues

3. **Regular testing**: Schedule monthly failover drills

---

## Files Created

1. **`nginx-configmap-recommended.yaml`** - Improved configuration with:
   - Consolidated health check code
   - Shared `get_healthy_upstream()` function
   - Configurable cache TTL
   - Metrics endpoint
   - Better error handling

2. **`values-recommended-additions.yaml`** - Configuration additions:
   - `proxy.dnsResolver`
   - `proxy.healthCheck.cacheTtl`
   - `proxy.healthCheck.sslVerify`
   - `proxy.failover.*` options
   - `proxy.observability.*` options

3. **`NGINX_PROXY_REVIEW.md`** - This comprehensive review

---

## Next Steps

1. **Review recommended configuration** - Compare with current setup
2. **Plan deployment** - Choose phase 1, 2, or 3 implementation
3. **Setup testing environment** - Prepare for failover testing
4. **Deploy changes** - Start with phase 1 critical fixes
5. **Monitor and validate** - Use testing strategy above
6. **Iterate** - Move to phase 2 and 3 as needed

---

## Questions to Consider

1. **Is OIDC failover required?** - Depends on OIDC availability requirements
2. **What should happen when both upstreams are down?** - Return 503 or attempt anyway?
3. **Is 10-second detection acceptable?** - Could go lower (5s) or higher (15s)
4. **Do you need metrics?** - Helpful for monitoring but adds complexity
5. **Certificate verification for health checks?** - Security vs. operational complexity

---

## Summary

The current configuration is functional but has reliability and maintainability issues. The recommended changes focus on:

- **Faster failover** (30s → 10s detection)
- **Cleaner code** (eliminate duplication)
- **Better reliability** (fix race conditions, add OIDC failover)
- **More configurable** (externalize settings to values.yaml)
- **More observable** (metrics endpoint, better logging)

For your low-traffic use case (< 100 req/s), implementing Phase 1 recommendations will provide immediate improvement with minimal risk.
