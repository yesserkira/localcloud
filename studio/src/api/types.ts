export interface TimelineEvent {
  id: string;
  runId: string;
  scenarioId?: string;
  replayRunId?: string;
  timestamp: string;
  source: string;
  service: string;
  action: string;
  summary: string;
  status: string;
  durationMs?: number;
  correlationId?: string;
  parentEventId?: string;
  request?: RequestData;
  response?: ResponseData;
  metadata?: Record<string, unknown>;
  rawPayload?: RawPayload;
  faults?: FaultAnnotation[];
}

export interface RequestData {
  method?: string;
  scheme?: string;
  host?: string;
  path?: string;
  query?: string;
  headers?: Record<string, string>;
  bodyPreview?: string;
  bodySha256?: string;
  bodyRedacted?: boolean;
  replayable?: boolean;
  replayWarning?: string;
}

export interface ResponseData {
  statusCode?: number;
  headers?: Record<string, string>;
  bodyPreview?: string;
  bodySha256?: string;
  bodyRedacted?: boolean;
}

export interface RawPayload {
  format: string;
  preview: string;
  sha256: string;
  byteSize: number;
  redacted: boolean;
}

export interface FaultAnnotation {
  ruleId: string;
  ruleName: string;
  kind: string;
  effect: string;
  appliedAt: string;
}

export interface Scenario {
  id: string;
  name: string;
  description: string;
  status: string;
  startedAt: string;
  stoppedAt?: string;
  eventCount: number;
  replayableCount: number;
  rootEventIds?: string[];
  tags?: string[];
  configSnapshotId?: string;
  redactionSummary?: Record<string, unknown>;
  createdBy: string;
  errorMessage?: string;
}

export interface ReplayRun {
  id: string;
  scenarioId: string;
  startedAt: string;
  finishedAt?: string;
  status: string;
  targetBaseUrl: string;
  requestCount: number;
  passedCount: number;
  failedCount: number;
  diffSummary?: Record<string, unknown>;
  createdBy: string;
  errorMessage?: string;
}

export interface FaultRule {
  id: string;
  name: string;
  enabled: boolean;
  kind: string;
  scope: string;
  match: FaultMatch;
  action: FaultAction;
  safety: FaultSafety;
  createdAt: string;
  updatedAt: string;
  hitCount: number;
  lastAppliedAt?: string;
}

export interface FaultMatch {
  source?: string;
  service?: string;
  method?: string;
  path?: string;
  pathPrefix?: string;
  host?: string;
  statusCode?: number;
  queue?: string;
  emailToContains?: string;
  headers?: Record<string, string>;
}

export interface FaultAction {
  statusCode?: number;
  bodyJson?: Record<string, unknown>;
  delayMs?: number;
  reason?: string;
}

export interface FaultSafety {
  maxHits?: number;
  expiresAfter?: string;
}

export interface ServiceHealth {
  service: string;
  type: string;
  status: string;
  endpoint: string;
  containerId?: string;
  lastCheckedAt: string;
  message?: string;
  metadata?: Record<string, unknown>;
}

export interface HealthResponse {
  status: string;
  version: string;
  runId: string;
  startedAt: string;
  database: string;
  liveStream: string;
}
