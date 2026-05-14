import { useEffect, useRef, useState, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import type { TimelineEvent } from './types';

export type SSEStatus = 'connecting' | 'live' | 'reconnecting' | 'error';

export function useSSE() {
  const [status, setStatus] = useState<SSEStatus>('connecting');
  const qc = useQueryClient();
  const esRef = useRef<EventSource | null>(null);
  const retryRef = useRef(0);

  const connect = useCallback(() => {
    if (esRef.current) {
      esRef.current.close();
    }

    const es = new EventSource('/api/events/stream');
    esRef.current = es;

    es.onopen = () => {
      setStatus('live');
      retryRef.current = 0;
    };

    es.addEventListener('timeline.event', (e: MessageEvent) => {
      try {
        const event: TimelineEvent = JSON.parse(e.data);
        // Prepend to events cache
        qc.setQueryData<{ items: TimelineEvent[]; nextCursor: string }>(
          ['events', 100],
          (old) => {
            if (!old) return { items: [event], nextCursor: '' };
            return { ...old, items: [event, ...old.items].slice(0, 200) };
          },
        );
      } catch {
        // ignore parse errors
      }
    });

    es.onerror = () => {
      es.close();
      retryRef.current++;
      const delay = Math.min(1000 * 2 ** retryRef.current, 30_000);
      setStatus('reconnecting');
      setTimeout(connect, delay);
    };
  }, [qc]);

  useEffect(() => {
    connect();
    return () => {
      esRef.current?.close();
    };
  }, [connect]);

  return { status };
}
