import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Mail, RefreshCw, Unplug } from "lucide-react";
import { toast } from "sonner";
import { contactsApi } from "@/api/crud";
import {
  useDismissUnmatched,
  useGmailConnect,
  useGmailDisconnect,
  useGmailStatus,
  useGmailSync,
  useResolveUnmatched,
  useUnmatched,
} from "@/api/gmail";
import { ApiError, errorMessage } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import type { UnmatchedEmail } from "@/types/api";
import { PageHeader } from "@/components/PageHeader";
import { ErrorState } from "@/components/ErrorState";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { EmptyState } from "@/components/EmptyState";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";

// Result codes the backend callback appends to /settings/gmail?status=...
const CALLBACK_MESSAGES: Record<string, { ok: boolean; text: string }> = {
  connected: { ok: true, text: "Gmail connected." },
  denied: { ok: false, text: "Access was denied in Google, so Gmail was not connected." },
  invalid_state: { ok: false, text: "The connection link expired or was already used. Please try again." },
  exchange_failed: { ok: false, text: "Google did not accept the authorization. Please try again." },
  no_refresh_token: {
    ok: false,
    text: "Google did not return offline access. Remove this app at myaccount.google.com/permissions and connect again.",
  },
  scope_missing: { ok: false, text: "The Gmail read permission was not granted. Please try again and allow it." },
  error: { ok: false, text: "Something went wrong while connecting Gmail." },
};

function UnmatchedRow({ item }: { item: UnmatchedEmail }) {
  const contacts = contactsApi.useList({ page_size: 100 });
  const resolve = useResolveUnmatched();
  const dismiss = useDismissUnmatched();
  const [contactId, setContactId] = useState("");

  return (
    <div className="space-y-3 rounded-md border p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="truncate font-medium">{item.subject || "(no subject)"}</p>
          <p className="text-sm text-muted-foreground">
            {item.from_address} · {formatDateTime(item.received_at)}
          </p>
          {(item.extracted_name || item.extracted_profile_url) && (
            <p className="mt-1 text-sm">
              Looks like: <span className="font-medium">{item.extracted_name ?? "unknown name"}</span>
            </p>
          )}
        </div>
        <Badge variant="outline" className="shrink-0">{item.reason === "AMBIGUOUS" ? "Ambiguous" : "No match"}</Badge>
      </div>
      <p className="text-xs text-muted-foreground">
        {item.reason === "AMBIGUOUS" ? "More than one contact could match this email." : "No contact with a pending invitation matched."}
      </p>
      <div className="flex flex-col gap-2 sm:flex-row">
        <Select className="sm:w-64" value={contactId} onChange={(e) => setContactId(e.target.value)} aria-label="Choose contact">
          <option value="">Choose the contact...</option>
          {contacts.data?.data.map((c) => (
            <option key={c.id} value={c.id}>
              {c.full_name}
              {c.company_name ? ` (${c.company_name})` : ""}
            </option>
          ))}
        </Select>
        <Button
          size="sm"
          disabled={!contactId || resolve.isPending}
          onClick={async () => {
            try {
              await resolve.mutateAsync({ id: item.id, contactId });
              toast.success("Marked as accepted for that contact");
            } catch (err) {
              toast.error(errorMessage(err));
            }
          }}
        >
          {resolve.isPending && <Spinner />} Mark accepted
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={dismiss.isPending}
          onClick={async () => {
            try {
              await dismiss.mutateAsync(item.id);
              toast.success("Dismissed");
            } catch (err) {
              toast.error(errorMessage(err));
            }
          }}
        >
          Dismiss
        </Button>
      </div>
    </div>
  );
}

export default function GmailSettings() {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const [banner, setBanner] = useState<{ ok: boolean; text: string } | null>(null);

  // Show the OAuth callback result once, then drop the query string so a refresh does not repeat it.
  useEffect(() => {
    const code = params.get("status");
    if (code) {
      setBanner(CALLBACK_MESSAGES[code] ?? CALLBACK_MESSAGES.error);
      navigate("/settings/gmail", { replace: true });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const status = useGmailStatus();
  const connect = useGmailConnect();
  const sync = useGmailSync();
  const disconnect = useGmailDisconnect();
  const unmatched = useUnmatched({ status: "PENDING", page_size: 50 });
  const [confirmDisconnect, setConfirmDisconnect] = useState(false);

  async function runSync() {
    try {
      const r = await sync.mutateAsync();
      toast.success(
        `Checked ${r.processed} email${r.processed === 1 ? "" : "s"}: ${r.matched} matched, ${r.unmatched} to review` +
          (r.has_more ? ". More are waiting, sync again." : "."),
      );
    } catch (err) {
      if (err instanceof ApiError && err.code === "gmail_reauth_required") {
        toast.error("Google access expired. Reconnect Gmail.");
      } else {
        toast.error(errorMessage(err));
      }
    }
  }

  if (status.error) return <ErrorState error={status.error} onRetry={() => void status.refetch()} />;
  const s = status.data;

  return (
    <>
      <PageHeader
        title="Gmail"
        description="Read-only. When LinkedIn emails you that an invitation was accepted, the matching contact moves to Connected. Run a sync yourself whenever you want; nothing runs in the background."
      />

      {banner && (
        <div
          className={`mb-4 rounded-md border px-4 py-3 text-sm ${
            banner.ok ? "border-green-200 bg-green-50 text-green-800" : "border-red-200 bg-red-50 text-red-800"
          }`}
        >
          {banner.text}
        </div>
      )}

      <div className="max-w-3xl space-y-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Mail className="h-4 w-4" /> Connection
            </CardTitle>
            <CardDescription>The app only requests permission to read mail (gmail.readonly) and never sends or deletes anything.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {status.isLoading || !s ? (
              <Skeleton className="h-20" />
            ) : !s.enabled ? (
              <p className="text-sm text-muted-foreground">
                Gmail sync is not configured on the server. Set the Google OAuth variables on the backend to enable it (see the deployment guide).
              </p>
            ) : !s.connected ? (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">No Gmail account connected.</p>
                <Button disabled={connect.isPending} onClick={() => connect.mutate(undefined, { onError: (e) => toast.error(errorMessage(e)) })}>
                  {connect.isPending && <Spinner />} Connect Gmail
                </Button>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="text-sm">
                  <p className="font-medium">{s.email_address}</p>
                  <p className="text-muted-foreground">
                    Last synced {s.last_synced_at ? formatDateTime(s.last_synced_at) : "never"}
                  </p>
                  {s.status === "REAUTH_REQUIRED" && (
                    <p className="mt-1 text-destructive">Google access expired. Reconnect to continue syncing.</p>
                  )}
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button disabled={sync.isPending || s.status === "REAUTH_REQUIRED"} onClick={() => void runSync()}>
                    {sync.isPending ? <Spinner /> : <RefreshCw className="h-4 w-4" />} Sync now
                  </Button>
                  {s.status === "REAUTH_REQUIRED" && (
                    <Button variant="outline" onClick={() => connect.mutate(undefined, { onError: (e) => toast.error(errorMessage(e)) })}>
                      Reconnect
                    </Button>
                  )}
                  <Button variant="outline" onClick={() => setConfirmDisconnect(true)}>
                    <Unplug className="h-4 w-4" /> Disconnect
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {s?.connected && (
          <Card>
            <CardHeader>
              <CardTitle>Needs your review</CardTitle>
              <CardDescription>
                Emails that could not be matched to exactly one contact. Choose the right contact or dismiss the email.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {unmatched.isLoading ? (
                <Skeleton className="h-24" />
              ) : unmatched.error ? (
                <ErrorState error={unmatched.error} onRetry={() => void unmatched.refetch()} />
              ) : unmatched.data && unmatched.data.data.length === 0 ? (
                <EmptyState title="Nothing to review" description="Unmatched acceptance emails will show up here after a sync." />
              ) : (
                unmatched.data?.data.map((u) => <UnmatchedRow key={u.id} item={u} />)
              )}
              <p className="text-xs text-muted-foreground">
                Contacts missing? Add them under <Link to="/contacts" className="underline">Contacts</Link>.
              </p>
            </CardContent>
          </Card>
        )}
      </div>

      <ConfirmDialog
        open={confirmDisconnect}
        onOpenChange={setConfirmDisconnect}
        title="Disconnect Gmail?"
        description="The stored Google token and sync history are deleted. Your contacts and referrals stay."
        confirmLabel="Disconnect"
        loading={disconnect.isPending}
        onConfirm={async () => {
          try {
            await disconnect.mutateAsync();
            toast.success("Gmail disconnected");
          } catch (err) {
            toast.error(errorMessage(err));
          }
          setConfirmDisconnect(false);
        }}
      />
    </>
  );
}
