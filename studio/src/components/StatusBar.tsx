import type { HealthResponse } from '../api/types';
import type { SSEStatus } from '../api/sse';

interface Props {
  health?: HealthResponse;
  sseStatus: SSEStatus;
}

export function StatusBar({ health, sseStatus }: Props) {
  const dotClass =
    sseStatus === 'live' ? 'live' :
    sseStatus === 'reconnecting' ? 'reconnecting' : 'error';

  return (
    <div className="status-bar">
      <span>
        <span className={`status-dot ${dotClass}`} />
        {sseStatus === 'live' ? 'Live' :
         sseStatus === 'reconnecting' ? 'Reconnecting...' :
         sseStatus === 'connecting' ? 'Connecting...' : 'Disconnected'}
      </span>
      {health && (
        <>
          <span>v{health.version}</span>
          <span>Run: {health.runId.slice(0, 16)}</span>
          <span>DB: {health.database}</span>
        </>
      )}
    </div>
  );
}
