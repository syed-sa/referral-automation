import { useEffect, useState, type FormEvent } from "react";
import { ExternalLink, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { companiesApi, contactsApi } from "@/api/crud";
import { errorMessage } from "@/lib/api";
import { emptyToNull } from "@/lib/format";
import { useDebounce } from "@/hooks/useDebounce";
import { useFieldErrors } from "@/hooks/useFieldErrors";
import type { Contact, ContactInput } from "@/types/api";
import { PageHeader } from "@/components/PageHeader";
import { SearchInput } from "@/components/SearchInput";
import { Pagination } from "@/components/Pagination";
import { EmptyState } from "@/components/EmptyState";
import { ErrorState } from "@/components/ErrorState";
import { TableSkeleton } from "@/components/TableSkeleton";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FormField } from "@/components/FormField";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Spinner } from "@/components/ui/spinner";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function ContactForm({ contact, open, onOpenChange }: { contact: Contact | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const create = contactsApi.useCreate();
  const update = contactsApi.useUpdate();
  const companies = companiesApi.useList({ page_size: 100 });
  const fe = useFieldErrors();
  const [f, setF] = useState({ company_id: "", full_name: "", linkedin_url: "", title: "", email: "", notes: "" });

  useEffect(() => {
    if (open) {
      fe.clear();
      setF({
        company_id: contact?.company_id ?? "",
        full_name: contact?.full_name ?? "",
        linkedin_url: contact?.linkedin_url ?? "",
        title: contact?.title ?? "",
        email: contact?.email ?? "",
        notes: contact?.notes ?? "",
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, contact]);

  const busy = create.isPending || update.isPending;

  async function submit(e: FormEvent) {
    e.preventDefault();
    fe.clear();
    const input: ContactInput = {
      company_id: emptyToNull(f.company_id),
      full_name: f.full_name.trim(),
      linkedin_url: emptyToNull(f.linkedin_url),
      title: emptyToNull(f.title),
      email: emptyToNull(f.email),
      notes: emptyToNull(f.notes),
    };
    try {
      if (contact) await update.mutateAsync({ id: contact.id, input });
      else await create.mutateAsync(input);
      toast.success(contact ? "Contact updated" : "Contact added");
      onOpenChange(false);
    } catch (err) {
      if (!fe.capture(err)) toast.error(errorMessage(err));
    }
  }

  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{contact ? "Edit contact" : "Add contact"}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <FormField label="Full name" htmlFor="ct-name" error={fe.errors.full_name}>
            <Input id="ct-name" required value={f.full_name} onChange={set("full_name")} />
          </FormField>
          <FormField label="Company" htmlFor="ct-company" error={fe.errors.company_id}>
            <Select id="ct-company" value={f.company_id} onChange={set("company_id")}>
              <option value="">No company</option>
              {companies.data?.data.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </Select>
          </FormField>
          <FormField label="LinkedIn URL" htmlFor="ct-li" error={fe.errors.linkedin_url} hint="https://www.linkedin.com/in/...">
            <Input id="ct-li" value={f.linkedin_url} onChange={set("linkedin_url")} />
          </FormField>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Title" htmlFor="ct-title" error={fe.errors.title}>
              <Input id="ct-title" value={f.title} onChange={set("title")} />
            </FormField>
            <FormField label="Email" htmlFor="ct-email" error={fe.errors.email}>
              <Input id="ct-email" type="email" value={f.email} onChange={set("email")} />
            </FormField>
          </div>
          <FormField label="Notes" htmlFor="ct-notes" error={fe.errors.notes}>
            <Textarea id="ct-notes" value={f.notes} onChange={set("notes")} />
          </FormField>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              {busy && <Spinner />} Save
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function Contacts() {
  const [q, setQ] = useState("");
  const [companyId, setCompanyId] = useState("");
  const [page, setPage] = useState(1);
  const dq = useDebounce(q);
  useEffect(() => setPage(1), [dq, companyId]);

  const companies = companiesApi.useList({ page_size: 100 });
  const { data, isLoading, error, refetch } = contactsApi.useList({ q: dq, company_id: companyId, page, page_size: 20 });
  const del = contactsApi.useDelete();
  const [editing, setEditing] = useState<Contact | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [deleting, setDeleting] = useState<Contact | null>(null);

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await del.mutateAsync(deleting.id);
      toast.success("Contact deleted");
    } catch (err) {
      toast.error(errorMessage(err));
    }
    setDeleting(null);
  }

  return (
    <>
      <PageHeader
        title="Contacts"
        description="People you can ask for referrals. Add LinkedIn URLs by hand; nothing is scraped."
        actions={
          <Button
            onClick={() => {
              setEditing(null);
              setFormOpen(true);
            }}
          >
            <Plus className="h-4 w-4" /> Add contact
          </Button>
        }
      />
      <div className="mb-4 flex flex-col gap-2 sm:flex-row">
        <SearchInput value={q} onChange={setQ} placeholder="Search contacts..." />
        <Select className="sm:w-52" value={companyId} onChange={(e) => setCompanyId(e.target.value)} aria-label="Filter by company">
          <option value="">All companies</option>
          {companies.data?.data.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </Select>
      </div>

      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading ? (
        <TableSkeleton />
      ) : data && data.data.length === 0 ? (
        <EmptyState title="No contacts found" description="Add people who work at the companies you are targeting." />
      ) : (
        data && (
          <>
            <div className="rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Company</TableHead>
                    <TableHead className="hidden md:table-cell">Title</TableHead>
                    <TableHead className="hidden lg:table-cell">Email</TableHead>
                    <TableHead className="w-24" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.map((c) => (
                    <TableRow key={c.id}>
                      <TableCell className="font-medium">
                        <span className="inline-flex items-center gap-1.5">
                          {c.full_name}
                          {c.linkedin_url && (
                            <a href={c.linkedin_url} target="_blank" rel="noopener noreferrer" aria-label="Open LinkedIn profile">
                              <ExternalLink className="h-3.5 w-3.5 text-muted-foreground" />
                            </a>
                          )}
                        </span>
                      </TableCell>
                      <TableCell>{c.company_name ?? "-"}</TableCell>
                      <TableCell className="hidden md:table-cell">{c.title ?? "-"}</TableCell>
                      <TableCell className="hidden lg:table-cell">{c.email ?? "-"}</TableCell>
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Edit"
                            onClick={() => {
                              setEditing(c);
                              setFormOpen(true);
                            }}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" aria-label="Delete" onClick={() => setDeleting(c)}>
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pagination pagination={data.pagination} onPage={setPage} />
          </>
        )
      )}

      <ContactForm contact={editing} open={formOpen} onOpenChange={setFormOpen} />
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => !o && setDeleting(null)}
        title="Delete contact?"
        description={`"${deleting?.full_name}" and their referral attempts will be deleted.`}
        loading={del.isPending}
        onConfirm={() => void confirmDelete()}
      />
    </>
  );
}
