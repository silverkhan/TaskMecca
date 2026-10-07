# A19 CI isolation follow-up

Original artifact run 37570527972 failed at lifecycle_http_integration_test.go:80: release-assets.githubusercontent.com VERSION.txt redirect entered the Telegram-only global mock. Earlier background CachedVersionInfo refreshes can remain in flight while this fixture temporarily installs http.DefaultTransport.

Test-only correction: the fixture intercepts api.telegram.org only and delegates all unrelated HTTP to the captured original RoundTripper. Telegram sendMessage validation and exact send counts remain unchanged. A deterministic regression checks initial GitHub version requests and release-assets redirects reach fallback, while Telegram reaches its own handler. No production, maintenance, UI or operational data changes.

Local validation: lifecycle projection -count=20 PASS; isolation plus lifecycle -race -count=10 PASS; full go test ./... PASS; git diff --check PASS. Prior UI evidence, original failures and isolated fixture remain preserved. Pending older CI was inspected before pushing; newer-head Linux artifact CI is authoritative.

Resume identity: assignment-ccad7c5c08195c37384e024ae0e8067b; attempt run-92e30255a8eed691; runtime 01a11477-13aa-76e3-866a-a8d007002787; turn 01a11493-b31d-71a1-9d74-697e7a2c1fc4; session 01a1101a-cbfd-79e2-91ee-6018b9bf2924. Parent bound assignment and recorded observed activity/resumed; start Hook was not observed and no historical started event was fabricated.
