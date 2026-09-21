# Social Finance — Web Frontend

React + TypeScript + Vite application for Social Finance.

## Development

```bash
npm install
npm run dev
```

Dev server runs on `http://localhost:3000`.

## Mock Login (DEVELOPMENT ONLY)

This frontend uses **temporary mock authentication only**. No backend calls are made.

```
Username: admin
Password: admin
```

Login sets a `SUPER_ADMIN` session in localStorage. Logout clears it.

> **THIS IS DEVELOPMENT MOCK AUTHENTICATION ONLY.**
> The `admin/admin` credentials exist solely in the browser.
> They are NOT stored in the backend database.
> This will be replaced when real Auth integration begins.

## Docker

Build production image:

```bash
docker build -t social-finance-frontend .
```

Run:

```bash
docker run --rm -p 8081:80 social-finance-frontend
```

The container serves on port 80; mapped to host port 8081.

Build from repository root:

```bash
docker build -t social-finance-frontend ./frontend
docker run --rm -p 8081:80 social-finance-frontend
```

---

**NOTICE:** This is a web client project under `frontend/`
(Formerly `web/` as of Web W1 Correction).
