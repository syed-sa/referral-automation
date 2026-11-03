import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { qk } from "./keys";
import type {
  Company,
  CompanyInput,
  Contact,
  ContactInput,
  Dashboard,
  Job,
  JobInput,
  Me,
} from "@/types/api";

export interface ListParams {
  page?: number;
  page_size?: number;
  q?: string;
  [key: string]: string | number | undefined;
}

// A factory for the three plain CRUD resources keeps the hooks identical in shape.
function resource<T, I>(path: string, key: readonly string[]) {
  return {
    useList: (params: ListParams = {}) =>
      useQuery({
        queryKey: [...key, "list", params],
        queryFn: () => api.page<T>(path, params),
        placeholderData: keepPreviousData,
      }),
    useGet: (id: string | undefined) =>
      useQuery({ queryKey: [...key, "detail", id], queryFn: () => api.get<T>(`${path}/${id}`), enabled: !!id }),
    useCreate: () => {
      const qc = useQueryClient();
      return useMutation({
        mutationFn: (input: I) => api.post<T>(path, input),
        onSuccess: () => {
          void qc.invalidateQueries({ queryKey: key });
          void qc.invalidateQueries({ queryKey: qk.dashboard });
        },
      });
    },
    useUpdate: () => {
      const qc = useQueryClient();
      return useMutation({
        mutationFn: ({ id, input }: { id: string; input: I }) => api.put<T>(`${path}/${id}`, input),
        onSuccess: () => {
          void qc.invalidateQueries({ queryKey: key });
          // Names are denormalized onto jobs, contacts and job contacts.
          void qc.invalidateQueries({ queryKey: qk.jobs });
          void qc.invalidateQueries({ queryKey: qk.contacts });
          void qc.invalidateQueries({ queryKey: qk.jobContacts });
        },
      });
    },
    useDelete: () => {
      const qc = useQueryClient();
      return useMutation({
        mutationFn: (id: string) => api.delete(`${path}/${id}`),
        onSuccess: () => {
          void qc.invalidateQueries({ queryKey: qk.companies });
          void qc.invalidateQueries({ queryKey: qk.jobs });
          void qc.invalidateQueries({ queryKey: qk.contacts });
          void qc.invalidateQueries({ queryKey: qk.jobContacts });
          void qc.invalidateQueries({ queryKey: qk.followups });
          void qc.invalidateQueries({ queryKey: qk.dashboard });
        },
      });
    },
  };
}

export const companiesApi = resource<Company, CompanyInput>("/companies", qk.companies);
export const jobsApi = resource<Job, JobInput>("/jobs", qk.jobs);
export const contactsApi = resource<Contact, ContactInput>("/contacts", qk.contacts);

export function useMe() {
  return useQuery({ queryKey: qk.me, queryFn: () => api.get<Me>("/me") });
}

export function useDashboard() {
  return useQuery({ queryKey: qk.dashboard, queryFn: () => api.get<Dashboard>("/dashboard") });
}
