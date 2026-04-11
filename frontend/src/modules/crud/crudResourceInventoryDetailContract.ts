/**
 * Contrato agnóstico para un detalle modal que combina:
 * - nivel de inventario (cantidades, alertas, ajustes), y
 * - ficha de la entidad enlazada al catálogo (nombre, SKU, imágenes, flags).
 *
 * El vertical solo mapea DTOs ↔ estos tipos, implementa `ports` y entrega `strings` (i18n).
 * Nada aquí nombra dominios concretos (stock, producto, bici, etc.).
 */

/** Fila canónica de inventario para UI genérico. */
export type CrudInventoryLevelSnapshot = {
  /** Id estable para keys de lista / tabla (puede coincidir con `linkedEntityId`). */
  listRecordId: string;
  /** Id de la entidad enlazada (catálogo) a la que pertenece el nivel. */
  linkedEntityId: string;
  displayTitle: string;
  displaySubtitle: string;
  quantity: number;
  minQuantity: number;
  /**
   * Si el backend no lo envía, el adaptador vertical debe definir un valor por defecto
   * coherente con su dominio (p. ej. `true` cuando el catálogo controla stock).
   */
  trackStock?: boolean;
  isLowStock: boolean;
  updatedAt: string;
};

/** Recorte de entidad enlazada para lectura / edición (PATCH típico de catálogo). */
export type CrudLinkedEntitySnapshot = {
  id: string;
  name: string;
  sku: string;
  imageUrls: string[];
  /** URL única legacy; vacío si solo hay `imageUrls`. */
  legacyImageUrl?: string;
  trackStock?: boolean;
};

/** Movimiento de inventario (datos crudos; el copy va en `strings` / formateadores). */
export type CrudInventoryMovementSnapshot = {
  id: string;
  kind: string;
  quantity: number;
  reason: string;
  notes: string;
  actorLabel: string;
  createdAt: string;
};

/** Payload agnóstico de ajuste; el adaptador lo serializa al API del vertical. */
export type CrudInventoryAdjustPayload = {
  quantityDelta: number;
  notes: string;
  minQuantity?: number;
};

/** PATCH parcial de entidad enlazada; el adaptador traduce nombres de campo al backend. */
export type CrudLinkedEntityPatch = {
  name?: string;
  sku?: string;
  imageUrls?: string[];
  trackStock?: boolean;
};

/** Textos de UI — siempre inyectados (p. ej. desde `lib/i18n`); el shell no hardcodea copy de negocio. */
export type CrudResourceInventoryDetailStrings = {
  dialogLoadingTitle: string;
  dialogFallbackTitle: string;
  loadErrorGeneric: string;
  sectionEditHeading: string;
  fieldDisplayNameLabel: string;
  fieldSkuLabel: string;
  fieldImageUrlsLabel: string;
  fieldImageUrlsHint: string;
  fieldTrackStockLabel: string;
  fieldQuantityLabel: string;
  fieldMinQuantityLabel: string;
  fieldNotesLabel: string;
  /** Texto auxiliar bajo el campo de notas (p. ej. cuándo es obligatorio). */
  fieldNotesHelper?: string;
  /** Título del subbloque «cantidades + notas» en modo edición. */
  inventoryQuantitiesSectionTitle: string;
  lastUpdatedPrefix: string;
  /** Plantilla opcional con `{datetime}` para la línea de última actualización al editar cantidades. */
  lastUpdatedEditHintTemplate?: string;
  movementsHeading: string;
  movementsEmpty: string;
  movementsLoading: string;
  movementColumns: {
    kind: string;
    quantity: string;
    reason: string;
    user: string;
    date: string;
  };
  badgeLowStock: string;
  readHintEdit: string;
  statCurrentLabel: string;
  statMinLabel: string;
  statUpdatedLabel: string;
  loadingBodyLabel: string;
  galleryAriaLabel: string;
  openImageFullscreenLabel: string;
  closeLabel: string;
  editLabel: string;
  cancelEditLabel: string;
  saveLabel: string;
  savingLabel: string;
  notesRequiredError: string;
  nameRequiredError: string;
  saveErrorGeneric: string;
  /** Opcional: pie de enlace a otra vista (precio, impuestos, etc.). */
  linkToAdvancedSettings?: string;
  archiveConfirm?: string;
  archiveError?: string;
  archiveLabel?: string;
  archivingLabel?: string;
};

export type CrudResourceInventoryDetailFeatureFlags = {
  linkedEntityFields: boolean;
  inventoryQuantities: boolean;
  movementsTable: boolean;
  archiveAction: boolean;
  linkedEntityTrackStock: boolean;
};

export const defaultCrudResourceInventoryDetailFeatureFlags: CrudResourceInventoryDetailFeatureFlags = {
  linkedEntityFields: true,
  inventoryQuantities: true,
  movementsTable: true,
  archiveAction: true,
  linkedEntityTrackStock: true,
};

/**
 * Puertos de datos (implementación = vertical con `apiRequest`, mocks en tests, etc.).
 */
export type CrudResourceInventoryDetailPorts<
  TMove extends CrudInventoryMovementSnapshot = CrudInventoryMovementSnapshot,
> = {
  loadInventoryLevel: (linkedEntityId: string) => Promise<CrudInventoryLevelSnapshot>;
  loadLinkedEntity: (linkedEntityId: string) => Promise<CrudLinkedEntitySnapshot | null>;
  loadMovements: (linkedEntityId: string) => Promise<TMove[]>;
  patchLinkedEntity: (linkedEntityId: string, patch: CrudLinkedEntityPatch) => Promise<CrudLinkedEntitySnapshot>;
  postInventoryAdjust: (linkedEntityId: string, body: CrudInventoryAdjustPayload) => Promise<void>;
  /** Archivado vía puerto; alternativa: prop `onArchive` en el modal. */
  archiveLinkedEntity?: (linkedEntityId: string) => Promise<void>;
};

/**
 * Props del shell de modal (la implementación del componente es otra tarea).
 * `linkedEntityId === null` ⇒ modal cerrado / sin render de contenido.
 */
/** Permisos de UI para acciones secundarias; `undefined` en cada clave = permitido. */
export type CrudResourceInventoryDetailPermissions = {
  canArchive?: boolean;
};

export type CrudResourceInventoryDetailModalProps<
  TMove extends CrudInventoryMovementSnapshot = CrudInventoryMovementSnapshot,
> = {
  linkedEntityId: string | null;
  onClose: () => void;
  onAfterSave?: () => void;
  strings: CrudResourceInventoryDetailStrings;
  flags?: Partial<CrudResourceInventoryDetailFeatureFlags>;
  ports: CrudResourceInventoryDetailPorts<TMove>;
  formatMovementKind: (kind: string) => string;
  formatDateTime: (iso: string) => string;
  /** Si se define y `strings.linkToAdvancedSettings` también, el shell puede mostrar `<Link>`. */
  advancedSettingsHref?: string;
  /**
   * Archivar la entidad enlazada (p. ej. el vertical inyecta `apiRequest` aquí).
   * Si está definido, tiene prioridad sobre `ports.archiveLinkedEntity`.
   */
  onArchive?: (linkedEntityId: string) => Promise<void>;
  /** Tras cancelar edición y volver el formulario al estado del servidor. */
  onCancelEdit?: () => void;
  permissions?: CrudResourceInventoryDetailPermissions;
};
