---
applyTo: "templates/**"
description: "HTML template patterns for the site-planner HTMX frontend. Use when: editing HTML templates, adding HTMX interactions, or modifying the UI."
---

# Template Patterns

## HTMX Conventions

- Load HTMX from CDN: `https://unpkg.com/htmx.org@2.0.4`
- Use `hx-post` for form submissions, `hx-target` for swap target, `hx-indicator` for loading state
- Result partials are plain HTML fragments (no `<html>`, `<head>`, or `<body>` wrappers)
- Loading indicator uses `.htmx-request` CSS class toggling

## Template Data

- `index.html`: No data context (static page)
- `result.html`: Receives `map[string]interface{}` with keys: `Address`, `ParcelID`, `Area`, `EdgeCount`, `DownloadID`, `Error`
- Error state: When `Error` key is present, render error UI instead of success

## Styling

- Inline CSS in `<style>` block (no external CSS framework)
- Dark theme: background `#0f172a`, cards `#1e293b`, borders `#334155`
- Accent: blue `#3b82f6` for buttons, green `#22c55e` for success
