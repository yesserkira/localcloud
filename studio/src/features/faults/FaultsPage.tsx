import { useState } from 'react';
import { useFaultRules, useCreateFaultRule, useToggleFaultRule, useDeleteFaultRule } from '../../api/queries';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import type { FaultRule } from '../../api/types';

const KINDS = [
  { value: 'delay_response', label: 'Delay Response' },
  { value: 'force_http_status', label: 'Force HTTP Status' },
  { value: 'drop_outbound_request', label: 'Drop Outbound' },
  { value: 'mutate_json_response', label: 'Mutate JSON' },
  { value: 'simulate_timeout', label: 'Simulate Timeout' },
  { value: 'block_email_delivery', label: 'Block Email' },
] as const;

export function FaultsPage() {
  const { data, isLoading } = useFaultRules();
  const createRule = useCreateFaultRule();
  const toggleRule = useToggleFaultRule();
  const deleteRule = useDeleteFaultRule();
  const [showForm, setShowForm] = useState(false);

  const rules = data?.items ?? [];
  const enabledRules = rules.filter((r) => r.enabled);

  return (
    <div>
      {enabledRules.length > 0 && (
        <div className="banner warning">
          <span>⚡ Fault injection active: {enabledRules.length} rule{enabledRules.length > 1 ? 's' : ''} enabled</span>
          <button
            className="btn danger"
            style={{ marginLeft: 'auto' }}
            onClick={() => enabledRules.forEach((r) => toggleRule.mutate({ id: r.id, enabled: false }))}
          >
            Disable all
          </button>
        </div>
      )}

      <div className="page-header">
        <h2>Fault Rules</h2>
        <button className="btn primary" onClick={() => setShowForm(true)}>
          New Rule
        </button>
      </div>

      {showForm && (
        <CreateFaultForm
          onSubmit={(rule) => createRule.mutate(rule, { onSuccess: () => setShowForm(false) })}
          onCancel={() => setShowForm(false)}
          isPending={createRule.isPending}
          error={createRule.error as Error | null}
        />
      )}

      {isLoading ? (
        <div className="empty-state"><p>Loading fault rules...</p></div>
      ) : rules.length === 0 ? (
        <EmptyState
          message="No fault rules configured. Create a rule to test how your app handles failures."
          action={
            <pre style={{ color: 'var(--text-muted)', fontSize: 12 }}>
              localcloud fault create --name db-500 --kind force_http_status --status-code 500 --path-prefix /signup
            </pre>
          }
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Kind</th>
                <th>Scope</th>
                <th>Match</th>
                <th>Enabled</th>
                <th>Hits</th>
                <th>Safety</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <FaultRow
                  key={rule.id}
                  rule={rule}
                  onToggle={() => toggleRule.mutate({ id: rule.id, enabled: !rule.enabled })}
                  onDelete={() => { if (confirm(`Delete rule "${rule.name}"?`)) deleteRule.mutate(rule.id); }}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function FaultRow({ rule, onToggle, onDelete }: {
  rule: FaultRule;
  onToggle: () => void;
  onDelete: () => void;
}) {
  const matchParts: string[] = [];
  if (rule.match.service) matchParts.push(`svc=${rule.match.service}`);
  if (rule.match.method) matchParts.push(rule.match.method);
  if (rule.match.pathPrefix) matchParts.push(`${rule.match.pathPrefix}*`);
  if (rule.match.path) matchParts.push(rule.match.path);
  const matchStr = matchParts.length > 0 ? matchParts.join(' ') : '*';

  const safetyParts: string[] = [];
  if (rule.safety.maxHits) safetyParts.push(`max=${rule.safety.maxHits}`);
  if (rule.safety.expiresAfter) safetyParts.push(`exp=${rule.safety.expiresAfter}`);

  return (
    <tr>
      <td><strong>{rule.name}</strong></td>
      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{rule.kind}</td>
      <td><StatusBadge status={rule.scope} /></td>
      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{matchStr}</td>
      <td>
        <button
          className={`btn ${rule.enabled ? 'danger' : ''}`}
          onClick={(e) => { e.stopPropagation(); onToggle(); }}
          style={{ fontSize: 11, padding: '2px 8px' }}
        >
          {rule.enabled ? 'ON' : 'OFF'}
        </button>
      </td>
      <td>{rule.hitCount}</td>
      <td style={{ fontSize: 11, color: 'var(--text-muted)' }}>
        {safetyParts.length > 0 ? safetyParts.join(', ') : '—'}
      </td>
      <td>
        <button className="btn" onClick={(e) => { e.stopPropagation(); onDelete(); }} style={{ fontSize: 11, padding: '2px 8px' }}>
          Delete
        </button>
      </td>
    </tr>
  );
}

function CreateFaultForm({ onSubmit, onCancel, isPending, error }: {
  onSubmit: (rule: Partial<FaultRule>) => void;
  onCancel: () => void;
  isPending: boolean;
  error: Error | null;
}) {
  const [name, setName] = useState('');
  const [kind, setKind] = useState('force_http_status');
  const [scope, setScope] = useState('both');
  const [service, setService] = useState('');
  const [method, setMethod] = useState('');
  const [pathPrefix, setPathPrefix] = useState('');
  const [statusCode, setStatusCode] = useState(500);
  const [delayMs, setDelayMs] = useState(1000);
  const [reason, setReason] = useState('');
  const [maxHits, setMaxHits] = useState(100);
  const [expiresAfter, setExpiresAfter] = useState('1h');

  function handleSubmit() {
    const rule: Partial<FaultRule> = {
      name,
      kind,
      scope,
      enabled: true,
      match: {
        ...(service && { service }),
        ...(method && { method }),
        ...(pathPrefix && { pathPrefix }),
      },
      action: {
        ...(kind === 'force_http_status' && { statusCode }),
        ...((kind === 'delay_response' || kind === 'simulate_timeout') && { delayMs }),
        ...(reason && { reason }),
      },
      safety: {
        ...(maxHits > 0 && { maxHits }),
        ...(expiresAfter && { expiresAfter }),
      },
    };
    onSubmit(rule);
  }

  return (
    <div style={{ background: 'var(--bg-secondary)', padding: 16, borderRadius: 6, marginBottom: 16, border: '1px solid var(--border)' }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
        <div className="form-group">
          <label>Rule Name</label>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="signup-500" />
        </div>
        <div className="form-group">
          <label>Kind</label>
          <select value={kind} onChange={(e) => setKind(e.target.value)}>
            {KINDS.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
          </select>
        </div>
        <div className="form-group">
          <label>Scope</label>
          <select value={scope} onChange={(e) => setScope(e.target.value)}>
            <option value="both">Both</option>
            <option value="live">Live</option>
            <option value="replay">Replay</option>
          </select>
        </div>
        <div className="form-group">
          <label>Service</label>
          <input value={service} onChange={(e) => setService(e.target.value)} placeholder="api" />
        </div>
        <div className="form-group">
          <label>Method</label>
          <input value={method} onChange={(e) => setMethod(e.target.value)} placeholder="POST" />
        </div>
        <div className="form-group">
          <label>Path Prefix</label>
          <input value={pathPrefix} onChange={(e) => setPathPrefix(e.target.value)} placeholder="/signup" />
        </div>
        {kind === 'force_http_status' && (
          <div className="form-group">
            <label>Status Code</label>
            <input type="number" value={statusCode} onChange={(e) => setStatusCode(Number(e.target.value))} />
          </div>
        )}
        {(kind === 'delay_response' || kind === 'simulate_timeout') && (
          <div className="form-group">
            <label>Delay (ms)</label>
            <input type="number" value={delayMs} onChange={(e) => setDelayMs(Number(e.target.value))} />
          </div>
        )}
        <div className="form-group">
          <label>Reason</label>
          <input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="Injected fault" />
        </div>
        <div className="form-group">
          <label>Max Hits (safety)</label>
          <input type="number" value={maxHits} onChange={(e) => setMaxHits(Number(e.target.value))} />
        </div>
        <div className="form-group">
          <label>Expires After</label>
          <input value={expiresAfter} onChange={(e) => setExpiresAfter(e.target.value)} placeholder="1h" />
        </div>
      </div>
      <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
        <button className="btn primary" onClick={handleSubmit} disabled={!name || isPending}>
          Create Rule
        </button>
        <button className="btn" onClick={onCancel}>Cancel</button>
      </div>
      {error && <p style={{ color: 'var(--red)', marginTop: 8 }}>{error.message}</p>}
    </div>
  );
}
