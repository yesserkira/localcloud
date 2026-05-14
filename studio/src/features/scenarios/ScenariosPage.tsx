import { useState } from 'react';
import { useScenarios, useScenario, useStartRecording, useStopRecording, useStartReplay } from '../../api/queries';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import { relativeTime, shortId } from '../../components/utils';
import { useNavigate } from 'react-router-dom';

export function ScenariosPage() {
  const { data, isLoading } = useScenarios();
  const startRec = useStartRecording();
  const stopRec = useStopRecording();
  const startReplay = useStartReplay();
  const navigate = useNavigate();

  const [showRecordForm, setShowRecordForm] = useState(false);
  const [showReplayForm, setShowReplayForm] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [recName, setRecName] = useState('');
  const [recDesc, setRecDesc] = useState('');
  const [recTags, setRecTags] = useState('');
  const [replayUrl, setReplayUrl] = useState('http://localhost:3000');
  const [replaySkip, setReplaySkip] = useState(false);
  const [replayConfirm, setReplayConfirm] = useState(false);

  const scenarios = data?.items ?? [];
  const activeRecording = scenarios.find((s) => s.status === 'recording');

  function handleRecord() {
    startRec.mutate(
      { name: recName, description: recDesc, tags: recTags.split(',').map((t) => t.trim()).filter(Boolean) },
      { onSuccess: () => { setShowRecordForm(false); setRecName(''); setRecDesc(''); setRecTags(''); } },
    );
  }

  function handleReplay(scenarioId: string) {
    startReplay.mutate(
      { scenarioId, baseUrl: replayUrl, skipUnsafe: replaySkip, confirmUnsafe: replayConfirm },
      { onSuccess: (result) => { setShowReplayForm(null); navigate(`/replay/${result.runId}`); } },
    );
  }

  return (
    <div>
      {activeRecording && (
        <div className="banner recording">
          <span className="badge recording">REC</span>
          <span>Recording: <strong>{activeRecording.name}</strong></span>
          <span style={{ marginLeft: 'auto' }}>{activeRecording.eventCount} events</span>
          <button className="btn danger" onClick={() => stopRec.mutate()} disabled={stopRec.isPending}>
            Stop
          </button>
        </div>
      )}

      <div className="page-header">
        <h2>Scenarios</h2>
        <div style={{ display: 'flex', gap: 8 }}>
          {!activeRecording && (
            <button className="btn primary" onClick={() => setShowRecordForm(true)}>
              Record
            </button>
          )}
        </div>
      </div>

      {showRecordForm && (
        <div style={{ background: 'var(--bg-secondary)', padding: 16, borderRadius: 6, marginBottom: 16, border: '1px solid var(--border)' }}>
          <div className="form-group">
            <label>Scenario Name</label>
            <input value={recName} onChange={(e) => setRecName(e.target.value)} placeholder="signup-flow" />
          </div>
          <div className="form-group">
            <label>Description</label>
            <input value={recDesc} onChange={(e) => setRecDesc(e.target.value)} placeholder="Full signup with email" />
          </div>
          <div className="form-group">
            <label>Tags (comma-separated)</label>
            <input value={recTags} onChange={(e) => setRecTags(e.target.value)} placeholder="signup, email" />
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button className="btn primary" onClick={handleRecord} disabled={!recName || startRec.isPending}>
              Start Recording
            </button>
            <button className="btn" onClick={() => setShowRecordForm(false)}>Cancel</button>
          </div>
          {startRec.error && <p style={{ color: 'var(--red)', marginTop: 8 }}>{(startRec.error as Error).message}</p>}
        </div>
      )}

      {isLoading ? (
        <div className="empty-state"><p>Loading scenarios...</p></div>
      ) : scenarios.length === 0 ? (
        <EmptyState
          message="Record your first scenario to capture a full backend flow."
          action={
            <pre style={{ color: 'var(--text-muted)', fontSize: 12 }}>
              localcloud record --name signup-flow
            </pre>
          }
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Events</th>
                <th>Replayable</th>
                <th>Tags</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {scenarios.map((s) => (
                <tr key={s.id} className={selectedId === s.id ? 'selected' : ''} onClick={() => setSelectedId(s.id === selectedId ? null : s.id)}>
                  <td><strong>{s.name}</strong></td>
                  <td><StatusBadge status={s.status} /></td>
                  <td>{s.eventCount}</td>
                  <td>{s.replayableCount}</td>
                  <td>{s.tags?.join(', ') ?? ''}</td>
                  <td style={{ color: 'var(--text-muted)' }}>{relativeTime(s.startedAt)}</td>
                  <td onClick={(e) => e.stopPropagation()}>
                    {s.status === 'completed' && (
                      <>
                        <button className="btn" onClick={() => setShowReplayForm(s.id)} style={{ marginRight: 4 }}>
                          Replay
                        </button>
                        <a
                          className="btn"
                          href={`/api/scenarios/${s.id}/export`}
                          target="_blank"
                          rel="noreferrer"
                        >
                          Export
                        </a>
                      </>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showReplayForm && (
        <ReplayDialog
          scenarioId={showReplayForm}
          baseUrl={replayUrl}
          setBaseUrl={setReplayUrl}
          skipUnsafe={replaySkip}
          setSkipUnsafe={setReplaySkip}
          confirmUnsafe={replayConfirm}
          setConfirmUnsafe={setReplayConfirm}
          onSubmit={() => handleReplay(showReplayForm)}
          onCancel={() => setShowReplayForm(null)}
          isPending={startReplay.isPending}
          error={startReplay.error as Error | null}
        />
      )}

      {selectedId && <ScenarioDetail id={selectedId} onClose={() => setSelectedId(null)} />}
    </div>
  );
}

function ReplayDialog({
  scenarioId: _scenarioId, baseUrl, setBaseUrl, skipUnsafe, setSkipUnsafe,
  confirmUnsafe, setConfirmUnsafe, onSubmit, onCancel, isPending, error,
}: {
  scenarioId: string; baseUrl: string; setBaseUrl: (v: string) => void;
  skipUnsafe: boolean; setSkipUnsafe: (v: boolean) => void;
  confirmUnsafe: boolean; setConfirmUnsafe: (v: boolean) => void;
  onSubmit: () => void; onCancel: () => void;
  isPending: boolean; error: Error | null;
}) {
  return (
    <div style={{ position: 'fixed', top: 0, left: 0, right: 0, bottom: 0, background: 'rgba(0,0,0,0.5)', display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 200 }}>
      <div style={{ background: 'var(--bg-secondary)', padding: 24, borderRadius: 8, width: 400, border: '1px solid var(--border)' }}>
        <h3 style={{ fontSize: 14, marginBottom: 16 }}>Replay Scenario</h3>
        <div className="form-group">
          <label>Target Base URL</label>
          <input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} />
        </div>
        <div className="form-group">
          <label>
            <input type="checkbox" checked={skipUnsafe} onChange={(e) => setSkipUnsafe(e.target.checked)} />
            {' '}Skip unsafe methods (POST, PUT, DELETE)
          </label>
        </div>
        <div className="form-group">
          <label>
            <input type="checkbox" checked={confirmUnsafe} onChange={(e) => setConfirmUnsafe(e.target.checked)} />
            {' '}Confirm unsafe methods (allow all)
          </label>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="btn primary" onClick={onSubmit} disabled={!baseUrl || isPending}>
            Start Replay
          </button>
          <button className="btn" onClick={onCancel}>Cancel</button>
        </div>
        {error && <p style={{ color: 'var(--red)', marginTop: 8 }}>{error.message}</p>}
      </div>
    </div>
  );
}

function ScenarioDetail({ id, onClose }: { id: string; onClose: () => void }) {
  const { data } = useScenario(id);
  if (!data) return null;

  const { scenario, events, replayRuns } = data;
  return (
    <div className="drawer-overlay">
      <div className="drawer-header">
        <span><strong>{scenario.name}</strong> <StatusBadge status={scenario.status} /></span>
        <button className="drawer-close" onClick={onClose}>✕</button>
      </div>
      <div className="drawer-body">
        <div className="kv-grid" style={{ marginBottom: 16 }}>
          <span className="kv-label">ID</span>
          <span className="kv-value mono">{scenario.id}</span>
          <span className="kv-label">Events</span>
          <span className="kv-value">{scenario.eventCount}</span>
          <span className="kv-label">Replayable</span>
          <span className="kv-value">{scenario.replayableCount}</span>
          <span className="kv-label">Started</span>
          <span className="kv-value">{new Date(scenario.startedAt).toLocaleString()}</span>
          {scenario.stoppedAt && <>
            <span className="kv-label">Stopped</span>
            <span className="kv-value">{new Date(scenario.stoppedAt).toLocaleString()}</span>
          </>}
          {scenario.description && <>
            <span className="kv-label">Description</span>
            <span className="kv-value">{scenario.description}</span>
          </>}
        </div>

        {replayRuns && replayRuns.length > 0 && (
          <>
            <h4 style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8 }}>Replay Runs</h4>
            <table>
              <thead><tr><th>ID</th><th>Status</th><th>Passed</th><th>Failed</th></tr></thead>
              <tbody>
                {replayRuns.map((r) => (
                  <tr key={r.id}>
                    <td className="mono">{shortId(r.id)}</td>
                    <td><StatusBadge status={r.status} /></td>
                    <td>{r.passedCount}</td>
                    <td>{r.failedCount}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}

        {events && events.length > 0 && (
          <>
            <h4 style={{ fontSize: 12, color: 'var(--text-secondary)', margin: '16px 0 8px' }}>Events ({events.length})</h4>
            <table>
              <thead><tr><th>Action</th><th>Service</th><th>Status</th></tr></thead>
              <tbody>
                {events.slice(0, 50).map((e) => (
                  <tr key={e.id}>
                    <td className="mono">{e.action}</td>
                    <td>{e.service}</td>
                    <td><StatusBadge status={e.status} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </div>
    </div>
  );
}
