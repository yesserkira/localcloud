interface Props {
  status: string;
}

const classMap: Record<string, string> = {
  ok: 'ok', completed: 'ok', passed: 'ok', running: 'pending',
  error: 'error', failed: 'error',
  warning: 'warning', partial: 'warning',
  pending: 'pending', recording: 'recording',
  faulted: 'faulted', blocked: 'error',
};

export function StatusBadge({ status }: Props) {
  const cls = classMap[status] ?? 'pending';
  return <span className={`badge ${cls}`}>{status}</span>;
}
