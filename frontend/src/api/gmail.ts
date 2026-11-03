import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { qk } from "./keys";
import type { ListParams } from "./crud";
import type { GmailStatus, GmailSyncResult, UnmatchedEmail } from "@/types/api";

export function useGmailStatus() {
  return useQuery({ queryKey: [...qk.gmail, "status"], queryFn: () => api.get<GmailStatus>("/gmail/status") });
}

export function useGmailConnect() {
  return useMutation({
    mutationFn: () => api.post<{ url: string }>("/gmail/connect"),
    // Hand the browser to Google. The backend callback redirects back to /settings/gmail.
    onSuccess: ({ url }) => {
      window.location.assign(url);
    },
  });
}

function useInvalidateGmail() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: qk.gmail });
    void qc.invalidateQueries({ queryKey: qk.jobContacts });
    void qc.invalidateQueries({ queryKey: qk.followups });
    void qc.invalidateQueries({ queryKey: qk.dashboard });
  };
}

export function useGmailSync() {
  const invalidate = useInvalidateGmail();
  return useMutation({
    mutationFn: () => api.post<GmailSyncResult>("/gmail/sync"),
    onSuccess: invalidate,
    onError: invalidate,
  });
}

export function useGmailDisconnect() {
  const invalidate = useInvalidateGmail();
  return useMutation({
    mutationFn: () => api.delete("/gmail/connection"),
    onSuccess: invalidate,
  });
}

export function useUnmatched(params: ListParams = {}) {
  return useQuery({
    queryKey: [...qk.gmail, "unmatched", params],
    queryFn: () => api.page<UnmatchedEmail>("/gmail/unmatched", params),
    placeholderData: keepPreviousData,
  });
}

export function useResolveUnmatched() {
  const invalidate = useInvalidateGmail();
  return useMutation({
    mutationFn: ({ id, contactId }: { id: string; contactId: string }) =>
      api.post(`/gmail/unmatched/${id}/resolve`, { contact_id: contactId }),
    onSuccess: invalidate,
  });
}

export function useDismissUnmatched() {
  const invalidate = useInvalidateGmail();
  return useMutation({
    mutationFn: (id: string) => api.post(`/gmail/unmatched/${id}/dismiss`),
    onSuccess: invalidate,
  });
}
