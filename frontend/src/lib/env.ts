// Public build-time configuration (Vite inlines VITE_* values into the bundle).
// Keep missing configuration from crashing module evaluation; main.tsx can then
// render a useful setup message instead of leaving the browser on a blank page.
// Only the Supabase anon key belongs here. Never put a service-role key in the frontend.
const supabaseUrl = import.meta.env.VITE_SUPABASE_URL ?? "";
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY ?? "";

export const env = {
  apiUrl: (import.meta.env.VITE_API_URL ?? "http://localhost:8080").replace(/\/+$/, ""),
  supabaseUrl,
  supabaseAnonKey,
  missing: [
    !supabaseUrl && "VITE_SUPABASE_URL",
    !supabaseAnonKey && "VITE_SUPABASE_ANON_KEY",
  ].filter((name): name is string => Boolean(name)),
};
