const icons: Record<string, string> = {
  http_proxy: '🌐',
  postgres: '🐘',
  redis: '🔴',
  mailpit: '✉️',
  docker: '🐳',
  replay: '🔁',
  fault: '⚡',
  agent: '⚙️',
  minio: '📦',
  localstack: '☁️',
  stripe: '💳',
};

export function sourceIcon(source: string): string {
  return icons[source] ?? '●';
}

export function formatDuration(ms?: number): string {
  if (ms == null) return '—';
  if (ms < 1) return '<1ms';
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

export function relativeTime(iso: string): string {
  const d = new Date(iso);
  const now = Date.now();
  const diff = now - d.getTime();
  if (diff < 60_000) return `${Math.floor(diff / 1000)}s ago`;
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}m ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}h ago`;
  return d.toLocaleDateString();
}

export function shortId(id: string): string {
  return id.length > 16 ? id.slice(0, 16) : id;
}
