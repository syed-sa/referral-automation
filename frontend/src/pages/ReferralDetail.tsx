import { useEffect, useState, type FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Check, Copy, ExternalLink, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";
import {
  useDeleteJobContact,
  useGenerateFollowup,
  useJobContact,
  useJobContactEvents,
  useMarkFollowupSent,
  useMessages,
  useTemplates,
  useTransition,
  useUpdateJobContactNotes,
  useUpdateMessage,
} from "@/api/referrals";
import { ApiError, errorMessage } from "@/lib/api";
import { emptyToNull, formatDateTime, statusLabel } from "@/lib/format";
import type { JobContact, Message, WorkflowStatus } from "@/types/api";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { ErrorState } from "@/components/ErrorState";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FormField } from "@/components/FormField";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";

function toIsoOrUndefined(local: string): string | undefined {
  if (!local) return undefined;
  const d = new Date(local);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

function nowLocalInput(): string {
  const d = new Date();
  d.setMinutes(d.getMinutes() - d.getTimezoneOffset());
  return d.toISOString().slice(0, 16);
}

function TransitionDialog({ jc, to, onClose }: { jc: JobContact; to: WorkflowStatus | null; onClose: () => void }) {
  const transition = useTransition();
  const [when, setWhen] = useState("");
  const [note, setNote] = useState("");

  useEffect(() => {
    if (to) {
      setWhen("");
      setNote("");
    }
  }, [to]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!to) return;
    try {
      await transition.mutateAsync({
        id: jc.id,
        input: { to_status: to, occurred_at: toIsoOrUndefined(when), note: emptyToNull(note) ?? undefined },
      });
      toast.success(`Moved to ${statusLabel(to)}`);
      onClose();
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  return (
    <Dialog open={!!to} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Move to {to ? statusLabel(to) : ""}</DialogTitle>
          <DialogDescription>
            Currently {statusLabel(jc.status)}. Leave the time empty to use now; set it if this happened earlier.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <FormField label="When did it happen? (optional)" htmlFor="t-when">
            <Input id="t-when" type="datetime-local" max={nowLocalInput()} value={when} onChange={(e) => setWhen(e.target.value)} />
          </FormField>
          <FormField label="Note (optional)" htmlFor="t-note">
            <Textarea id="t-note" value={note} onChange={(e) => setNote(e.target.value)} />
          </FormField>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={transition.isPending}>
              {transition.isPending && <Spinner />} Confirm
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function NotesCard({ jc }: { jc: JobContact }) {
  const save = useUpdateJobContactNotes();
  const [notes, setNotes] = useState(jc.notes ?? "");
  useEffect(() => setNotes(jc.notes ?? ""), [jc.notes]);
  const dirty = notes !== (jc.notes ?? "");

  return (
    <Card>
      <CardHeader>
        <CardTitle>Notes</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <Textarea value={notes} onChange={(e) => setNotes(e.target.value)} rows={4} />
        <Button
          size="sm"
          disabled={!dirty || save.isPending}
          onClick={async () => {
            try {
              await save.mutateAsync({ id: jc.id, notes: emptyToNull(notes) });
              toast.success("Notes saved");
            } catch (err) {
              toast.error(errorMessage(err));
            }
          }}
        >
          {save.isPending && <Spinner />} Save notes
        </Button>
      </CardContent>
    </Card>
  );
}

function DraftEditor({ message, jobContactId, canMarkSent }: { message: Message; jobContactId: string; canMarkSent: boolean }) {
  const update = useUpdateMessage();
  const markSent = useMarkFollowupSent();
  const [body, setBody] = useState(message.body);
  useEffect(() => setBody(message.body), [message.id, message.body]);
  const dirty = body !== message.body;

  async function copy() {
    try {
      await navigator.clipboard.writeText(body);
      toast.success("Copied to clipboard");
    } catch {
      toast.error("Could not copy. Select the text and copy it manually.");
    }
  }

  async function save() {
    try {
      await update.mutateAsync({ id: message.id, body });
      toast.success("Draft saved");
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  async function sent() {
    try {
      // Persist unsaved edits first so the stored message matches what was actually sent.
      if (dirty) await update.mutateAsync({ id: message.id, body });
      await markSent.mutateAsync(jobContactId);
      toast.success("Marked as sent. Now awaiting a response.");
    } catch (err) {
      toast.error(errorMessage(err));
    }
  }

  return (
    <div className="space-y-3">
      <Textarea value={body} onChange={(e) => setBody(e.target.value)} rows={9} />
      <div className="flex flex-wrap gap-2">
        <Button variant="outline" size="sm" onClick={() => void copy()}>
          <Copy className="h-4 w-4" /> Copy
        </Button>
        <Button variant="outline" size="sm" disabled={!dirty || update.isPending} onClick={() => void save()}>
          {update.isPending && <Spinner />} Save draft
        </Button>
        {canMarkSent && (
          <Button size="sm" disabled={markSent.isPending || !body.trim()} onClick={() => void sent()}>
            {markSent.isPending ? <Spinner /> : <Check className="h-4 w-4" />} I sent it
          </Button>
        )}
      </div>
      <p className="text-xs text-muted-foreground">
        Copy the draft, send it to the contact on LinkedIn yourself, then press "I sent it". This app never sends messages.
      </p>
    </div>
  );
}

function FollowupCard({ jc }: { jc: JobContact }) {
  const messages = useMessages(jc.id);
  const templates = useTemplates();
  const generate = useGenerateFollowup();
  const [templateId, setTemplateId] = useState("");

  const draft = messages.data?.find((m) => m.status === "DRAFT");
  const sent = messages.data?.filter((m) => m.status === "SENT") ?? [];
  const canGenerate = jc.status === "CONNECTED" || jc.status === "MESSAGE_READY";

  async function run(regenerate: boolean) {
    try {
      await generate.mutateAsync({ id: jc.id, templateId: templateId || undefined, regenerate });
      toast.success(regenerate ? "Draft regenerated" : "Draft ready");
    } catch (err) {
      if (err instanceof ApiError && err.code === "followup_not_eligible") {
        toast.info(err.message);
      } else {
        toast.error(errorMessage(err));
      }
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Follow-up message</CardTitle>
        <CardDescription>A draft built from your template. Edit it freely before you send it yourself.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {messages.isLoading ? (
          <Skeleton className="h-24" />
        ) : (
          <>
            {draft && <DraftEditor message={draft} jobContactId={jc.id} canMarkSent={jc.status === "MESSAGE_READY"} />}

            {canGenerate && (
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                <Select
                  className="sm:w-56"
                  value={templateId}
                  onChange={(e) => setTemplateId(e.target.value)}
                  aria-label="Template"
                >
                  <option value="">Default template</option>
                  {templates.data?.data.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.name}
                    </option>
                  ))}
                </Select>
                {draft ? (
                  <Button variant="outline" disabled={generate.isPending} onClick={() => void run(true)}>
                    {generate.isPending ? <Spinner /> : <RefreshCw className="h-4 w-4" />} Regenerate draft
                  </Button>
                ) : (
                  <Button disabled={generate.isPending} onClick={() => void run(false)}>
                    {generate.isPending && <Spinner />} Generate draft
                  </Button>
                )}
              </div>
            )}

            {!draft && !canGenerate && sent.length === 0 && (
              <p className="text-sm text-muted-foreground">
                A follow-up draft can be generated once this contact is connected and your follow-up delay has passed.
              </p>
            )}

            {sent.length > 0 && (
              <div className="space-y-2">
                <p className="text-sm font-medium">Sent</p>
                {sent.map((m) => (
                  <div key={m.id} className="rounded-md border bg-muted/40 p-3 text-sm">
                    <p className="mb-1 text-xs text-muted-foreground">{formatDateTime(m.sent_at)}</p>
                    <p className="whitespace-pre-wrap">{m.body}</p>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}

function Timeline({ id }: { id: string }) {
  const { data, isLoading } = useJobContactEvents(id);
  return (
    <Card>
      <CardHeader>
        <CardTitle>History</CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <Skeleton className="h-20" />
        ) : (
          <ol className="relative space-y-4 border-l pl-5">
            {data?.map((ev) => (
              <li key={ev.id} className="relative">
                <span className="absolute -left-[26px] top-1.5 h-2.5 w-2.5 rounded-full bg-primary" />
                <p className="text-sm font-medium">
                  {ev.to_status
                    ? ev.from_status
                      ? `${statusLabel(ev.from_status)} → ${statusLabel(ev.to_status)}`
                      : `Added as ${statusLabel(ev.to_status)}`
                    : statusLabel(ev.event_type)}
                </p>
                <p className="text-xs text-muted-foreground">{formatDateTime(ev.created_at)}</p>
                {typeof ev.metadata?.note === "string" && <p className="mt-1 text-sm">{ev.metadata.note}</p>}
              </li>
            ))}
          </ol>
        )}
      </CardContent>
    </Card>
  );
}

export default function ReferralDetail() {
  const { id } = useParams();
  const navigate = useNavigate();
  const { data: jc, isLoading, error, refetch } = useJobContact(id);
  const del = useDeleteJobContact();
  const [target, setTarget] = useState<WorkflowStatus | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);

  if (error) {
    if (error instanceof ApiError && error.status === 404) {
      return <ErrorState error={new Error("This referral does not exist or was deleted.")} />;
    }
    return <ErrorState error={error} onRetry={() => void refetch()} />;
  }
  if (isLoading || !jc) return <Skeleton className="h-64" />;

  const dates: [string, string | null][] = [
    ["Invitation sent", jc.invitation_sent_at],
    ["Accepted", jc.accepted_at],
    ["Message sent", jc.message_sent_at],
    ["Follow-up due", jc.followup_due_at],
    ["Referral received", jc.referral_received_at],
  ];

  return (
    <>
      <Link to="/referrals" className="mb-3 inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground">
        <ArrowLeft className="h-4 w-4" /> Referrals
      </Link>
      <PageHeader
        title={jc.contact_name}
        description={`${jc.job_title} at ${jc.company_name}`}
        actions={
          <>
            <StatusBadge status={jc.status} />
            {jc.contact_linkedin_url && (
              <Button asChild variant="outline" size="sm">
                <a href={jc.contact_linkedin_url} target="_blank" rel="noopener noreferrer">
                  <ExternalLink className="h-4 w-4" /> LinkedIn
                </a>
              </Button>
            )}
            <Button variant="ghost" size="icon" aria-label="Delete referral" onClick={() => setConfirmDelete(true)}>
              <Trash2 className="h-4 w-4 text-destructive" />
            </Button>
          </>
        }
      />

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="space-y-6 lg:col-span-2">
          <Card>
            <CardHeader>
              <CardTitle>Next step</CardTitle>
              <CardDescription>Update the status when something happens on LinkedIn or by email.</CardDescription>
            </CardHeader>
            <CardContent>
              {jc.allowed_next_statuses.length === 0 ? (
                <p className="text-sm text-muted-foreground">This referral is closed. No further moves are possible.</p>
              ) : (
                <div className="flex flex-wrap gap-2">
                  {jc.allowed_next_statuses.map((s) => (
                    <Button key={s} variant={s === "CLOSED" || s === "DECLINED" ? "outline" : "default"} onClick={() => setTarget(s)}>
                      {statusLabel(s)}
                    </Button>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <FollowupCard jc={jc} />
          <NotesCard jc={jc} />
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Dates</CardTitle>
            </CardHeader>
            <CardContent>
              <dl className="space-y-2 text-sm">
                {dates.map(([label, value]) => (
                  <div key={label} className="flex justify-between gap-3">
                    <dt className="text-muted-foreground">{label}</dt>
                    <dd className="text-right">{formatDateTime(value)}</dd>
                  </div>
                ))}
              </dl>
            </CardContent>
          </Card>
          <Timeline id={jc.id} />
        </div>
      </div>

      <TransitionDialog jc={jc} to={target} onClose={() => setTarget(null)} />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title="Delete this referral?"
        description="The referral attempt, its history and its drafts will be deleted. The job and the contact stay."
        loading={del.isPending}
        onConfirm={async () => {
          try {
            await del.mutateAsync(jc.id);
            toast.success("Referral deleted");
            navigate("/referrals", { replace: true });
          } catch (err) {
            toast.error(errorMessage(err));
            setConfirmDelete(false);
          }
        }}
      />
    </>
  );
}
