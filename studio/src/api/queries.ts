import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from './client';

export function useHealth() {
  return useQuery({ queryKey: ['health'], queryFn: api.getHealth, refetchInterval: 10_000 });
}

export function useEvents(limit = 100) {
  return useQuery({ queryKey: ['events', limit], queryFn: () => api.getEvents(limit) });
}

export function useEvent(id: string | null) {
  return useQuery({
    queryKey: ['event', id],
    queryFn: () => api.getEvent(id!),
    enabled: !!id,
  });
}

export function useScenarios() {
  return useQuery({ queryKey: ['scenarios'], queryFn: api.getScenarios });
}

export function useScenario(id: string | null) {
  return useQuery({
    queryKey: ['scenario', id],
    queryFn: () => api.getScenario(id!),
    enabled: !!id,
  });
}

export function useStartRecording() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { name: string; description: string; tags: string[] }) =>
      api.startRecording(vars.name, vars.description, vars.tags),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scenarios'] }),
  });
}

export function useStopRecording() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => api.stopRecording(),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scenarios'] }),
  });
}

export function useStartReplay() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { scenarioId: string; baseUrl: string; skipUnsafe?: boolean; confirmUnsafe?: boolean }) =>
      api.startReplay(vars.scenarioId, vars.baseUrl, vars),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['scenarios'] }),
  });
}

export function useReplayRun(id: string | null) {
  return useQuery({
    queryKey: ['replayRun', id],
    queryFn: () => api.getReplayRun(id!),
    enabled: !!id,
  });
}

export function useFaultRules() {
  return useQuery({ queryKey: ['faultRules'], queryFn: api.getFaultRules });
}

export function useCreateFaultRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.createFaultRule,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['faultRules'] }),
  });
}

export function useToggleFaultRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: { id: string; enabled: boolean }) =>
      api.updateFaultRule(vars.id, { enabled: vars.enabled }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['faultRules'] }),
  });
}

export function useDeleteFaultRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api.deleteFaultRule(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['faultRules'] }),
  });
}

export function useServices() {
  return useQuery({ queryKey: ['services'], queryFn: api.getServices });
}
