import { Route, Routes } from "react-router-dom";
import { AppLayout } from "@/components/AppLayout";
import { ProtectedRoute } from "@/components/ProtectedRoute";
import Login from "@/pages/Login";
import Dashboard from "@/pages/Dashboard";
import Companies from "@/pages/Companies";
import Jobs from "@/pages/Jobs";
import Contacts from "@/pages/Contacts";
import Referrals from "@/pages/Referrals";
import ReferralDetail from "@/pages/ReferralDetail";
import FollowUps from "@/pages/FollowUps";
import Templates from "@/pages/Templates";
import SettingsPage from "@/pages/Settings";
import GmailSettings from "@/pages/GmailSettings";
import NotFound from "@/pages/NotFound";

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<AppLayout />}>
          <Route index element={<Dashboard />} />
          <Route path="referrals" element={<Referrals />} />
          <Route path="referrals/:id" element={<ReferralDetail />} />
          <Route path="follow-ups" element={<FollowUps />} />
          <Route path="jobs" element={<Jobs />} />
          <Route path="companies" element={<Companies />} />
          <Route path="contacts" element={<Contacts />} />
          <Route path="templates" element={<Templates />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="settings/gmail" element={<GmailSettings />} />
          <Route path="*" element={<NotFound />} />
        </Route>
      </Route>
    </Routes>
  );
}
