import { useParams } from 'react-router-dom';
import { useReplayRun } from '../../api/queries';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import { sourceIcon, formatDuration, shortId } from '../../components/utils';

export function ReplayRunsPage() {
  const { id } = useParams<{ id: string }>();
  const { data, isLoading, error } = useReplayRun(id ?? null);

  if (!id) return <EmptyState message="No replay run selected." />;
  if (isLoading) return <div className="empty-state"><p>Loading replay run...</p></div>;
  if (error) return <div className="empty-state"><p>Error: {(error as Error).message}</p></div>;
  if (!data) return <EmptyState message="Replay run not found." />;

  const { run, originalEvents, replayEvents } = data;

  return (
    <div>
      <div className="page-header">
        <h2>Replay Run</h2>
        <StatusBadge status={run.status} />
      </div>

      <div className="kv-grid" style={{ marginBottom: 16 }}>
        <span className="kv-label">Run ID</span>
        <span className="kv-value mono">{run.id}</span>

        <span className="kv-label">Scenario</span>
        <span className="kv-value mono">{shortId(run.scenarioId)}</span>

        <span className="kv-label">Target</span>
        <span className="kv-value">{run.targetBaseUrl}</span>

        <span className="kv-label">Requests</span>
        <span className="kv-value">{run.requestCount}</span>

        <span className="kv-label">Passed</span>
        <span className="kv-value" style={{ color: 'var(--green)' }}>{run.passedCount}</span>

        <span className="kv-label">Failed</span>
        <span className="kv-value" style={{ color: run.failedCount > 0 ? 'var(--red)' : undefined }}>{run.failedCount}</span>

        <span className="kv-label">Started</span>
        <span className="kv-value">{new Date(run.startedAt).toLocaleString()}</span>

        {run.finishedAt && <>
          <span className="kv-label">Finished</span>
          <span className="kv-value">{new Date(run.finishedAt).toLocaleString()}</span>
        </>}

        {run.errorMessage && <>
          <span className="kv-label">Error</span>
          <span className="kv-value" style={{ color: 'var(--red)' }}>{run.errorMessage}</span>
        </>}
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
        <div>
          <h3 style={{ fontSize: 13, marginBottom: 8, color: 'var(--text-secondary)' }}>
            Original Events ({originalEvents?.length ?? 0})
          </h3>
          <EventTable events={originalEvents ?? []} />
        </div>
        <div>
          <h3 style={{ fontSize: 13, marginBottom: 8, color: 'var(--text-secondary)' }}>
            Replay Events ({replayEvents?.length ?? 0})
          </h3>
          <EventTable events={replayEvents ?? []} />
        </div>
      </div>
    </div>
  );
}

function EventTable({ events }: { events: import('../../api/types').TimelineEvent[] }) {
  if (events.length === 0) {
    return <p style={{ color: 'var(--text-muted)', fontSize: 12 }}>No events.</p>;
  }

  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th></th>
            <th>Action</th>
            <th>Service</th>
            <th>Status</th>
            <th>Duration</th>
          </tr>
        </thead>
        <tbody>
          {events.map((e) => (
            <tr key={e.id}>
              <td style={{ fontSize: 14 }}>{sourceIcon(e.source)}</td>
              <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{e.action}</td>
              <td>{e.service}</td>
              <td><StatusBadge status={e.status} /></td>
              <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{formatDuration(e.durationMs)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
