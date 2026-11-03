import { createClient } from "@supabase/supabase-js";
import { env } from "./env";

// Safe placeholders let the app render its configuration screen when .env is
// missing. The client is never used until the required values are configured.
export const supabase = createClient(
  env.supabaseUrl || "https://configuration-required.supabase.co",
  env.supabaseAnonKey || "configuration-required-anon-key",
  {
  auth: { persistSession: true, autoRefreshToken: true, detectSessionInUrl: true },
  },
);
