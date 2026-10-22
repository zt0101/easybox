import { z } from 'zod';

import {
  CloudflareViewSchema,
  EndpointViewSchema,
  PreviewViewSchema,
  SubscriptionViewSchema,
  TemplateViewSchema,
} from '@/generated/zod';

export { CloudflareViewSchema, EndpointViewSchema, PreviewViewSchema, SubscriptionViewSchema };
export type {
  CloudflareView,
  PreferredAddress,
  EndpointView,
  PreviewView,
  SubscriptionView,
  TemplateView,
} from '@/generated/zod';

export const EndpointListSchema = z.array(EndpointViewSchema);
export const TemplateListSchema = z.array(TemplateViewSchema);

export const QuickCreateFormSchema = z.object({
  name: z.string().trim().max(64, 'easybox.create.nameTooLong'),
  address: z.string().trim().max(253, 'easybox.create.addressTooLong'),
});
export type QuickCreateFormValues = z.infer<typeof QuickCreateFormSchema>;
