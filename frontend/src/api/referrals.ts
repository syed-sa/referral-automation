import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { qk } from "./keys";
import type { ListParams } from "./crud";
import type {
  AppSettings,
  JobContact,
  MarkSentResult,
  Message,
  Template,
  TemplateInput,
  TransitionInput,
  WorkflowEvent,
} from "@/types/api";

function useInvalidateWorkflow() {
  const qc = useQueryClient();
  return () => {
    void qc.invalidateQueries({ queryKey: qk.jobContacts });
    void qc.invalidateQueries({ queryKey: qk.followups });
    void qc.invalidateQueries({ queryKey: qk.dashboard });
    void qc.invalidateQueries({ queryKey: qk.companies });
  };
}

export function useJobContacts(params: ListParams = {}) {
  return useQuery({
    queryKey: [...qk.jobContacts, "list", params],
    queryFn: () => api.page<JobContact>("/job-contacts", params),
    placeholderData: keepPreviousData,
  });
}

export function useJobContact(id: string | undefined) {
  return useQuery({
    queryKey: [...qk.jobContacts, "detail", id],
    queryFn: () => api.get<JobContact>(`/job-contacts/${id}`),
    enabled: !!id,
  });
}

export function useJobContactEvents(id: string | undefined) {
  return useQuery({
    queryKey: [...qk.jobContacts, "events", id],
    queryFn: () => api.get<WorkflowEvent[]>(`/job-contacts/${id}/events`),
    enabled: !!id,
  });
}

export function useLinkContact() {
  const invalidate = useInvalidateWorkflow();
  return useMutation({
    mutationFn: ({ jobId, contactId, notes }: { jobId: string; contactId: string; notes: string | null }) =>
      api.post<JobContact>(`/jobs/${jobId}/contacts`, { contact_id: contactId, notes }),
    onSuccess: invalidate,
  });
}

export function useUpdateJobContactNotes() {
  const invalidate = useInvalidateWorkflow();
  return useMutation({
    mutationFn: ({ id, notes }: { id: string; notes: string | null }) =>
      api.patch<JobContact>(`/job-contacts/${id}`, { notes }),
    onSuccess: invalidate,
  });
}

export function useDeleteJobContact() {
  const invalidate = useInvalidateWorkflow();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/job-contacts/${id}`),
    onSuccess: invalidate,
  });
}

export function useTransition() {
  const invalidate = useInvalidateWorkflow();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: TransitionInput }) =>
      api.post<JobContact>(`/job-contacts/${id}/transitions`, input),
    onSuccess: invalidate,
  });
}

// Follow-ups

export function useDueFollowups(params: ListParams = {}) {
  return useQuery({
    queryKey: [...qk.followups, "due", params],
    queryFn: () => api.page<JobContact>("/followups/due", params),
    placeholderData: keepPreviousData,
  });
}

export function useMessages(jobContactId: string | undefined) {
  return useQuery({
    queryKey: [...qk.jobContacts, "messages", jobContactId],
    queryFn: () => api.get<Message[]>(`/job-contacts/${jobContactId}/messages`),
    enabled: !!jobContactId,
  });
}

export function useGenerateFollowup() {
  const invalidate = useInvalidateWorkflow();
  return useMutation({
    mutationFn: ({ id, templateId, regenerate }: { id: string; templateId?: string; regenerate?: boolean }) =>
      api.post<Message>(`/job-contacts/${id}/followup`, {
        ...(templateId ? { template_id: templateId } : {}),
        ...(regenerate ? { regenerate: true } : {}),
      }),
    onSuccess: invalidate,
  });
}

export function useUpdateMessage() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) => api.put<Message>(`/messages/${id}`, { body }),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qk.jobContacts }),
  });
}

export function useMarkFollowupSent() {
  const invalidate = useInvalidateWorkflow();
  return useMutation({
    mutationFn: (id: string) => api.post<MarkSentResult>(`/job-contacts/${id}/followup/mark-sent`),
    onSuccess: invalidate,
  });
}

// Templates and settings

export function useTemplates() {
  return useQuery({
    queryKey: qk.templates,
    queryFn: () => api.page<Template>("/templates", { page_size: 100 }),
  });
}

export function useSaveTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id?: string; input: TemplateInput }) =>
      id ? api.put<Template>(`/templates/${id}`, input) : api.post<Template>("/templates", input),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qk.templates }),
  });
}

export function useDeleteTemplate() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.delete(`/templates/${id}`),
    onSuccess: () => void qc.invalidateQueries({ queryKey: qk.templates }),
  });
}

export function useSettings() {
  return useQuery({ queryKey: qk.settings, queryFn: () => api.get<AppSettings>("/settings") });
}

export function useSaveSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AppSettings) => api.put<AppSettings>("/settings", input),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: qk.settings });
      void qc.invalidateQueries({ queryKey: qk.followups });
      void qc.invalidateQueries({ queryKey: qk.dashboard });
    },
  });
}
