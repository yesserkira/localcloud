const BASE = '';

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    const msg = (body as { error?: { message?: string } })?.error?.message ?? res.statusText;
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

export const api = {
  getHealth: () => fetchJSON<import('./types').HealthResponse>('/api/health'),

  getEvents: (limit = 100, cursor = 0) =>
    fetchJSON<{ items: import('./types').TimelineEvent[]; nextCursor: string }>(
      `/api/events?limit=${limit}&cursor=${cursor}`,
    ),

  getEvent: (id: string) =>
    fetchJSON<import('./types').TimelineEvent>(`/api/events/${id}`),

  getScenarios: () =>
    fetchJSON<{ items: import('./types').Scenario[] }>('/api/scenarios'),

  getScenario: (id: string) =>
    fetchJSON<{
      scenario: import('./types').Scenario;
      events: import('./types').TimelineEvent[];
      replayRuns: import('./types').ReplayRun[];
    }>(`/api/scenarios/${id}`),

  startRecording: (name: string, description: string, tags: string[]) =>
    fetchJSON<import('./types').Scenario>('/api/scenarios/start', {
      method: 'POST',
      body: JSON.stringify({ name, description, tags }),
    }),

  stopRecording: () =>
    fetchJSON<import('./types').Scenario>('/api/scenarios/stop', {
      method: 'POST',
      body: '{}',
    }),

  startReplay: (scenarioId: string, baseUrl: string, opts: { skipUnsafe?: boolean; confirmUnsafe?: boolean }) =>
    fetchJSON<{
      runId: string; scenarioId: string;
      total: number; passed: number; failed: number; skipped: number;
      diffs: Array<{
        eventId: string; method: string; path: string;
        originalStatus: number; replayStatus: number;
        statusMatch: boolean; bodyDiffs?: string[];
        durationMs: number; error?: string;
      }>;
    }>(`/api/scenarios/${scenarioId}/replay`, {
      method: 'POST',
      body: JSON.stringify({ baseUrl, ...opts }),
    }),

  getReplayRun: (id: string) =>
    fetchJSON<{
      run: import('./types').ReplayRun;
      originalEvents: import('./types').TimelineEvent[];
      replayEvents: import('./types').TimelineEvent[];
    }>(`/api/replay-runs/${id}`),

  getFaultRules: () =>
    fetchJSON<{ items: import('./types').FaultRule[] }>('/api/fault-rules'),

  createFaultRule: (rule: Partial<import('./types').FaultRule>) =>
    fetchJSON<import('./types').FaultRule>('/api/fault-rules', {
      method: 'POST',
      body: JSON.stringify(rule),
    }),

  updateFaultRule: (id: string, patch: { enabled?: boolean }) =>
    fetchJSON<{ id: string; updated: boolean }>(`/api/fault-rules/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(patch),
    }),

  deleteFaultRule: (id: string) =>
    fetchJSON<{ id: string; deleted: boolean }>(`/api/fault-rules/${id}`, {
      method: 'DELETE',
    }),

  getServices: () =>
    fetchJSON<{ services: import('./types').ServiceHealth[] }>('/api/services'),
};
