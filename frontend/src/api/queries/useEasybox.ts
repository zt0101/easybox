import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import {
  EndpointListSchema,
  EndpointViewSchema,
  SubscriptionViewSchema,
  type EndpointView,
  type QuickCreateFormValues,
  type SubscriptionView,
} from '@/schemas/api/easybox';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

async function fetchEndpoints(): Promise<EndpointView[]> {
  const msg = await HttpUtil.get('/panel/api/easybox/endpoints/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch lines');
  const validated = parseMsg(msg, EndpointListSchema, 'easybox/endpoints/list');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

async function fetchSubscription(): Promise<SubscriptionView> {
  const msg = await HttpUtil.get('/panel/api/easybox/subscription/info', undefined, {
    silent: true,
  });
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to fetch subscription');
  return parseMsg(msg, SubscriptionViewSchema, 'easybox/subscription/info').obj as SubscriptionView;
}

export function useEasyboxEndpoints() {
  return useQuery({ queryKey: keys.easybox.endpoints(), queryFn: fetchEndpoints });
}

export function useEasyboxSubscription() {
  return useQuery({ queryKey: keys.easybox.subscription(), queryFn: fetchSubscription });
}

export interface CreateResult {
  ok: boolean;
  line: EndpointView | null;
  message: string;
}

/* Quick Create reports failures through the returned line (failedStep /
 * lastError), so every call here is silent and the page renders the outcome. */
export function useEasyboxMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.easybox.root() });

  const createMut = useMutation({
    mutationFn: async (values: QuickCreateFormValues): Promise<CreateResult> => {
      const msg = await HttpUtil.post('/panel/api/easybox/endpoints/create', values, {
        ...JSON_HEADERS,
        silent: true,
      });
      const parsed = EndpointViewSchema.safeParse(msg?.obj);
      return {
        ok: Boolean(msg?.success),
        line: parsed.success ? parsed.data : null,
        message: msg?.msg ?? '',
      };
    },
    onSettled: () => invalidate(),
  });

  const removeMut = useMutation({
    mutationFn: (id: number) =>
      HttpUtil.post(`/panel/api/easybox/endpoints/del/${id}`, undefined, { silentSuccess: true }),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const setEnableMut = useMutation({
    mutationFn: ({ id, enable }: { id: number; enable: boolean }) =>
      HttpUtil.post(
        `/panel/api/easybox/endpoints/setEnable/${id}`,
        { enable },
        {
          ...JSON_HEADERS,
          silentSuccess: true,
        },
      ),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  const setInSubscriptionMut = useMutation({
    mutationFn: ({ id, include }: { id: number; include: boolean }) =>
      HttpUtil.post(
        `/panel/api/easybox/endpoints/setInSubscription/${id}`,
        { include },
        {
          ...JSON_HEADERS,
          silentSuccess: true,
        },
      ),
    onSuccess: (msg) => {
      if (msg?.success) invalidate();
    },
  });

  return {
    create: createMut.mutateAsync,
    creating: createMut.isPending,
    remove: removeMut.mutateAsync,
    setEnable: (id: number, enable: boolean) => setEnableMut.mutateAsync({ id, enable }),
    setInSubscription: (id: number, include: boolean) =>
      setInSubscriptionMut.mutateAsync({ id, include }),
  };
}

export async function fetchEndpointLink(id: number): Promise<string> {
  const msg = await HttpUtil.get(`/panel/api/easybox/endpoints/link/${id}`);
  return msg?.success && typeof msg.obj === 'string' ? msg.obj : '';
}
