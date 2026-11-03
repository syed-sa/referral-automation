import { Link } from "react-router-dom";
import { Briefcase, Building2, Send, Users } from "lucide-react";
import { useDashboard } from "@/api/crud";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { ErrorState } from "@/components/ErrorState";
import { EmptyState } from "@/components/EmptyState";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { statusLabel, timeAgo } from "@/lib/format";
import { WORKFLOW_STATUSES } from "@/types/api";

function Stat({ label, value, icon: Icon, to }: { label: string; value: number; icon: typeof Users; to: string }) {
  return (
    <Link to={to}>
      <Card className="transition-colors hover:bg-muted/40">
        <CardContent className="flex items-center justify-between p-5">
          <div>
            <p className="text-sm text-muted-foreground">{label}</p>
            <p className="mt-1 text-3xl font-semibold">{value}</p>
          </div>
          <Icon className="h-6 w-6 text-muted-foreground" />
        </CardContent>
      </Card>
    </Link>
  );
}

export default function Dashboard() {
  const { data, isLoading, error, refetch } = useDashboard();

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />;

  return (
    <>
      <PageHeader title="Dashboard" description="Where your referral outreach stands." />
      {isLoading || !data ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-24" />
          ))}
        </div>
      ) : (
        <div className="space-y-6">
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <Stat label="Companies" value={data.companies} icon={Building2} to="/companies" />
            <Stat label="Active jobs" value={data.active_jobs} icon={Briefcase} to="/jobs" />
            <Stat label="Contacts" value={data.contacts} icon={Users} to="/contacts" />
            <Stat label="Follow-ups due" value={data.followups_due} icon={Send} to="/follow-ups" />
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Referrals by status</CardTitle>
              </CardHeader>
              <CardContent className="space-y-2">
                {WORKFLOW_STATUSES.map((s) => (
                  <Link
                    key={s}
                    to={`/referrals?status=${s}`}
                    className="flex items-center justify-between rounded-md px-2 py-1.5 hover:bg-muted/60"
                  >
                    <StatusBadge status={s} />
                    <span className="text-sm font-medium">{data.job_contacts_by_status[s] ?? 0}</span>
                  </Link>
                ))}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Recent activity</CardTitle>
              </CardHeader>
              <CardContent>
                {data.recent_activity.length === 0 ? (
                  <EmptyState title="No activity yet" description="Link a contact to a job to start tracking." />
                ) : (
                  <ul className="space-y-3">
                    {data.recent_activity.map((a) => (
                      <li key={a.id} className="flex items-start justify-between gap-3 text-sm">
                        <div className="min-w-0">
                          <Link to={`/referrals/${a.job_contact_id}`} className="font-medium hover:underline">
                            {a.contact_name}
                          </Link>
                          <p className="text-muted-foreground">
                            {a.to_status
                              ? a.from_status
                                ? `${statusLabel(a.from_status)} → ${statusLabel(a.to_status)}`
                                : `Added as ${statusLabel(a.to_status)}`
                              : statusLabel(a.event_type)}
                          </p>
                        </div>
                        <span className="shrink-0 text-xs text-muted-foreground">{timeAgo(a.created_at)}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </>
  );
}
