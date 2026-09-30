# ServeFlow frontend

The React application is built with Vite, React Router, Tailwind CSS, Lucide React, and Axios. The landing page is implemented; application routes are intentionally limited to clear Day 1 placeholders.

## Run locally

```powershell
Copy-Item .env.example .env
npm install
npm run dev
```

Set `VITE_API_BASE_URL` in `.env` to configure the shared Axios client. The default local value is `http://localhost:8080/api/v1`.

## Routes

- `/` — ServeFlow public landing page
- `/404` and unknown paths — Not found page
- `/login`, `/register`, `/dashboard`, `/tickets`, `/customers`, `/technicians`, `/appointments`, `/invoices`, `/analytics`, and `/settings` — Coming soon placeholders

Build for production with `npm run build`.