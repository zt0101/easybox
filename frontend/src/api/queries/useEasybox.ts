import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import {
  CloudflareViewSchema,
  EndpointListSchema,
  EndpointViewSchema,
  PreviewViewSchema,
  SubscriptionViewSchema,
  TemplateListSchema,
  type CloudflareView,
  type EndpointView,
  type PreviewView,
  type QuickCreateFormValues,
  type SubscriptionView,
  type TemplateView,
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

async function fetchTemplates(): Promise<TemplateView[]> {
  const msg = await HttpUtil.get('/panel/api/easybox/templates', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to fetch templates');
  const validated = parseMsg(msg, TemplateListSchema, 'easybox/templates');
  return Array.isArray(validated.obj) ? validated.obj : [];
}

export function useEasyboxTemplates() {
  return useQuery({ queryKey: keys.easybox.templates(), queryFn: fetchTemplates });
}

async function fetchCloudflare(): Promise<CloudflareView> {
  const msg = await HttpUtil.get('/panel/api/easybox/cloudflare', undefined, { silent: true });
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to fetch Cloudflare settings');
  return parseMsg(msg, CloudflareViewSchema, 'easybox/cloudflare').obj as CloudflareView;
}

export function useEasyboxCloudflare() {
  return useQuery({ queryKey: keys.easybox.cloudflare(), queryFn: fetchCloudflare });
}

/* Saving checks the token against Cloudflare; the page shows the error text,
 * so the call is silent. Both calls refresh the templates (CDN availability). */
export function useEasyboxCloudflareMutations() {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: keys.easybox.root() });

  const saveMut = useMutation({
    mutationFn: async (values: { token: string; domain: string }) => {
      const msg = await HttpUtil.post('/panel/api/easybox/cloudflare', values, {
        ...JSON_HEADERS,
        silent: true,
      });
      return { ok: Boolean(msg?.success), message: msg?.msg ?? '' };
    },
    onSettled: () => invalidate(),
  });

  const clearMut = useMutation({
    mutationFn: () =>
      HttpUtil.post('/panel/api/easybox/cloudflare/clear', undefined, { silentSuccess: true }),
    onSettled: () => invalidate(),
  });

  const preferredMut = useMutation({
    mutationFn: async (addresses: string[]) => {
      const msg = await HttpUtil.post(
        '/panel/api/easybox/cloudflare/preferred',
        { addresses },
        { ...JSON_HEADERS, silent: true },
      );
      return { ok: Boolean(msg?.success), message: msg?.msg ?? '' };
    },
    onSettled: () => invalidate(),
  });

  return {
    save: saveMut.mutateAsync,
    saving: saveMut.isPending,
    clear: clearMut.mutateAsync,
    savePreferred: preferredMut.mutateAsync,
    savingPreferred: preferredMut.isPending,
  };
}

async function fetchPreview(client: string): Promise<PreviewView> {
  const msg = await HttpUtil.get(
    '/panel/api/easybox/subscription/preview',
    { client },
    {
      silent: true,
    },
  );
  if (!msg?.success || !msg.obj) throw new Error(msg?.msg || 'Failed to render the preview');
  return parseMsg(msg, PreviewViewSchema, 'easybox/subscription/preview').obj as PreviewView;
}

// Rendered on demand: only once the admin picked a client to preview as.
export function useEasyboxPreview(client: string | null) {
  return useQuery({
    queryKey: keys.easybox.preview(client ?? ''),
    queryFn: () => fetchPreview(client ?? ''),
    enabled: client !== null,
  });
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
    mutationFn: async (
      values: QuickCreateFormValues & { template: string },
    ): Promise<CreateResult> => {
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

  const regenerateMut = useMutation({
    mutationFn: async ({
      id,
      keepOld,
    }: {
      id: number;
      keepOld: boolean;
    }): Promise<CreateResult> => {
      const msg = await HttpUtil.post(
        `/panel/api/easybox/endpoints/regenerate/${id}`,
        { keepOld },
        { ...JSON_HEADERS, silent: true },
      );
      const parsed = EndpointViewSchema.safeParse(msg?.obj);
      return {
        ok: Boolean(msg?.success),
        line: parsed.success ? parsed.data : null,
        message: msg?.msg ?? '',
      };
    },
    onSettled: () => invalidate(),
  });

  const rotateMut = useMutation({
    mutationFn: () =>
      HttpUtil.post('/panel/api/easybox/subscription/rotate', undefined, { silentSuccess: true }),
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
    regenerate: (id: number, keepOld: boolean) => regenerateMut.mutateAsync({ id, keepOld }),
    regenerating: regenerateMut.isPending,
    rotateSubscription: rotateMut.mutateAsync,
  };
}

export async function fetchEndpointLink(id: number): Promise<string> {
  const msg = await HttpUtil.get(`/panel/api/easybox/endpoints/link/${id}`);
  return msg?.success && typeof msg.obj === 'string' ? msg.obj : '';
}
