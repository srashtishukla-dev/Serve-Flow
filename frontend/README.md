# ServeFlow frontend

The React application is built with Vite, React Router, Tailwind CSS, Lucide React, and Axios. It includes authenticated workspace pages for customers, services, technicians, bookings, appointments, invoices, and admin analytics.

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
- `/login`, `/register` — Authentication
- `/dashboard` — Account and workspace overview
- `/customers`, `/technicians`, `/services` — Organization management
- `/bookings` — Admin booking management with server-side filtering and pagination
- `/appointments` — Role-scoped appointment scheduling and status workflow
- `/invoices` — Admin invoice/payment management and customer-scoped invoice history
- `/analytics` — Organization-scoped admin analytics with date presets, invoice totals, service popularity, customer activity, and technician workload
- `/tickets`, `/settings` — Coming soon placeholders

Build for production with `npm run build`.