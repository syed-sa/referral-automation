import { useEffect, useRef, useState, type FormEvent } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { useDeleteTemplate, useSaveTemplate, useTemplates } from "@/api/referrals";
import { errorMessage } from "@/lib/api";
import { useFieldErrors } from "@/hooks/useFieldErrors";
import type { Template } from "@/types/api";
import { PageHeader } from "@/components/PageHeader";
import { EmptyState } from "@/components/EmptyState";
import { ErrorState } from "@/components/ErrorState";
import { TableSkeleton } from "@/components/TableSkeleton";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FormField } from "@/components/FormField";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";

const PLACEHOLDERS = ["contact_name", "contact_first_name", "company", "job_title", "job_url", "sender_name"];

function TemplateForm({ template, open, onOpenChange }: { template: Template | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const save = useSaveTemplate();
  const fe = useFieldErrors();
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const [name, setName] = useState("");
  const [body, setBody] = useState("");
  const [isDefault, setIsDefault] = useState(false);

  useEffect(() => {
    if (open) {
      fe.clear();
      setName(template?.name ?? "");
      setBody(template?.body ?? "");
      setIsDefault(template?.is_default ?? false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, template]);

  function insert(key: string) {
    const token = `{{${key}}}`;
    const el = bodyRef.current;
    if (!el) {
      setBody(body + token);
      return;
    }
    const start = el.selectionStart ?? body.length;
    const end = el.selectionEnd ?? body.length;
    setBody(body.slice(0, start) + token + body.slice(end));
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(start + token.length, start + token.length);
    });
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    fe.clear();
    try {
      await save.mutateAsync({ id: template?.id, input: { name: name.trim(), body, is_default: isDefault } });
      toast.success("Template saved");
      onOpenChange(false);
    } catch (err) {
      if (!fe.capture(err)) toast.error(errorMessage(err));
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{template ? "Edit template" : "New template"}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <FormField label="Name" htmlFor="tp-name" error={fe.errors.name}>
            <Input id="tp-name" required value={name} onChange={(e) => setName(e.target.value)} />
          </FormField>
          <FormField label="Message" htmlFor="tp-body" error={fe.errors.body}>
            <Textarea id="tp-body" ref={bodyRef} required rows={9} value={body} onChange={(e) => setBody(e.target.value)} />
          </FormField>
          <div className="flex flex-wrap gap-1.5">
            <span className="mr-1 self-center text-xs text-muted-foreground">Insert:</span>
            {PLACEHOLDERS.map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => insert(p)}
                className="rounded-md border bg-muted px-2 py-0.5 font-mono text-xs hover:bg-accent"
              >
                {`{{${p}}}`}
              </button>
            ))}
          </div>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} />
            Use as my default template
          </label>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={save.isPending}>
              {save.isPending && <Spinner />} Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function Templates() {
  const { data, isLoading, error, refetch } = useTemplates();
  const del = useDeleteTemplate();
  const [editing, setEditing] = useState<Template | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [deleting, setDeleting] = useState<Template | null>(null);

  return (
    <>
      <PageHeader
        title="Message templates"
        description="Used to draft follow-ups. Without a default template, a built-in message is used."
        actions={
          <Button
            onClick={() => {
              setEditing(null);
              setFormOpen(true);
            }}
          >
            <Plus className="h-4 w-4" /> New template
          </Button>
        }
      />

      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading ? (
        <TableSkeleton rows={3} />
      ) : data && data.data.length === 0 ? (
        <EmptyState title="No templates yet" description="Create one to control the wording of your follow-ups." />
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {data?.data.map((t) => (
            <Card key={t.id}>
              <CardHeader className="flex-row items-start justify-between space-y-0">
                <CardTitle className="flex items-center gap-2">
                  {t.name}
                  {t.is_default && <Badge variant="secondary">Default</Badge>}
                </CardTitle>
                <div className="flex gap-1">
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Edit"
                    onClick={() => {
                      setEditing(t);
                      setFormOpen(true);
                    }}
                  >
                    <Pencil className="h-4 w-4" />
                  </Button>
                  <Button variant="ghost" size="icon" aria-label="Delete" onClick={() => setDeleting(t)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <p className="line-clamp-6 whitespace-pre-wrap text-sm text-muted-foreground">{t.body}</p>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <TemplateForm template={editing} open={formOpen} onOpenChange={setFormOpen} />
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => !o && setDeleting(null)}
        title="Delete template?"
        description={`"${deleting?.name}" will be deleted. Drafts already made from it are kept.`}
        loading={del.isPending}
        onConfirm={async () => {
          if (!deleting) return;
          try {
            await del.mutateAsync(deleting.id);
            toast.success("Template deleted");
          } catch (err) {
            toast.error(errorMessage(err));
          }
          setDeleting(null);
        }}
      />
    </>
  );
}
