import { useServices } from '../../api/queries';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import { relativeTime } from '../../components/utils';

export function ServicesPage() {
  const { data, isLoading, error } = useServices();

  const services = data?.services ?? [];

  if (error) {
    return <div className="empty-state"><p>Failed to load services: {(error as Error).message}</p></div>;
  }

  return (
    <div>
      <div className="page-header">
        <h2>Services</h2>
      </div>

      {isLoading ? (
        <div className="empty-state"><p>Loading services...</p></div>
      ) : services.length === 0 ? (
        <EmptyState
          message="No services configured. Add services to localcloud.yml or run localcloud init --example demo-saas."
        />
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Service</th>
                <th>Type</th>
                <th>Status</th>
                <th>Endpoint</th>
                <th>Container</th>
                <th>Last Checked</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              {services.map((s) => (
                <tr key={s.service}>
                  <td><strong>{s.service}</strong></td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{s.type}</td>
                  <td><StatusBadge status={s.status} /></td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11 }}>{s.endpoint || '—'}</td>
                  <td style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-muted)' }}>
                    {s.containerId ? s.containerId.slice(0, 12) : '—'}
                  </td>
                  <td style={{ color: 'var(--text-muted)' }}>{relativeTime(s.lastCheckedAt)}</td>
                  <td style={{ maxWidth: 200 }}>{s.message || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
