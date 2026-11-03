import { useEffect, useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { toast } from "sonner";
import { useSaveSettings, useSettings } from "@/api/referrals";
import { useMe } from "@/api/crud";
import { errorMessage } from "@/lib/api";
import { emptyToNull } from "@/lib/format";
import { useAuth } from "@/hooks/useAuth";
import { useFieldErrors } from "@/hooks/useFieldErrors";
import { PageHeader } from "@/components/PageHeader";
import { ErrorState } from "@/components/ErrorState";
import { FormField } from "@/components/FormField";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";

export default function SettingsPage() {
  const { signOut } = useAuth();
  const me = useMe();
  const { data, isLoading, error, refetch } = useSettings();
  const save = useSaveSettings();
  const fe = useFieldErrors();
  const [hours, setHours] = useState("24");
  const [sender, setSender] = useState("");

  useEffect(() => {
    if (data) {
      setHours(String(data.followup_delay_hours));
      setSender(data.sender_name ?? "");
    }
  }, [data]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    fe.clear();
    try {
      await save.mutateAsync({ followup_delay_hours: Number(hours), sender_name: emptyToNull(sender) });
      toast.success("Settings saved");
    } catch (err) {
      if (!fe.capture(err)) toast.error(errorMessage(err));
    }
  }

  if (error) return <ErrorState error={error} onRetry={() => void refetch()} />;

  return (
    <>
      <PageHeader title="Settings" />
      <div className="max-w-xl space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>Follow-ups</CardTitle>
            <CardDescription>How long to wait after a contact accepts before a follow-up is due.</CardDescription>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-32" />
            ) : (
              <form onSubmit={submit} className="space-y-4">
                <FormField label="Follow-up delay (hours)" htmlFor="s-hours" error={fe.errors.followup_delay_hours} hint="Between 1 and 720.">
                  <Input id="s-hours" type="number" min={1} max={720} required value={hours} onChange={(e) => setHours(e.target.value)} />
                </FormField>
                <FormField label="Your name in messages" htmlFor="s-sender" error={fe.errors.sender_name} hint="Fills {{sender_name}} in templates.">
                  <Input id="s-sender" value={sender} onChange={(e) => setSender(e.target.value)} />
                </FormField>
                <Button type="submit" disabled={save.isPending}>
                  {save.isPending && <Spinner />} Save settings
                </Button>
              </form>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Account</CardTitle>
            <CardDescription>{me.data?.email ?? "Loading..."}</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2">
            <Button asChild variant="outline">
              <Link to="/settings/gmail">Gmail connection</Link>
            </Button>
            <Button variant="outline" onClick={() => void signOut()}>
              Sign out
            </Button>
          </CardContent>
        </Card>
      </div>
    </>
  );
}
