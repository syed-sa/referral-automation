import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useJobContacts } from "@/api/referrals";
import { formatDate, statusLabel } from "@/lib/format";
import { WORKFLOW_STATUSES } from "@/types/api";
import { PageHeader } from "@/components/PageHeader";
import { Pagination } from "@/components/Pagination";
import { EmptyState } from "@/components/EmptyState";
import { ErrorState } from "@/components/ErrorState";
import { TableSkeleton } from "@/components/TableSkeleton";
import { StatusBadge } from "@/components/StatusBadge";
import { Select } from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export default function Referrals() {
  const [params, setParams] = useSearchParams();
  const status = params.get("status") ?? "";
  const [page, setPage] = useState(1);
  useEffect(() => setPage(1), [status]);

  const { data, isLoading, error, refetch } = useJobContacts({ status, page, page_size: 20 });

  return (
    <>
      <PageHeader
        title="Referrals"
        description="Every contact you are asking for a referral, and where each one stands."
        actions={
          <Button asChild variant="outline">
            <Link to="/jobs">Link a contact from Jobs</Link>
          </Button>
        }
      />
      <div className="mb-4">
        <Select
          className="sm:w-60"
          value={status}
          aria-label="Filter by status"
          onChange={(e) => {
            const next = new URLSearchParams(params);
            if (e.target.value) next.set("status", e.target.value);
            else next.delete("status");
            setParams(next, { replace: true });
          }}
        >
          <option value="">All statuses</option>
          {WORKFLOW_STATUSES.map((s) => (
            <option key={s} value={s}>
              {statusLabel(s)}
            </option>
          ))}
        </Select>
      </div>

      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading ? (
        <TableSkeleton />
      ) : data && data.data.length === 0 ? (
        <EmptyState
          title="No referrals here"
          description="Open Jobs and link a contact to a job to start a referral attempt."
        />
      ) : (
        data && (
          <>
            <div className="rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Contact</TableHead>
                    <TableHead>Job</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="hidden md:table-cell">Updated</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.map((jc) => (
                    <TableRow key={jc.id}>
                      <TableCell className="font-medium">
                        <Link to={`/referrals/${jc.id}`} className="hover:underline">
                          {jc.contact_name}
                        </Link>
                      </TableCell>
                      <TableCell>
                        {jc.job_title}
                        <span className="block text-xs text-muted-foreground">{jc.company_name}</span>
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={jc.status} />
                      </TableCell>
                      <TableCell className="hidden md:table-cell">{formatDate(jc.updated_at)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <Pagination pagination={data.pagination} onPage={setPage} />
          </>
        )
      )}
    </>
  );
}
