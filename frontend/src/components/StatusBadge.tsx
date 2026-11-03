import { cn } from "@/lib/utils";
import { statusLabel } from "@/lib/format";

const colors: Record<string, string> = {
  ADDED: "bg-slate-100 text-slate-700",
  INVITATION_PENDING: "bg-amber-100 text-amber-800",
  CONNECTED: "bg-sky-100 text-sky-800",
  MESSAGE_READY: "bg-indigo-100 text-indigo-800",
  MESSAGE_SENT: "bg-violet-100 text-violet-800",
  AWAITING_RESPONSE: "bg-orange-100 text-orange-800",
  REFERRAL_RECEIVED: "bg-green-100 text-green-800",
  DECLINED: "bg-red-100 text-red-800",
  CLOSED: "bg-zinc-200 text-zinc-600",
  ACTIVE: "bg-green-100 text-green-800",
  DRAFT: "bg-amber-100 text-amber-800",
  SENT: "bg-green-100 text-green-800",
  SUPERSEDED: "bg-zinc-200 text-zinc-600",
  PENDING: "bg-amber-100 text-amber-800",
  RESOLVED: "bg-green-100 text-green-800",
  DISMISSED: "bg-zinc-200 text-zinc-600",
};

export function StatusBadge({ status, className }: { status: string; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium",
        colors[status] ?? "bg-slate-100 text-slate-700",
        className,
      )}
    >
      {statusLabel(status)}
    </span>
  );
}
