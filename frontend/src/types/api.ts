export type WorkflowStatus =
  | "ADDED"
  | "INVITATION_PENDING"
  | "CONNECTED"
  | "MESSAGE_READY"
  | "MESSAGE_SENT"
  | "AWAITING_RESPONSE"
  | "REFERRAL_RECEIVED"
  | "DECLINED"
  | "CLOSED";

export const WORKFLOW_STATUSES: WorkflowStatus[] = [
  "ADDED",
  "INVITATION_PENDING",
  "CONNECTED",
  "MESSAGE_READY",
  "MESSAGE_SENT",
  "AWAITING_RESPONSE",
  "REFERRAL_RECEIVED",
  "DECLINED",
  "CLOSED",
];

export interface Me {
  id: string;
  email: string;
  display_name: string | null;
  created_at: string;
  updated_at: string;
}

export interface Company {
  id: string;
  name: string;
  website: string | null;
  industry: string | null;
  location: string | null;
  notes: string | null;
  job_count: number;
  contact_count: number;
  created_at: string;
  updated_at: string;
}

export interface CompanyInput {
  name: string;
  website: string | null;
  industry: string | null;
  location: string | null;
  notes: string | null;
}

export type JobStatus = "ACTIVE" | "CLOSED";

export interface Job {
  id: string;
  company_id: string;
  company_name: string;
  title: string;
  job_url: string | null;
  location: string | null;
  status: JobStatus;
  notes: string | null;
  created_at: string;
  updated_at: string;
}

export interface JobInput {
  company_id: string;
  title: string;
  job_url: string | null;
  location: string | null;
  status: JobStatus;
  notes: string | null;
}

export interface Contact {
  id: string;
  company_id: string | null;
  company_name: string | null;
  full_name: string;
  linkedin_url: string | null;
  title: string | null;
  email: string | null;
  notes: string | null;
  created_at: string;
  updated_at: string;
}

export interface ContactInput {
  company_id: string | null;
  full_name: string;
  linkedin_url: string | null;
  title: string | null;
  email: string | null;
  notes: string | null;
}

export interface JobContact {
  id: string;
  job_id: string;
  job_title: string;
  company_id: string;
  company_name: string;
  contact_id: string;
  contact_name: string;
  contact_linkedin_url: string | null;
  status: WorkflowStatus;
  invitation_sent_at: string | null;
  accepted_at: string | null;
  message_sent_at: string | null;
  followup_due_at: string | null;
  referral_received_at: string | null;
  notes: string | null;
  created_at: string;
  updated_at: string;
  allowed_next_statuses: WorkflowStatus[];
}

export interface WorkflowEvent {
  id: string;
  event_type: string;
  from_status: WorkflowStatus | null;
  to_status: WorkflowStatus | null;
  metadata: Record<string, unknown> | null;
  created_at: string;
}

export interface TransitionInput {
  to_status: WorkflowStatus;
  occurred_at?: string;
  note?: string;
}

export interface Message {
  id: string;
  job_contact_id: string;
  template_id: string | null;
  kind: string;
  body: string;
  status: "DRAFT" | "SENT" | "SUPERSEDED";
  generated_at: string;
  sent_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface MarkSentResult {
  message: Message;
  job_contact_status: WorkflowStatus;
}

export interface Template {
  id: string;
  name: string;
  body: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface TemplateInput {
  name: string;
  body: string;
  is_default: boolean;
}

export interface AppSettings {
  followup_delay_hours: number;
  sender_name: string | null;
}

export interface DashboardActivity {
  id: string;
  job_contact_id: string;
  contact_name: string;
  event_type: string;
  from_status: WorkflowStatus | null;
  to_status: WorkflowStatus | null;
  created_at: string;
}

export interface Dashboard {
  companies: number;
  active_jobs: number;
  contacts: number;
  followups_due: number;
  job_contacts_by_status: Partial<Record<WorkflowStatus, number>>;
  recent_activity: DashboardActivity[];
}

export interface GmailSync {
  id?: string;
  status?: string;
  started_at?: string;
  finished_at?: string | null;
  processed?: number;
  matched?: number;
  unmatched?: number;
  [key: string]: unknown;
}

export interface GmailStatus {
  enabled: boolean;
  connected: boolean;
  email_address: string | null;
  status: string | null;
  last_synced_at: string | null;
  recent_syncs: GmailSync[];
}

export interface GmailSyncResult {
  sync_id: string;
  processed: number;
  matched: number;
  unmatched: number;
  ignored: number;
  failed: number;
  has_more: boolean;
}

export interface UnmatchedEmail {
  id: string;
  from_address: string;
  subject: string;
  received_at: string;
  extracted_name: string | null;
  extracted_profile_url: string | null;
  reason: "NO_CANDIDATE" | "AMBIGUOUS";
  status: "PENDING" | "RESOLVED" | "DISMISSED";
  resolved_at: string | null;
  created_at: string;
}
