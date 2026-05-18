import { createElement } from 'react';
import {
  IconAlert,
  IconBell,
  IconBuilding,
  IconCreditCard,
  IconDollar,
  IconEdit,
  IconGlobe,
  IconPalette,
  IconSettings,
  IconTrash,
  IconUsers,
} from '@devpablocristo/modules-ui-data-display/icons';
import { parseSectionHubSelection, type SectionHubSection } from '@devpablocristo/modules-ui-section-hub';

export type SettingsSection =
  | null
  | 'branches'
  | 'profile'
  | 'team'
  | 'rbac'
  | 'audit'
  | 'notifications'
  | 'automation'
  | 'company'
  | 'firebaseNotif'
  | 'currencies'
  | 'gateway'
  | 'appearance'
  | 'language'
  | 'workspace';

export type SettingsSectionCard = SectionHubSection<Exclude<SettingsSection, null>>;

export const SETTING_SECTIONS: SettingsSectionCard[] = [
  { id: 'profile', label: 'Perfil', desc: 'Datos personales y cuenta', icon: createElement(IconUsers) },
  { id: 'team', label: 'Equipo', desc: 'Invitar usuarios y ver miembros del tenant', icon: createElement(IconUsers) },
  { id: 'branches', label: 'Sucursales', desc: 'Sucursal principal del tenant', icon: createElement(IconBuilding) },
  { id: 'workspace', label: 'Negocio', desc: 'Razón social, monedas, IVA, prefijos', icon: createElement(IconBuilding) },
  { id: 'rbac', label: 'Roles y permisos', desc: 'Accesos administrativos y catálogo RBAC', icon: createElement(IconEdit) },
  { id: 'audit', label: 'Auditoría', desc: 'Actividad del espacio y exportación CSV', icon: createElement(IconTrash) },
  { id: 'appearance', label: 'Apariencia', desc: 'Tema, skin, logos y colores', icon: createElement(IconPalette) },
  { id: 'language', label: 'Idioma', desc: 'Idioma de la plataforma', icon: createElement(IconGlobe) },
  {
    id: 'notifications',
    label: 'Notificaciones',
    desc: 'Preferencias de correo y canales de alerta',
    icon: createElement(IconBell),
  },
  {
    id: 'automation',
    label: 'Automatización',
    desc: 'Reglas del asistente y tareas proactivas',
    icon: createElement(IconAlert),
  },
  { id: 'gateway', label: 'Pagos y facturación', desc: 'Plan, pasarelas y métodos de cobro', icon: createElement(IconCreditCard) },
  { id: 'currencies', label: 'Monedas', desc: 'Monedas habilitadas', icon: createElement(IconDollar) },
  { id: 'company', label: 'Empresa', desc: 'Datos de contacto y dirección', icon: createElement(IconBuilding) },
  { id: 'firebaseNotif', label: 'Firebase', desc: 'Configuración push notifications', icon: createElement(IconSettings) },
];

export const NON_ADMIN_SECTIONS = SETTING_SECTIONS.filter((item) => item.id !== 'rbac' && item.id !== 'audit');

export function sectionFromSearchParam(sections: SettingsSectionCard[], raw: string | null): SettingsSection {
  return parseSectionHubSelection(sections, raw);
}
