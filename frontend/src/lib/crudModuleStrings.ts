import type { CrudStrings } from '@devpablocristo/modules-crud-ui';
import type { LanguageCode } from './i18n';
import { commonMessages } from './i18n/messages/common';
import { crudMessages } from './i18n/messages/crud';

/**
 * Solo adaptador i18n → `CrudStrings`: el contrato y defaults viven en `@devpablocristo/modules-crud-ui`;
 * acá no se duplican textos del motor, solo se leen `common` + `crud` de Pymes.
 */
export function buildPymesCrudStrings(language: LanguageCode): CrudStrings {
  const c = commonMessages[language];
  const r = crudMessages[language];
  return {
    statusLoading: c['common.status.loading'],
    statusSaving: c['common.status.saving'],
    actionSave: c['common.actions.save'],
    actionCancel: c['common.actions.cancel'],
    actionEdit: c['common.actions.edit'],
    actionDelete: c['common.actions.delete'],
    actionArchive: c['common.actions.archive'],
    actionRestore: c['common.actions.restore'],
    actionConfirm: c['common.actions.confirm'],
    titleArchived: r['crud.title.archived'],
    searchPlaceholder: r['crud.search.placeholder'],
    selectPlaceholder: r['crud.select.placeholder'],
    toggleShowActive: r['crud.toggle.showActive'],
    toggleShowArchived: r['crud.toggle.showArchived'],
    emptySearch: r['crud.empty.search'],
    emptyArchived: r['crud.empty.archived'],
    emptyActive: r['crud.empty.active'],
    tableActions: r['crud.table.actions'],
    buttonNew: r['crud.button.new'],
    buttonCreateFirst: r['crud.button.createFirst'],
    formEdit: r['crud.form.edit'],
    formCreate: r['crud.form.create'],
    confirmHint: r['crud.confirm.hint'],
    confirmPlaceholder: r['crud.confirm.placeholder'],
    confirmWord: r['crud.confirm.word'],
  };
}
