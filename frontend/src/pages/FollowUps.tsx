import { useState } from "react";
import { Link } from "react-router-dom";
import { useDueFollowups } from "@/api/referrals";
import { formatDateTime } from "@/lib/format";
import { PageHeader } from "@/components/PageHeader";
import { Pagination } from "@/components/Pagination";
import { EmptyState } from "@/components/EmptyState";
import { ErrorState } from "@/components/ErrorState";
import { TableSkeleton } from "@/components/TableSkeleton";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";

export default function FollowUps() {
  const [page, setPage] = useState(1);
  const { data, isLoading, error, refetch } = useDueFollowups({ page, page_size: 20 });

  return (
    <>
      <PageHeader
        title="Follow-ups due"
        description="Contacts who accepted your invitation and are ready for a message. Nothing is sent automatically; you copy the draft and send it yourself."
      />
      {error ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : isLoading ? (
        <TableSkeleton />
      ) : data && data.data.length === 0 ? (
        <EmptyState title="Nothing due" description="Contacts appear here once they have been connected longer than your follow-up delay." />
      ) : (
        data && (
          <>
            <div className="rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Contact</TableHead>
                    <TableHead>Job</TableHead>
                    <TableHead className="hidden md:table-cell">Accepted</TableHead>
                    <TableHead className="w-32" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {data.data.map((jc) => (
                    <TableRow key={jc.id}>
                      <TableCell className="font-medium">{jc.contact_name}</TableCell>
                      <TableCell>
                        {jc.job_title}
                        <span className="block text-xs text-muted-foreground">{jc.company_name}</span>
                      </TableCell>
                      <TableCell className="hidden md:table-cell">{formatDateTime(jc.accepted_at)}</TableCell>
                      <TableCell className="text-right">
                        <Button asChild size="sm">
                          <Link to={`/referrals/${jc.id}`}>Open</Link>
                        </Button>
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
    </>
  );
}
