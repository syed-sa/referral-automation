import { useEffect, useState, type FormEvent } from "react";
import { ExternalLink, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { companiesApi, jobsApi } from "@/api/crud";
import { useLinkContact } from "@/api/referrals";
import { contactsApi } from "@/api/crud";
import { errorMessage } from "@/lib/api";
import { emptyToNull } from "@/lib/format";
import { useDebounce } from "@/hooks/useDebounce";
import { useFieldErrors } from "@/hooks/useFieldErrors";
import type { Job, JobInput, JobStatus } from "@/types/api";
import { PageHeader } from "@/components/PageHeader";
import { SearchInput } from "@/components/SearchInput";
import { Pagination } from "@/components/Pagination";
import { EmptyState } from "@/components/EmptyState";
import { ErrorState } from "@/components/ErrorState";
import { TableSkeleton } from "@/components/TableSkeleton";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FormField } from "@/components/FormField";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Spinner } from "@/components/ui/spinner";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function JobForm({ job, open, onOpenChange }: { job: Job | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const create = jobsApi.useCreate();
  const update = jobsApi.useUpdate();
  const companies = companiesApi.useList({ page_size: 100 });
  const fe = useFieldErrors();
  const [f, setF] = useState({ company_id: "", title: "", job_url: "", location: "", status: "ACTIVE" as JobStatus, notes: "" });

  useEffect(() => {
    if (open) {
      fe.clear();
      setF({
        company_id: job?.company_id ?? "",
        title: job?.title ?? "",
        job_url: job?.job_url ?? "",
        location: job?.location ?? "",
        status: job?.status ?? "ACTIVE",
        notes: job?.notes ?? "",
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, job]);

  const busy = create.isPending || update.isPending;

  async function submit(e: FormEvent) {
    e.preventDefault();
    fe.clear();
    const input: JobInput = {
      company_id: f.company_id,
      title: f.title.trim(),
      job_url: emptyToNull(f.job_url),
      location: emptyToNull(f.location),
      status: f.status,
      notes: emptyToNull(f.notes),
    };
    try {
      if (job) await update.mutateAsync({ id: job.id, input });
      else await create.mutateAsync(input);
      toast.success(job ? "Job updated" : "Job added");
      onOpenChange(false);
    } catch (err) {
      if (!fe.capture(err)) toast.error(errorMessage(err));
    }
  }

  const set = (k: keyof typeof f) => (e: { target: { value: string } }) => setF({ ...f, [k]: e.target.value });
  const noCompanies = companies.data && companies.data.data.length === 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{job ? "Edit job" : "Add job"}</DialogTitle>
        </DialogHeader>
        {noCompanies ? (
          <p className="text-sm text-muted-foreground">Add a company first; every job belongs to one.</p>
        ) : (
          <form onSubmit={submit} className="space-y-4">
            <FormField label="Company" htmlFor="j-company" error={fe.errors.company_id}>
              <Select id="j-company" required value={f.company_id} onChange={set("company_id")}>
                <option value="">Select a company...</option>
                {companies.data?.data.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </FormField>
            <FormField label="Title" htmlFor="j-title" error={fe.errors.title}>
              <Input id="j-title" required value={f.title} onChange={set("title")} />
            </FormField>
            <FormField label="Job URL" htmlFor="j-url" error={fe.errors.job_url}>
              <Input id="j-url" value={f.job_url} onChange={set("job_url")} />
            </FormField>
            <div className="grid gap-4 sm:grid-cols-2">
              <FormField label="Location" htmlFor="j-loc" error={fe.errors.location}>
                <Input id="j-loc" value={f.location} onChange={set("location")} />
              </FormField>
              <FormField label="Status" htmlFor="j-status" error={fe.errors.status}>
                <Select id="j-status" value={f.status} onChange={set("status")}>
                  <option value="ACTIVE">Active</option>
                  <option value="CLOSED">Closed</option>
                </Select>
              </FormField>
            </div>
            <FormField label="Notes" htmlFor="j-notes" error={fe.errors.notes}>
              <Textarea id="j-notes" value={f.notes} onChange={set("notes")} />
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
        )}
      </DialogContent>
    </Dialog>
  );
}

/** Link an existing contact to a job, which starts a referral attempt in status ADDED. */
function LinkContactDialog({ job, onClose }: { job: Job | null; onClose: () => void }) {
  const contacts = contactsApi.useList({ page_size: 100 });
  const link = useLinkContact();
  const fe = useFieldErrors();
  const [contactId, setContactId] = useState("");
  const [notes, setNotes] = useState("");

  useEffect(() => {
    if (job) {
      setContactId("");
      setNotes("");
      fe.clear();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [job]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!job) return;
    fe.clear();
    try {
      await link.mutateAsync({ jobId: job.id, contactId, notes: emptyToNull(notes) });
      toast.success("Contact linked. Track it under Referrals.");
      onClose();
    } catch (err) {
      if (!fe.capture(err)) toast.error(errorMessage(err));
    }
  }

  return (
    <Dialog open={!!job} onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Link a contact</DialogTitle>
          <DialogDescription>
            {job?.title} at {job?.company_name}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <FormField label="Contact" htmlFor="l-contact" error={fe.errors.contact_id}>
            <Select id="l-contact" required value={contactId} onChange={(e) => setContactId(e.target.value)}>
              <option value="">Select a contact...</option>
              {contacts.data?.data.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.full_name}
                  {c.company_name ? ` (${c.company_name})` : ""}
                </option>
              ))}
            </Select>
          </FormField>
          <FormField label="Notes" htmlFor="l-notes" error={fe.errors.notes}>
            <Textarea id="l-notes" value={notes} onChange={(e) => setNotes(e.target.value)} />
          </FormField>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={link.isPending || !contactId}>
              {link.isPending && <Spinner />} Link contact
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default function Jobs() {
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [companyId, setCompanyId] = useState("");
  const [page, setPage] = useState(1);
  const dq = useDebounce(q);
  useEffect(() => setPage(1), [dq, status, companyId]);

  const companies = companiesApi.useList({ page_size: 100 });
  const { data, isLoading, error, refetch } = jobsApi.useList({ q: dq, status, company_id: companyId, page, page_size: 20 });
  const del = jobsApi.useDelete();
  const [editing, setEditing] = useState<Job | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [deleting, setDeleting] = useState<Job | null>(null);
  const [linking, setLinking] = useState<Job | null>(null);

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await del.mutateAsync(deleting.id);
      toast.success("Job deleted");
    } catch (err) {
      toast.error(errorMessage(err));
    }
    setDeleting(null);
  }

  return (
    <>
      <PageHeader
        title="Jobs"
        actions={
          <Button
            onClick={() => {
              setEditing(null);
              setFormOpen(true);
            }}
          >
            <Plus className="h-4 w-4" /> Add job
          </Button>
        }
      />
      <div className="mb-4 flex flex-col gap-2 sm:flex-row">
        <SearchInput value={q} onChange={setQ} placeholder="Search jobs..." />
        <Select className="sm:w-44" value={status} onChange={(e) => setStatus(e.target.value)} aria-label="Filter by status">
          <option value="">All statuses</option>
          <option value="ACTIVE">Active</option>
          <option value="CLOSED">Closed</option>
        </Select>
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
        <EmptyState title="No jobs found" description="Add a job, then link contacts who can refer you." />
      ) : (
        data && (
          <>
            <div className="rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Title</TableHead>
                    <TableHead>Company</TableHead>
                    <TableHead className="hidden md:table-cell">Location</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="w-36" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.map((j) => (
                    <TableRow key={j.id}>
                      <TableCell className="font-medium">
                        <span className="inline-flex items-center gap-1.5">
                          {j.title}
                          {j.job_url && (
                            <a href={j.job_url} target="_blank" rel="noopener noreferrer" aria-label="Open job posting">
                              <ExternalLink className="h-3.5 w-3.5 text-muted-foreground" />
                            </a>
                          )}
                        </span>
                      </TableCell>
                      <TableCell>{j.company_name}</TableCell>
                      <TableCell className="hidden md:table-cell">{j.location ?? "-"}</TableCell>
                      <TableCell>
                        <StatusBadge status={j.status} />
                      </TableCell>
                      <TableCell>
                        <div className="flex justify-end gap-1">
                          <Button variant="outline" size="sm" onClick={() => setLinking(j)}>
                            Link contact
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Edit"
                            onClick={() => {
                              setEditing(j);
                              setFormOpen(true);
                            }}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" aria-label="Delete" onClick={() => setDeleting(j)}>
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

      <JobForm job={editing} open={formOpen} onOpenChange={setFormOpen} />
      <LinkContactDialog job={linking} onClose={() => setLinking(null)} />
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => !o && setDeleting(null)}
        title="Delete job?"
        description={`"${deleting?.title}" and every referral attempt linked to it will be deleted.`}
        loading={del.isPending}
        onConfirm={() => void confirmDelete()}
      />
    </>
  );
}
