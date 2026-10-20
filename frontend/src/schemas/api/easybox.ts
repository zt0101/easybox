import { z } from 'zod';

import { EndpointViewSchema, SubscriptionViewSchema } from '@/generated/zod';

export { EndpointViewSchema, SubscriptionViewSchema };
export type { EndpointView, SubscriptionView } from '@/generated/zod';

export const EndpointListSchema = z.array(EndpointViewSchema);

export const QuickCreateFormSchema = z.object({
  name: z.string().trim().max(64, 'easybox.create.nameTooLong'),
  address: z.string().trim().max(253, 'easybox.create.addressTooLong'),
});
export type QuickCreateFormValues = z.infer<typeof QuickCreateFormSchema>;
