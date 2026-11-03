import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import {
  Building2,
  Briefcase,
  LayoutDashboard,
  LogOut,
  Mail,
  Menu,
  MessageSquareText,
  Send,
  Settings,
  Users,
  X,
  Handshake,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAuth } from "@/hooks/useAuth";
import { Button } from "@/components/ui/button";

const nav = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard, end: true },
  { to: "/referrals", label: "Referrals", icon: Handshake },
  { to: "/follow-ups", label: "Follow-ups", icon: Send },
  { to: "/jobs", label: "Jobs", icon: Briefcase },
  { to: "/companies", label: "Companies", icon: Building2 },
  { to: "/contacts", label: "Contacts", icon: Users },
  { to: "/templates", label: "Templates", icon: MessageSquareText },
  { to: "/settings/gmail", label: "Gmail", icon: Mail },
  { to: "/settings", label: "Settings", icon: Settings, end: true },
];

export function AppLayout() {
  const { user, signOut } = useAuth();
  const [open, setOpen] = useState(false);

  const links = (
    <nav className="flex flex-1 flex-col gap-1 p-3">
      {nav.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={to}
          to={to}
          end={end}
          onClick={() => setOpen(false)}
          className={({ isActive }) =>
            cn(
              "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              isActive ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:bg-accent hover:text-foreground",
            )
          }
        >
          <Icon className="h-4 w-4" />
          {label}
        </NavLink>
      ))}
    </nav>
  );

  const account = (
    <div className="border-t p-3">
      <p className="truncate px-3 pb-2 text-xs text-muted-foreground" title={user?.email ?? ""}>
        {user?.email}
      </p>
      <Button variant="ghost" size="sm" className="w-full justify-start" onClick={() => void signOut()}>
        <LogOut className="h-4 w-4" /> Sign out
      </Button>
    </div>
  );

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 hidden h-screen w-60 shrink-0 flex-col border-r bg-card md:flex">
        <div className="flex h-14 items-center px-6 text-base font-semibold">Referral Tracker</div>
        {links}
        {account}
      </aside>

      {open && (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/50" onClick={() => setOpen(false)} />
          <aside className="absolute left-0 top-0 flex h-full w-64 flex-col bg-card shadow-lg">
            <div className="flex h-14 items-center justify-between px-6 text-base font-semibold">
              Referral Tracker
              <Button variant="ghost" size="icon" onClick={() => setOpen(false)} aria-label="Close menu">
                <X className="h-4 w-4" />
              </Button>
            </div>
            {links}
            {account}
          </aside>
        </div>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center gap-2 border-b bg-card px-4 md:hidden">
          <Button variant="ghost" size="icon" onClick={() => setOpen(true)} aria-label="Open menu">
            <Menu className="h-5 w-5" />
          </Button>
          <span className="font-semibold">Referral Tracker</span>
        </header>
        <main className="mx-auto w-full max-w-6xl flex-1 p-4 md:p-8">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
