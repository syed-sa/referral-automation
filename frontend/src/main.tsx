import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter } from "react-router-dom";
import { Toaster } from "sonner";
import { queryClient } from "@/lib/queryClient";
import { AuthProvider } from "@/hooks/useAuth";
import { env } from "@/lib/env";
import App from "./App";
import "./index.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      {env.missing.length ? (
        <main className="mx-auto flex min-h-screen max-w-2xl items-center px-6">
          <section className="w-full rounded-lg border bg-white p-8 shadow-sm">
            <h1 className="text-2xl font-semibold">Finish configuring Referral Tracker</h1>
            <p className="mt-3 text-muted-foreground">
              The frontend is running, but these public environment variables are missing:
            </p>
            <ul className="mt-3 list-inside list-disc font-mono text-sm">
              {env.missing.map((name) => <li key={name}>{name}</li>)}
            </ul>
            <p className="mt-4 text-sm text-muted-foreground">
              Copy <code>.env.example</code> to <code>.env.local</code> in the frontend folder,
              fill in the Supabase project URL and anon key, then restart the dev server.
            </p>
          </section>
        </main>
      ) : (
        <AuthProvider>
          <BrowserRouter>
            <App />
          </BrowserRouter>
          <Toaster richColors position="top-right" />
        </AuthProvider>
      )}
    </QueryClientProvider>
  </React.StrictMode>,
);
