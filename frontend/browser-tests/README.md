# Browser-level smoke tests

Component tests (`frontend/component-tests`) run against jsdom and are
verified as part of this repository's construction. Full browser-level
end-to-end tests against a deployed baseline/candidate (Playwright or
equivalent) require the SOMA Genesis Sandbox's live preview URL and were
not run in this construction pass — see `VERIFICATION.md`.

Planned coverage once a sandbox preview is available:

- Order search → approve → fulfil happy path.
- Recovery Actions bulk-status override, confirming the UI reflects the
  updated status.
- Configuration view round-trip (put then get).
- Navigation across all eight tabs with no console errors.