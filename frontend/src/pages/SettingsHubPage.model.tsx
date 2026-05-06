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
  { id: 'profile', label: 'Perfil', desc: 'Datos personales y cuenta', icon: <IconUsers /> },
  { id: 'branches', label: 'Sucursales', desc: 'Sucursal principal del tenant', icon: <IconBuilding /> },
  { id: 'workspace', label: 'Negocio', desc: 'Razón social, monedas, IVA, prefijos', icon: <IconBuilding /> },
  { id: 'rbac', label: 'Roles y permisos', desc: 'Accesos administrativos y catálogo RBAC', icon: <IconEdit /> },
  { id: 'audit', label: 'Auditoría', desc: 'Actividad del espacio y exportación CSV', icon: <IconTrash /> },
  { id: 'appearance', label: 'Apariencia', desc: 'Tema, skin, logos y colores', icon: <IconPalette /> },
  { id: 'language', label: 'Idioma', desc: 'Idioma de la plataforma', icon: <IconGlobe /> },
  {
    id: 'notifications',
    label: 'Notificaciones',
    desc: 'Preferencias de correo y canales de alerta',
    icon: <IconBell />,
  },
  { id: 'automation', label: 'Automatización', desc: 'Reglas del asistente y tareas proactivas', icon: <IconAlert /> },
  { id: 'gateway', label: 'Pagos y facturación', desc: 'Plan, pasarelas y métodos de cobro', icon: <IconCreditCard /> },
  { id: 'currencies', label: 'Monedas', desc: 'Monedas habilitadas', icon: <IconDollar /> },
  { id: 'company', label: 'Empresa', desc: 'Datos de contacto y dirección', icon: <IconBuilding /> },
  { id: 'firebaseNotif', label: 'Firebase', desc: 'Configuración push notifications', icon: <IconSettings /> },
];

export const NON_ADMIN_SECTIONS = SETTING_SECTIONS.filter((item) => item.id !== 'rbac' && item.id !== 'audit');

export function sectionFromSearchParam(sections: SettingsSectionCard[], raw: string | null): SettingsSection {
  return parseSectionHubSelection(sections, raw);
}
