# Frontend

React, TypeScript, Vite, Tailwind CSS, shadcn/ui components, Lucide icons, React Router, TanStack Query. Authentication is Supabase Auth; every API call carries the user's Supabase access token and the Go API (on Render) decides who the user is. The browser never sends a user id.

## Run locally

```
cp .env.example .env.local     # then fill in the three values
npm install
npm run dev                    # http://localhost:5173
```

Start the API too (see the root README) and point `VITE_API_URL` at it, e.g. `http://localhost:8080`. For local use, add `http://localhost:5173` to `ALLOWED_ORIGINS` and set `FRONTEND_URL=http://localhost:5173` in the backend environment.

| Variable | Meaning |
| --- | --- |
| `VITE_API_URL` | API base URL, without a trailing slash and without `/api/v1` |
| `VITE_SUPABASE_URL` | `https://<ref>.supabase.co` |
| `VITE_SUPABASE_ANON_KEY` | Supabase anon (publishable) key. **Never the service-role key.** |

`VITE_*` values are embedded in the public bundle. Only put values here that are safe for anyone to see.

## Scripts

- `npm run dev` development server
- `npm run build` typecheck (`tsc -b`) then production build into `dist/`
- `npm run preview` serve the production build locally
- `npm run typecheck` types only

## Deploy to Vercel

Import the monorepo and set **Root Directory** to `frontend`. `vercel.json` handles the build command, the SPA rewrite and skipping builds for backend-only commits. Add the three variables above. Full steps, including the CORS and Supabase redirect settings, are in `../docs/deployment.md`.

## Structure

```
src/
  lib/        env, supabase client, API client (envelope, errors, auth header), formatting
  types/      API types that mirror docs/api-contracts.md
  api/        TanStack Query hooks per area (crud, referrals, gmail)
  hooks/      auth provider, debounce, field-error mapping
  components/ layout, shared pieces, ui/ (shadcn-style primitives)
  pages/      one file per route
```

## Pages

Dashboard, Referrals (list and detail with the workflow, history, notes and follow-up drafts), Follow-ups due, Jobs, Companies, Contacts, Templates, Settings, Gmail (`/settings/gmail`, also the landing page after Google's consent screen).

## Notes

- Server validation errors (HTTP 422) are shown next to the matching form field.
- A 401 from the API signs the user out and returns them to the login page.
- All cached data is cleared on sign-in and sign-out so one account never sees another's data on a shared browser.
- Messages are never sent by this app. The follow-up draft is copied by you, sent on LinkedIn by you, and then marked as sent.
