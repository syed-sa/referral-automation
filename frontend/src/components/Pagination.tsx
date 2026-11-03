import { ChevronLeft, ChevronRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Pagination as P } from "@/lib/api";

export function Pagination({ pagination, onPage }: { pagination?: P; onPage: (page: number) => void }) {
  if (!pagination || pagination.total_pages <= 1) return null;
  return (
    <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
      <span>
        Page {pagination.page} of {pagination.total_pages} · {pagination.total} total
      </span>
      <div className="flex gap-2">
        <Button variant="outline" size="sm" disabled={pagination.page <= 1} onClick={() => onPage(pagination.page - 1)}>
          <ChevronLeft className="h-4 w-4" /> Prev
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={pagination.page >= pagination.total_pages}
          onClick={() => onPage(pagination.page + 1)}
        >
          Next <ChevronRight className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
