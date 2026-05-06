import { downloadAPIFile } from '../lib/api';
import { buildCrudContextEntityPath, getCrudContextEntityParams } from '../modules/crud';
import {
  AttachmentsListModeContent,
  AuditListModeContent,
  TimelineListModeContent,
  WebhooksListModeContent,
  createAttachmentsCrudConfig,
  createAuditCrudConfig,
  createTimelineCrudConfig,
  createWebhooksCrudConfig,
  type AttachmentRow,
  type AuditEntryRow,
  type TimelineEntryRow,
  type WebhookEndpoint,
} from '../modules/audit-trail';
import { defineCrudDomain } from './defineCrudDomain';
import { asBoolean, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain(
  {
    attachments: {
      ...createAttachmentsCrudConfig<AttachmentRow>({
        renderList: () => <AttachmentsListModeContent />,
        formatDate,
        buildCrudContextEntityPath,
        getCrudContextEntityParams,
      }),
      featureFlags: { tagPills: false, standardMedia: false },
    },
    audit: {
      ...createAuditCrudConfig<AuditEntryRow>({
        renderList: () => <AuditListModeContent />,
        formatDate,
      }),
      featureFlags: { tagPills: false, standardMedia: false },
    },
    timeline: {
      ...createTimelineCrudConfig<TimelineEntryRow>({
        renderList: () => <TimelineListModeContent />,
        formatDate,
        buildCrudContextEntityPath,
        getCrudContextEntityParams,
        asString,
        asOptionalString,
      }),
      featureFlags: { tagPills: false, standardMedia: false },
    },
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
  {
    csvOverrides: {
      audit: {
        mode: 'server',
        allowImport: false,
        serverExport: {
          download: async (_entity) => {
            await downloadAPIFile('/v1/audit/export?format=csv');
          },
        },
      },
    },
  },
);
