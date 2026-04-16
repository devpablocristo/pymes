import { apiRequest } from '../../lib/api';
import { readActiveBranchId } from '../../lib/branchSelectionStorage';
import type {
  CrudInventoryAdjustPayload,
  CrudInventoryLevelSnapshot,
  CrudInventoryMovementSnapshot,
  CrudLinkedEntityPatch,
  CrudLinkedEntitySnapshot,
  CrudResourceInventoryDetailPorts,
  CrudResourceInventoryDetailStrings,
} from '../crud/crudResourceInventoryDetailContract';
import { fetchStockLevelByProductId, type StockLevelRow } from './stockData';

type ProductApiRow = {
  id: string;
  name: string;
  sku?: string;
  image_url?: string;
  image_urls?: string[];
  track_stock?: boolean;
};

type MovementApiRow = {
  id: string;
  type: string;
  quantity: number;
  reason: string;
  notes: string;
  created_by: string;
  created_at: string;
};

type AttachmentUploadRequest = {
  storage_key: string;
  upload_url: string;
};

type AttachmentRow = {
  id: string;
};

export const stockInventoryDetailModalStringsEs: CrudResourceInventoryDetailStrings = {
  dialogLoadingTitle: 'Cargando…',
  dialogFallbackTitle: 'Inventario',
  loadErrorGeneric: 'No se pudo cargar el inventario.',
  sectionEditHeading: 'Editar',
  fieldDisplayNameLabel: 'Nombre',
  fieldSkuLabel: 'SKU',
  fieldImageUrlsLabel: 'Imágenes',
  fieldImageUrlsHint: 'Podés subir una o varias imágenes.',
  fieldImageUploadActionLabel: 'Subir imágenes',
  fieldImageUploadingLabel: 'Subiendo imágenes…',
  fieldImageRemoveLabel: 'Quitar',
  fieldTrackStockLabel: 'Controlar stock en depósito',
  fieldQuantityLabel: 'Cantidad actual',
  fieldMinQuantityLabel: 'Stock mínimo',
  fieldNotesLabel: 'Notas / motivo',
  fieldNotesHelper: 'Obligatorio solo si cambiás la cantidad actual o el stock mínimo (con control de stock activo).',
  inventoryQuantitiesSectionTitle: 'Cantidades y notas',
  lastUpdatedPrefix: 'Última actualización en servidor:',
  lastUpdatedEditHintTemplate: 'Última actualización en servidor: {datetime}',
  movementsHeading: 'Movimientos recientes',
  movementsEmpty: 'Sin movimientos registrados.',
  movementsLoading: 'Cargando movimientos…',
  movementColumns: {
    kind: 'Tipo',
    quantity: 'Cant.',
    reason: 'Motivo',
    user: 'Usuario',
    date: 'Fecha',
  },
  badgeLowStock: 'bajo mínimo',
  readHintEdit:
    'Usá Editar para modificar nombre, SKU, imágenes (URLs) y control de stock. Las cantidades van en el bloque inferior; si las cambiás, indicá notas.',
  statCurrentLabel: 'Actual',
  statMinLabel: 'Mínimo',
  statUpdatedLabel: 'Actualizado',
  loadingBodyLabel: 'Cargando inventario…',
  galleryAriaLabel: 'Imágenes',
  openImageFullscreenLabel: 'Ver imagen a pantalla completa',
  closeLabel: 'Cerrar',
  editLabel: 'Editar',
  cancelEditLabel: 'Cancelar edición',
  saveLabel: 'Guardar',
  savingLabel: 'Guardando…',
  notesRequiredError: 'Indicá notas cuando cambiás cantidad actual o stock mínimo.',
  nameRequiredError: 'El nombre no puede quedar vacío.',
  saveErrorGeneric: 'Error al guardar.',
  linkToAdvancedSettings: 'Ir a catálogo de productos (nombre, precio, SKU…)',
  archiveConfirm:
    '¿Archivar este producto? Dejará de mostrarse en listados activos; podés restaurarlo desde Productos → archivados.',
  archiveError: 'No se pudo archivar.',
  archiveLabel: 'Archivar producto',
  archivingLabel: 'Archivando…',
};

function mapLevel(row: StockLevelRow): CrudInventoryLevelSnapshot {
  return {
    listRecordId: row.id,
    linkedEntityId: row.product_id,
    displayTitle: row.product_name,
    displaySubtitle: row.sku?.trim() || '',
    quantity: row.quantity,
    minQuantity: row.min_quantity,
    trackStock: row.track_stock,
    isLowStock: row.is_low_stock,
    updatedAt: row.updated_at,
  };
}

function mapLinked(p: ProductApiRow): CrudLinkedEntitySnapshot {
  return {
    id: p.id,
    name: p.name,
    sku: p.sku ?? '',
    imageUrls: p.image_urls ?? [],
    legacyImageUrl: p.image_url,
    trackStock: p.track_stock,
  };
}

function mapMovement(m: MovementApiRow): CrudInventoryMovementSnapshot {
  return {
    id: m.id,
    kind: m.type,
    quantity: m.quantity,
    reason: m.reason,
    notes: m.notes,
    actorLabel: m.created_by,
    createdAt: m.created_at,
  };
}

export function formatStockInventoryDateTime(raw: string): string {
  if (!raw) return '';
  try {
    return new Date(raw).toLocaleString('es-AR', {
      day: '2-digit',
      month: '2-digit',
      year: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return raw;
  }
}

export function formatStockInventoryMovementKind(kind: string): string {
  switch (kind) {
    case 'in':
      return 'Entrada';
    case 'out':
      return 'Salida';
    case 'adjustment':
      return 'Ajuste';
    default:
      return kind;
  }
}

export type StockInventoryDetailHandlers = {
  fetchLevel: (productId: string) => Promise<CrudInventoryLevelSnapshot>;
  fetchEntity: (productId: string) => Promise<CrudLinkedEntitySnapshot>;
  fetchMovements: (productId: string) => Promise<CrudInventoryMovementSnapshot[]>;
  patchEntity: (productId: string, patch: CrudLinkedEntityPatch) => Promise<CrudLinkedEntitySnapshot>;
  uploadImages: (productId: string, files: File[]) => Promise<string[]>;
  postAdjust: (productId: string, body: CrudInventoryAdjustPayload) => Promise<void>;
  archiveEntity: (productId: string) => Promise<void>;
};

export async function defaultFetchStockInventoryLevel(productId: string): Promise<CrudInventoryLevelSnapshot> {
  return mapLevel(await fetchStockLevelByProductId(productId));
}

export async function defaultFetchStockLinkedEntity(productId: string): Promise<CrudLinkedEntitySnapshot> {
  const p = await apiRequest<ProductApiRow>(`/v1/products/${encodeURIComponent(productId)}`);
  return mapLinked(p);
}

export async function defaultFetchStockInventoryMovements(productId: string): Promise<CrudInventoryMovementSnapshot[]> {
  const query = new URLSearchParams({ limit: '50', product_id: productId });
  const branchId = readActiveBranchId();
  if (branchId) query.set('branch_id', branchId);
  const data = await apiRequest<{ items?: MovementApiRow[] | null }>(
    `/v1/inventory/movements?${query.toString()}`,
  );
  return (data.items ?? []).map(mapMovement);
}

export async function defaultPatchStockLinkedEntity(
  productId: string,
  patch: CrudLinkedEntityPatch,
): Promise<CrudLinkedEntitySnapshot> {
  const body: Record<string, unknown> = {};
  if (patch.name !== undefined) body.name = patch.name;
  if (patch.sku !== undefined) body.sku = patch.sku;
  if (patch.imageUrls !== undefined) body.image_urls = patch.imageUrls;
  if (patch.trackStock !== undefined) body.track_stock = patch.trackStock;
  const p = await apiRequest<ProductApiRow>(`/v1/products/${encodeURIComponent(productId)}`, {
    method: 'PATCH',
    body,
  });
  return mapLinked(p);
}

export async function defaultUploadStockLinkedEntityImages(productId: string, files: File[]): Promise<string[]> {
  const urls: string[] = [];
  for (const file of files) {
    const upload = await apiRequest<AttachmentUploadRequest>('/v1/attachments/upload-url', {
      method: 'POST',
      body: {
        entity_type: 'products',
        entity_id: productId,
        file_name: file.name,
        content_type: file.type || 'application/octet-stream',
        size_bytes: file.size,
      },
    });

    const response = await fetch(upload.upload_url, {
      method: 'PUT',
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
      body: file,
      credentials: 'same-origin',
    });
    if (!response.ok) throw new Error('No se pudo subir una imagen.');

    const attachment = await apiRequest<AttachmentRow>('/v1/attachments/confirm', {
      method: 'POST',
      body: {
        entity_type: 'products',
        entity_id: productId,
        file_name: file.name,
        content_type: file.type || 'application/octet-stream',
        size_bytes: file.size,
        storage_key: upload.storage_key,
      },
    });

    urls.push(`/v1/attachments/${attachment.id}/download`);
  }
  return urls;
}

export async function defaultPostStockInventoryAdjust(productId: string, body: CrudInventoryAdjustPayload): Promise<void> {
  const query = new URLSearchParams();
  const branchId = readActiveBranchId();
  if (branchId) query.set('branch_id', branchId);
  const suffix = query.size > 0 ? `?${query.toString()}` : '';
  await apiRequest(`/v1/inventory/${encodeURIComponent(productId)}/adjust${suffix}`, {
    method: 'POST',
    body: {
      quantity: body.quantityDelta,
      notes: body.notes,
      ...(body.minQuantity !== undefined ? { min_quantity: body.minQuantity } : {}),
    },
  });
}

export async function defaultArchiveStockLinkedEntity(productId: string): Promise<void> {
  await apiRequest(`/v1/products/${encodeURIComponent(productId)}/archive`, { method: 'POST', body: {} });
}

export function buildStockInventoryDetailPorts(
  overrides?: Partial<StockInventoryDetailHandlers>,
): CrudResourceInventoryDetailPorts {
  return {
    loadInventoryLevel: overrides?.fetchLevel ?? defaultFetchStockInventoryLevel,
    loadLinkedEntity: overrides?.fetchEntity ?? defaultFetchStockLinkedEntity,
    loadMovements: overrides?.fetchMovements ?? defaultFetchStockInventoryMovements,
    patchLinkedEntity: overrides?.patchEntity ?? defaultPatchStockLinkedEntity,
    uploadLinkedEntityImages: overrides?.uploadImages ?? defaultUploadStockLinkedEntityImages,
    postInventoryAdjust: overrides?.postAdjust ?? defaultPostStockInventoryAdjust,
    archiveLinkedEntity: overrides?.archiveEntity ?? defaultArchiveStockLinkedEntity,
  };
}
