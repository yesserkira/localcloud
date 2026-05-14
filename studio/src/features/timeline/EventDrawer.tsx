import { useState } from 'react';
import type { TimelineEvent } from '../../api/types';
import { StatusBadge } from '../../components/StatusBadge';
import { JsonView } from '../../components/JsonView';
import { sourceIcon, formatDuration, shortId } from '../../components/utils';

interface Props {
  event: TimelineEvent;
  onClose: () => void;
}

type Tab = 'overview' | 'request' | 'response' | 'metadata' | 'raw';

export function EventDrawer({ event, onClose }: Props) {
  const [tab, setTab] = useState<Tab>('overview');

  const tabs: Tab[] = ['overview'];
  if (event.request) tabs.push('request');
  if (event.response) tabs.push('response');
  if (event.metadata) tabs.push('metadata');
  tabs.push('raw');

  return (
    <div className="drawer-overlay">
      <div className="drawer-header">
        <span>
          {sourceIcon(event.source)} {event.action}
        </span>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>

      <div className="tabs">
        {tabs.map((t) => (
          <button key={t} className={`tab ${tab === t ? 'active' : ''}`} onClick={() => setTab(t)}>
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>

      <div className="drawer-body">
        {tab === 'overview' && <OverviewTab event={event} />}
        {tab === 'request' && event.request && <RequestTab req={event.request} />}
        {tab === 'response' && event.response && <ResponseTab res={event.response} />}
        {tab === 'metadata' && event.metadata && <JsonView data={event.metadata} />}
        {tab === 'raw' && <JsonView data={event} />}
      </div>
    </div>
  );
}

function OverviewTab({ event }: { event: TimelineEvent }) {
  return (
    <div className="kv-grid">
      <span className="kv-label">ID</span>
      <span className="kv-value mono">{event.id}</span>

      <span className="kv-label">Source</span>
      <span className="kv-value">{sourceIcon(event.source)} {event.source}</span>

      <span className="kv-label">Service</span>
      <span className="kv-value">{event.service}</span>

      <span className="kv-label">Action</span>
      <span className="kv-value mono">{event.action}</span>

      <span className="kv-label">Status</span>
      <span className="kv-value"><StatusBadge status={event.status} /></span>

      <span className="kv-label">Timestamp</span>
      <span className="kv-value mono">{new Date(event.timestamp).toISOString()}</span>

      <span className="kv-label">Duration</span>
      <span className="kv-value">{formatDuration(event.durationMs)}</span>

      <span className="kv-label">Summary</span>
      <span className="kv-value">{event.summary}</span>

      {event.correlationId && <>
        <span className="kv-label">Correlation</span>
        <span className="kv-value mono">{shortId(event.correlationId)}</span>
      </>}

      {event.scenarioId && <>
        <span className="kv-label">Scenario</span>
        <span className="kv-value mono">{shortId(event.scenarioId)}</span>
      </>}

      {event.replayRunId && <>
        <span className="kv-label">Replay Run</span>
        <span className="kv-value mono">{shortId(event.replayRunId)}</span>
      </>}

      {event.faults && event.faults.length > 0 && <>
        <span className="kv-label">Faults</span>
        <span className="kv-value">
          {event.faults.map((f) => (
            <span key={f.ruleId} className="badge faulted" style={{ marginRight: 4 }}>
              {f.ruleName}: {f.effect}
            </span>
          ))}
        </span>
      </>}

      <span className="kv-label">Run ID</span>
      <span className="kv-value mono">{shortId(event.runId)}</span>
    </div>
  );
}

function RequestTab({ req }: { req: NonNullable<TimelineEvent['request']> }) {
  return (
    <div>
      <div className="kv-grid" style={{ marginBottom: 12 }}>
        <span className="kv-label">Method</span>
        <span className="kv-value mono">{req.method}</span>

        <span className="kv-label">Path</span>
        <span className="kv-value mono">{req.path}</span>

        {req.host && <>
          <span className="kv-label">Host</span>
          <span className="kv-value">{req.host}</span>
        </>}

        {req.query && <>
          <span className="kv-label">Query</span>
          <span className="kv-value mono">{req.query}</span>
        </>}

        <span className="kv-label">Replayable</span>
        <span className="kv-value">{req.replayable ? '✓ Yes' : '✗ No'}</span>
      </div>

      {req.headers && Object.keys(req.headers).length > 0 && (
        <>
          <h4 style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 6 }}>Headers</h4>
          <div className="kv-grid" style={{ marginBottom: 12 }}>
            {Object.entries(req.headers).map(([k, v]) => (
              <span key={k}>
                <span className="kv-label">{k}</span>
                <span className="kv-value mono" style={{ color: v === '[REDACTED]' ? 'var(--amber)' : undefined }}>
                  {v}
                </span>
              </span>
            ))}
          </div>
        </>
      )}

      {req.bodyPreview && (
        <>
          <h4 style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 6 }}>
            Body {req.bodyRedacted && <span className="badge warning">Redacted</span>}
          </h4>
          <pre className="json-view">{req.bodyPreview}</pre>
        </>
      )}
    </div>
  );
}

function ResponseTab({ res }: { res: NonNullable<TimelineEvent['response']> }) {
  return (
    <div>
      <div className="kv-grid" style={{ marginBottom: 12 }}>
        <span className="kv-label">Status</span>
        <span className="kv-value mono">{res.statusCode}</span>
      </div>

      {res.headers && Object.keys(res.headers).length > 0 && (
        <>
          <h4 style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 6 }}>Headers</h4>
          <div className="kv-grid" style={{ marginBottom: 12 }}>
            {Object.entries(res.headers).map(([k, v]) => (
              <span key={k}>
                <span className="kv-label">{k}</span>
                <span className="kv-value mono">{v}</span>
              </span>
            ))}
          </div>
        </>
      )}

      {res.bodyPreview && (
        <>
          <h4 style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 6 }}>
            Body {res.bodyRedacted && <span className="badge warning">Redacted</span>}
          </h4>
          <pre className="json-view">{res.bodyPreview}</pre>
        </>
      )}
    </div>
  );
}
