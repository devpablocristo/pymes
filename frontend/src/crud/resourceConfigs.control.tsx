/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { type CrudFieldValue, type CrudFormValues, type CrudPageConfig, type CrudResourceConfigMap } from '../components/CrudPage';
import { apiRequest, downloadAPIFile } from '../lib/api';
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
import { withCSVToolbar } from './csvToolbar';
import { buildConfiguredCrudPage, getCrudPageConfigFromMap, hasCrudResourceInMap } from './resourceConfigs.runtime';
import { asBoolean, asOptionalString, asString, formatDate } from './resourceConfigs.shared';

const controlResourceConfigs: CrudResourceConfigMap = {
  attachments: {
    ...createAttachmentsCrudConfig<AttachmentRow>({
      renderList: () => <AttachmentsListModeContent />,
      formatDate,
      buildCrudContextEntityPath,
      getCrudContextEntityParams,
    }),
  },
  audit: {
    ...createAuditCrudConfig<AuditEntryRow>({
      renderList: () => <AuditListModeContent />,
      formatDate,
    }),
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
  },
  webhooks: {
    ...createWebhooksCrudConfig<WebhookEndpoint>({
      renderList: () => <WebhooksListModeContent />,
      formatDate,
      asString,
      asOptionalString,
      asBoolean,
    }),
  },
};

const resourceConfigs = Object.fromEntries(
  Object.entries(controlResourceConfigs).map(([resourceId, config]) => [
    resourceId,
    resourceId === 'audit'
      ? withCSVToolbar(resourceId, config, {
          mode: 'server',
          allowImport: false,
          serverExport: {
            download: async (_entity) => {
              await downloadAPIFile('/v1/audit/export?format=csv');
            },
          },
        })
      : withCSVToolbar(resourceId, config, {}),
  ]),
) as CrudResourceConfigMap;

export const ConfiguredCrudPage = buildConfiguredCrudPage(resourceConfigs);

export function hasCrudResource(resourceId: string): boolean {
  return hasCrudResourceInMap(resourceConfigs, resourceId);
}

export function getCrudPageConfig<TRecord extends { id: string } = { id: string }>(
  resourceId: string,
  opts?: { preserveCsvToolbar?: boolean },
): CrudPageConfig<TRecord> | null {
  return getCrudPageConfigFromMap<TRecord>(resourceConfigs, resourceId, opts);
}
