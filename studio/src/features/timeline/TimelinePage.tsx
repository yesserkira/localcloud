import { useState, useMemo } from 'react';
import { useEvents } from '../../api/queries';
import type { TimelineEvent } from '../../api/types';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import { sourceIcon, formatDuration, relativeTime } from '../../components/utils';
import { EventDrawer } from './EventDrawer';

const SOURCES = ['', 'http_proxy', 'postgres', 'redis', 'mailpit', 'docker', 'replay', 'fault'] as const;
const STATUSES = ['', 'ok', 'error', 'warning', 'pending', 'faulted'] as const;

export function TimelinePage() {
  const { data, isLoading, error } = useEvents();
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [search, setSearch] = useState('');
  const [sourceFilter, setSourceFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [serviceFilter, setServiceFilter] = useState('');

  const events = data?.items ?? [];

  const services = useMemo(() => {
    const set = new Set<string>();
    for (const e of events) set.add(e.service);
    return Array.from(set).sort();
  }, [events]);

  const filtered = useMemo(() => {
    return events.filter((e) => {
      if (sourceFilter && e.source !== sourceFilter) return false;
      if (statusFilter && e.status !== statusFilter) return false;
      if (serviceFilter && e.service !== serviceFilter) return false;
      if (search) {
        const q = search.toLowerCase();
        const haystack = `${e.summary} ${e.action} ${e.service} ${e.id}`.toLowerCase();
        if (!haystack.includes(q)) return false;
      }
      return true;
    });
  }, [events, sourceFilter, statusFilter, serviceFilter, search]);

  const selectedEvent = useMemo(
    () => events.find((e) => e.id === selectedId) ?? null,
    [events, selectedId],
  );

  if (error) {
    return (
      <div className="empty-state">
        <p>Failed to load events: {(error as Error).message}</p>
        <button className="btn" onClick={() => window.location.reload()}>Retry</button>
      </div>
    );
  }

  return (
    <div>
      <div className="page-header">
        <h2>Timeline</h2>
      </div>

      <div className="filter-bar">
        <input
          type="text"
          placeholder="Search events..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{ width: 200 }}
        />
        <select value={sourceFilter} onChange={(e) => setSourceFilter(e.target.value)}>
          <option value="">All sources</option>
          {SOURCES.filter(Boolean).map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
          <option value="">All statuses</option>
          {STATUSES.filter(Boolean).map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        <select value={serviceFilter} onChange={(e) => setServiceFilter(e.target.value)}>
          <option value="">All services</option>
          {services.map((s) => (
            <option key={s} value={s}>{s}</option>
          ))}
        </select>
        {(search || sourceFilter || statusFilter || serviceFilter) && (
          <button className="btn" onClick={() => { setSearch(''); setSourceFilter(''); setStatusFilter(''); setServiceFilter(''); }}>
            Clear filters
          </button>
        )}
      </div>

      {isLoading ? (
        <div className="empty-state"><p>Loading events...</p></div>
      ) : filtered.length === 0 ? (
        <EmptyState
          message={events.length === 0
            ? 'No events yet. Start the demo app or send traffic through the configured proxy.'
            : 'No events match filters.'}
          action={events.length > 0 ? (
            <button className="btn" onClick={() => { setSearch(''); setSourceFilter(''); setStatusFilter(''); setServiceFilter(''); }}>
              Clear filters
            </button>
          ) : undefined}
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Time</th>
                <th></th>
                <th>Service</th>
                <th>Action</th>
                <th>Summary</th>
                <th>Status</th>
                <th>Duration</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((e) => (
                <EventRow
                  key={e.id}
                  event={e}
                  selected={e.id === selectedId}
                  onClick={() => setSelectedId(e.id === selectedId ? null : e.id)}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}

      {selectedEvent && (
        <EventDrawer event={selectedEvent} onClose={() => setSelectedId(null)} />
      )}
    </div>
  );
}

function EventRow({ event, selected, onClick }: {
  event: TimelineEvent;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <tr className={selected ? 'selected' : ''} onClick={onClick}>
      <td style={{ color: 'var(--text-muted)', fontFamily: 'var(--font-mono)', fontSize: 11 }}>
        {relativeTime(event.timestamp)}
      </td>
      <td style={{ fontSize: 16 }}>{sourceIcon(event.source)}</td>
      <td>{event.service}</td>
      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{event.action}</td>
      <td style={{ maxWidth: 300 }}>{event.summary}</td>
      <td><StatusBadge status={event.status} /></td>
      <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>
        {formatDuration(event.durationMs)}
      </td>
    </tr>
  );
}
