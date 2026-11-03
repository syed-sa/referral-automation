import { useEffect, useState, type FormEvent } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";
import { companiesApi } from "@/api/crud";
import { errorMessage } from "@/lib/api";
import { emptyToNull } from "@/lib/format";
import { useDebounce } from "@/hooks/useDebounce";
import { useFieldErrors } from "@/hooks/useFieldErrors";
import type { Company, CompanyInput } from "@/types/api";
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
import { Textarea } from "@/components/ui/textarea";
import { Spinner } from "@/components/ui/spinner";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

function CompanyForm({ company, open, onOpenChange }: { company: Company | null; open: boolean; onOpenChange: (o: boolean) => void }) {
  const create = companiesApi.useCreate();
  const update = companiesApi.useUpdate();
  const fe = useFieldErrors();
  const [f, setF] = useState({ name: "", website: "", industry: "", location: "", notes: "" });

  useEffect(() => {
    if (open) {
      fe.clear();
      setF({
        name: company?.name ?? "",
        website: company?.website ?? "",
        industry: company?.industry ?? "",
        location: company?.location ?? "",
        notes: company?.notes ?? "",
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, company]);

  const busy = create.isPending || update.isPending;

  async function submit(e: FormEvent) {
    e.preventDefault();
    fe.clear();
    const input: CompanyInput = {
      name: f.name.trim(),
      website: emptyToNull(f.website),
      industry: emptyToNull(f.industry),
      location: emptyToNull(f.location),
      notes: emptyToNull(f.notes),
    };
    try {
      if (company) await update.mutateAsync({ id: company.id, input });
      else await create.mutateAsync(input);
      toast.success(company ? "Company updated" : "Company added");
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
          <DialogTitle>{company ? "Edit company" : "Add company"}</DialogTitle>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <FormField label="Name" htmlFor="c-name" error={fe.errors.name}>
            <Input id="c-name" required maxLength={200} value={f.name} onChange={set("name")} />
          </FormField>
          <FormField label="Website" htmlFor="c-web" error={fe.errors.website} hint="https://example.com">
            <Input id="c-web" value={f.website} onChange={set("website")} />
          </FormField>
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label="Industry" htmlFor="c-ind" error={fe.errors.industry}>
              <Input id="c-ind" value={f.industry} onChange={set("industry")} />
            </FormField>
            <FormField label="Location" htmlFor="c-loc" error={fe.errors.location}>
              <Input id="c-loc" value={f.location} onChange={set("location")} />
            </FormField>
          </div>
          <FormField label="Notes" htmlFor="c-notes" error={fe.errors.notes}>
            <Textarea id="c-notes" value={f.notes} onChange={set("notes")} />
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

export default function Companies() {
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const dq = useDebounce(q);
  useEffect(() => setPage(1), [dq]);

  const { data, isLoading, error, refetch } = companiesApi.useList({ q: dq, page, page_size: 20 });
  const del = companiesApi.useDelete();
  const [editing, setEditing] = useState<Company | null>(null);
  const [formOpen, setFormOpen] = useState(false);
  const [deleting, setDeleting] = useState<Company | null>(null);

  async function confirmDelete() {
    if (!deleting) return;
    try {
      await del.mutateAsync(deleting.id);
      toast.success("Company deleted");
      setDeleting(null);
    } catch (err) {
      toast.error(errorMessage(err));
      setDeleting(null);
    }
  }

  return (
    <>
      <PageHeader
        title="Companies"
        actions={
          <Button
            onClick={() => {
              setEditing(null);
              setFormOpen(true);
            }}
          >
            <Plus className="h-4 w-4" /> Add company
          </Button>
        }
      />
      <div className="mb-4">
        <SearchInput value={q} onChange={setQ} placeholder="Search companies..." />
      </div>

      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading ? (
        <TableSkeleton />
      ) : data && data.data.length === 0 ? (
        <EmptyState title={dq ? "No matching companies" : "No companies yet"} description="Add the companies you want referrals at." />
      ) : (
        data && (
          <>
            <div className="rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead className="hidden md:table-cell">Industry</TableHead>
                    <TableHead className="hidden md:table-cell">Location</TableHead>
                    <TableHead>Jobs</TableHead>
                    <TableHead>Contacts</TableHead>
                    <TableHead className="w-24" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.map((c) => (
                    <TableRow key={c.id}>
                      <TableCell className="font-medium">
                        {c.website ? (
                          <a href={c.website} target="_blank" rel="noopener noreferrer" className="hover:underline">
                            {c.name}
                          </a>
                        ) : (
                          c.name
                        )}
                      </TableCell>
                      <TableCell className="hidden md:table-cell">{c.industry ?? "-"}</TableCell>
                      <TableCell className="hidden md:table-cell">{c.location ?? "-"}</TableCell>
                      <TableCell>{c.job_count}</TableCell>
                      <TableCell>{c.contact_count}</TableCell>
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

      <CompanyForm company={editing} open={formOpen} onOpenChange={setFormOpen} />
      <ConfirmDialog
        open={!!deleting}
        onOpenChange={(o) => !o && setDeleting(null)}
        title="Delete company?"
        description={`"${deleting?.name}" will be deleted. A company that still has jobs or contacts cannot be deleted.`}
        loading={del.isPending}
        onConfirm={() => void confirmDelete()}
      />
    </>
  );
}
