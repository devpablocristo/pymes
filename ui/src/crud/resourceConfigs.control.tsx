import {
  WebhooksListModeContent,
  createWebhooksCrudConfig,
  type WebhookEndpoint,
} from '../modules/audit-trail';
import { defineCrudDomain } from './defineCrudDomain';
import { asBoolean, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  {
    webhooks: {
      ...createWebhooksCrudConfig<WebhookEndpoint>({
        renderList: () => <WebhooksListModeContent />,
        formatDate,
        asString,
        asOptionalString,
        asBoolean,
      }),
      featureFlags: { tagPills: false, standardMedia: false },
    },
  },
);
