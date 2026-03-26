/**
 * Inventario 1:1 de features del template (`wowdash-port/App.jsx`) bajo `/console/wowdash/*`.
 * Comentario: al añadir una Route en App.jsx, añadir aquí la misma fila (orden = orden del template).
 */
export type WowdashTemplateFeature = {
  /** Segmento tras `/console/wowdash/`; vacío = índice. */
  segment: string;
  /** Agrupa los tests en la salida de Playwright. */
  category: string;
  /** Nombre legible del feature extraído del template. */
  titleEs: string;
};

const ROWS: [string, string, string][] = [
  ['', 'Dashboards', 'Dashboard AI — variante 1 (index)'],
  ['index-2', 'Dashboards', 'Dashboard CRM — variante 2'],
  ['index-3', 'Dashboards', 'Dashboard eCommerce — variante 3'],
  ['index-4', 'Dashboards', 'Dashboard cripto — variante 4'],
  ['index-5', 'Dashboards', 'Dashboard inversión — variante 5'],
  ['index-6', 'Dashboards', 'Dashboard LMS — variante 6'],
  ['index-7', 'Dashboards', 'Dashboard NFT / gaming — variante 7'],
  ['index-8', 'Dashboards', 'Dashboard médico — variante 8'],
  ['index-9', 'Dashboards', 'Dashboard analítica — variante 9'],
  ['index-10', 'Dashboards', 'Dashboard POS / inventario — variante 10'],
  ['index-11', 'Dashboards', 'Dashboard finanzas / banca — variante 11'],

  ['add-user', 'Usuarios y perfiles', 'Alta de usuario (demo)'],
  ['users-list', 'Usuarios y perfiles', 'Listado de usuarios'],
  ['users-grid', 'Usuarios y perfiles', 'Usuarios en grilla'],
  ['view-profile', 'Usuarios y perfiles', 'Ver perfil de usuario'],
  ['assign-role', 'Usuarios y perfiles', 'Asignar rol'],
  ['role-access', 'Usuarios y perfiles', 'Roles y acceso'],

  ['alert', 'Componentes UI', 'Alertas'],
  ['avatar', 'Componentes UI', 'Avatares'],
  ['badges', 'Componentes UI', 'Badges'],
  ['button', 'Componentes UI', 'Botones'],
  ['card', 'Componentes UI', 'Cards'],
  ['carousel', 'Componentes UI', 'Carrusel'],
  ['colors', 'Componentes UI', 'Paleta de colores'],
  ['dropdown', 'Componentes UI', 'Dropdowns'],
  ['pagination', 'Componentes UI', 'Paginación'],
  ['progress', 'Componentes UI', 'Barras de progreso'],
  ['radio', 'Componentes UI', 'Radio buttons'],
  ['star-rating', 'Componentes UI', 'Valoración con estrellas'],
  ['switch', 'Componentes UI', 'Switches'],
  ['tabs', 'Componentes UI', 'Pestañas y acordeón'],
  ['tags', 'Componentes UI', 'Tags'],
  ['tooltip', 'Componentes UI', 'Tooltip y popover'],
  ['typography', 'Componentes UI', 'Tipografía'],
  ['list', 'Componentes UI', 'Listas'],
  ['videos', 'Componentes UI', 'Videos embebidos'],

  ['form', 'Formularios', 'Inputs de formulario'],
  ['form-layout', 'Formularios', 'Layout de formulario'],
  ['form-validation', 'Formularios', 'Validación de formulario'],
  ['wizard', 'Formularios', 'Asistente (wizard)'],
  ['image-upload', 'Formularios', 'Subida de imágenes'],

  ['table-basic', 'Tablas', 'Tabla básica'],
  ['table-data', 'Tablas', 'Tabla con datos (DataTables)'],

  ['line-chart', 'Gráficos y widgets', 'Gráfico de líneas'],
  ['column-chart', 'Gráficos y widgets', 'Gráfico de columnas'],
  ['pie-chart', 'Gráficos y widgets', 'Gráfico circular'],
  ['widgets', 'Gráficos y widgets', 'Widgets'],

  ['email', 'Aplicación', 'Bandeja de email (demo)'],
  ['starred', 'Aplicación', 'Email destacados'],
  ['chat-empty', 'Aplicación', 'Chat vacío'],
  ['chat-message', 'Aplicación', 'Chat con mensajes'],
  ['chat-profile', 'Aplicación', 'Chat / perfil'],
  ['calendar-main', 'Aplicación', 'Calendario principal'],
  ['calendar', 'Aplicación', 'Calendario (alias)'],
  ['kanban', 'Aplicación', 'Tablero Kanban'],

  ['invoice-list', 'Facturas', 'Listado de facturas'],
  ['invoice-preview', 'Facturas', 'Vista previa de factura'],
  ['invoice-add', 'Facturas', 'Alta de factura'],
  ['invoice-edit', 'Facturas', 'Edición de factura'],

  ['text-generator', 'IA (demo)', 'Generador de texto'],
  ['text-generator-new', 'IA (demo)', 'Generador de texto (nuevo)'],
  ['code-generator', 'IA (demo)', 'Generador de código'],
  ['code-generator-new', 'IA (demo)', 'Generador de código (nuevo)'],
  ['image-generator', 'IA (demo)', 'Generador de imágenes'],
  ['voice-generator', 'IA (demo)', 'Generador de voz'],
  ['video-generator', 'IA (demo)', 'Generador de video'],

  ['wallet', 'Cripto / marketplace', 'Billetera (demo)'],
  ['marketplace', 'Cripto / marketplace', 'Marketplace'],
  ['marketplace-details', 'Cripto / marketplace', 'Detalle marketplace'],
  ['portfolio', 'Cripto / marketplace', 'Portafolio'],

  ['gallery', 'Galería y blog', 'Galería con descripción'],
  ['gallery-grid', 'Galería y blog', 'Galería en grilla'],
  ['gallery-masonry', 'Galería y blog', 'Galería masonry'],
  ['gallery-hover', 'Galería y blog', 'Galería con hover'],
  ['blog', 'Galería y blog', 'Listado blog'],
  ['blog-details', 'Galería y blog', 'Detalle de post'],
  ['add-blog', 'Galería y blog', 'Alta / edición de post'],

  ['testimonials', 'Páginas de contenido', 'Testimonios'],
  ['faq', 'Páginas de contenido', 'FAQ'],
  ['pricing', 'Páginas de contenido', 'Precios / planes'],
  ['terms-condition', 'Páginas de contenido', 'Términos y condiciones'],
  ['blank-page', 'Páginas de contenido', 'Página en blanco'],
  ['view-details', 'Páginas de contenido', 'Vista de detalle genérica'],

  ['coming-soon', 'Estado y errores', 'Próximamente'],
  ['access-denied', 'Estado y errores', 'Acceso denegado'],
  ['maintenance', 'Estado y errores', 'Mantenimiento'],
  ['sign-in', 'Estado y errores', 'Pantalla sign-in (demo)'],
  ['sign-up', 'Estado y errores', 'Pantalla sign-up (demo)'],
  ['forgot-password', 'Estado y errores', 'Olvidé mi contraseña (demo)'],

  ['company', 'Ajustes (demo)', 'Empresa / settings'],
  ['notification', 'Ajustes (demo)', 'Notificaciones'],
  ['notification-alert', 'Ajustes (demo)', 'Alertas de notificación'],
  ['theme', 'Ajustes (demo)', 'Tema'],
  ['currencies', 'Ajustes (demo)', 'Monedas'],
  ['language', 'Ajustes (demo)', 'Idiomas'],
  ['payment-gateway', 'Ajustes (demo)', 'Pasarela de pago'],

  ['__e2e_unknown_route__', 'Catch-all', 'Ruta inexistente → página 404 del template'],
];

export const WOWDASH_TEMPLATE_FEATURES: WowdashTemplateFeature[] = ROWS.map(([segment, category, titleEs]) => ({
  segment,
  category,
  titleEs,
}));

/** @deprecated usar WOWDASH_TEMPLATE_FEATURES */
export const WOWDASH_RELATIVE_PATHS: string[] = WOWDASH_TEMPLATE_FEATURES.map((f) => f.segment);

export const WOWDASH_TEMPLATE_FEATURE_COUNT = WOWDASH_TEMPLATE_FEATURES.length;
