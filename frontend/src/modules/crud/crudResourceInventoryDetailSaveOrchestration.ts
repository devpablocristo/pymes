import type {
  CrudInventoryAdjustPayload,
  CrudInventoryLevelSnapshot,
  CrudLinkedEntityPatch,
  CrudLinkedEntitySnapshot,
  CrudResourceInventoryDetailFeatureFlags,
  CrudResourceInventoryDetailPorts,
} from './crudResourceInventoryDetailContract';

function imageUrlListsEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i += 1) {
    if (a[i] !== b[i]) return false;
  }
  return true;
}

export type CrudInventoryDetailParsedInputs = {
  minParsed: number;
  absoluteQtyParsed: number;
};

export type CrudInventoryDetailDirtyResult = {
  productDirty: boolean;
  inventoryDirty: boolean;
  dirty: boolean;
};

/**
 * `productDirty` / `inventoryDirty` / `dirty` a partir del nivel, entidad enlazada y borradores de formulario.
 */
export function computeCrudInventoryDetailDirty(
  level: CrudInventoryLevelSnapshot,
  linked: CrudLinkedEntitySnapshot | null,
  serverImageUrls: string[],
  draftImageUrls: string[],
  nameInput: string,
  skuInput: string,
  trackStockInput: boolean,
  parsed: CrudInventoryDetailParsedInputs,
  flags: Pick<CrudResourceInventoryDetailFeatureFlags, 'linkedEntityFields' | 'inventoryQuantities' | 'linkedEntityTrackStock'>,
): CrudInventoryDetailDirtyResult {
  let productDirty = false;
  if (flags.linkedEntityFields) {
    const nameBaseline = (linked?.name ?? level.displayTitle ?? '').trim();
    const skuBaseline = (linked?.sku ?? level.displaySubtitle ?? '').trim();
    const urlsBaseline = serverImageUrls;
    const trackBaseline = level.trackStock !== false;
    const trackDirty = flags.linkedEntityTrackStock && trackStockInput !== trackBaseline;
    productDirty =
      nameInput.trim() !== nameBaseline ||
      skuInput.trim() !== skuBaseline ||
      !imageUrlListsEqual(draftImageUrls, urlsBaseline) ||
      trackDirty;
  }

  let inventoryDirty = false;
  if (flags.inventoryQuantities && trackStockInput) {
    const minChanged = Number.isFinite(parsed.minParsed) && parsed.minParsed !== level.minQuantity;
    const qtyChanged = Number.isFinite(parsed.absoluteQtyParsed) && parsed.absoluteQtyParsed !== level.quantity;
    inventoryDirty = minChanged || qtyChanged;
  }

  return { productDirty, inventoryDirty, dirty: productDirty || inventoryDirty };
}

export type CrudInventoryDetailPatchBuild = {
  patch: CrudLinkedEntityPatch;
  hasProductPatch: boolean;
  minChanged: boolean;
  qtyChanged: boolean;
  hasInventoryChange: boolean;
};

/** PATCH parcial de entidad enlazada + señales de cambio en inventario (cantidades). */
export function buildCrudInventoryDetailSavePatch(
  level: CrudInventoryLevelSnapshot,
  linked: CrudLinkedEntitySnapshot | null,
  serverImageUrls: string[],
  draftImageUrls: string[],
  nameTrim: string,
  skuTrim: string,
  trackStockInput: boolean,
  parsed: CrudInventoryDetailParsedInputs,
  flags: Pick<CrudResourceInventoryDetailFeatureFlags, 'linkedEntityFields' | 'inventoryQuantities' | 'linkedEntityTrackStock'>,
): CrudInventoryDetailPatchBuild {
  const nameBaseline = (linked?.name ?? level.displayTitle ?? '').trim();
  const skuBaseline = (linked?.sku ?? level.displaySubtitle ?? '').trim();
  const urlsBaseline = serverImageUrls;
  const trackBaseline = level.trackStock !== false;

  const patch: CrudLinkedEntityPatch = {};
  if (flags.linkedEntityFields) {
    if (nameTrim !== nameBaseline) patch.name = nameTrim;
    if (skuTrim !== skuBaseline) patch.sku = skuTrim;
    if (!imageUrlListsEqual(draftImageUrls, urlsBaseline)) patch.imageUrls = draftImageUrls;
    if (flags.linkedEntityTrackStock && trackStockInput !== trackBaseline) patch.trackStock = trackStockInput;
  }

  const hasProductPatch = flags.linkedEntityFields && Object.keys(patch).length > 0;

  const minChanged = Number.isFinite(parsed.minParsed) && parsed.minParsed !== level.minQuantity;
  const qtyChanged = Number.isFinite(parsed.absoluteQtyParsed) && parsed.absoluteQtyParsed !== level.quantity;
  const hasInventoryChange = Boolean(flags.inventoryQuantities && trackStockInput && (minChanged || qtyChanged));

  return { patch, hasProductPatch, minChanged, qtyChanged, hasInventoryChange };
}

/** Payload de ajuste de inventario; `null` si no hay cambio de inventario. */
export function buildCrudInventoryAdjustPayload(
  level: CrudInventoryLevelSnapshot,
  parsed: CrudInventoryDetailParsedInputs,
  build: Pick<CrudInventoryDetailPatchBuild, 'minChanged' | 'qtyChanged' | 'hasInventoryChange'>,
  notesTrimmed: string,
): CrudInventoryAdjustPayload | null {
  if (!build.hasInventoryChange) return null;
  return {
    quantityDelta: build.qtyChanged ? parsed.absoluteQtyParsed - level.quantity : 0,
    notes: notesTrimmed,
    ...(build.minChanged ? { minQuantity: parsed.minParsed } : {}),
  };
}

export type CrudInventoryDetailSaveValidation =
  | { ok: true }
  | { ok: false; kind: 'name' | 'notes' | 'noop' };

export function validateCrudInventoryDetailSave(
  hasProductPatch: boolean,
  hasInventoryChange: boolean,
  nameHasDisplay: boolean,
  notesTrimmed: string,
): CrudInventoryDetailSaveValidation {
  if (!hasProductPatch && !hasInventoryChange) return { ok: false, kind: 'noop' };
  if (!nameHasDisplay) return { ok: false, kind: 'name' };
  if (hasInventoryChange && !notesTrimmed) return { ok: false, kind: 'notes' };
  return { ok: true };
}

export type CrudInventoryDetailPersistResult = {
  level: CrudInventoryLevelSnapshot;
  linked: CrudLinkedEntitySnapshot | null;
  movements: Awaited<ReturnType<CrudResourceInventoryDetailPorts['loadMovements']>>;
};

/**
 * Orquestación: PATCH entidad enlazada (si hay) → POST ajuste inventario (si hay) → recarga nivel y entidad.
 * Un solo `try/catch` en el caller puede mapear el error a UI.
 */
export async function persistCrudInventoryDetailSave(
  ports: CrudResourceInventoryDetailPorts,
  args: {
    linkedEntityId: string;
    hasProductPatch: boolean;
    patch: CrudLinkedEntityPatch;
    hasInventoryChange: boolean;
    adjustPayload: CrudInventoryAdjustPayload | null;
  },
): Promise<CrudInventoryDetailPersistResult> {
  const { linkedEntityId, hasProductPatch, patch, hasInventoryChange, adjustPayload } = args;

  let linkedAfterPatch: CrudLinkedEntitySnapshot | null = null;
  if (hasProductPatch) {
    linkedAfterPatch = await ports.patchLinkedEntity(linkedEntityId, patch);
  }

  if (hasInventoryChange) {
    if (!adjustPayload) {
      throw new Error('persistCrudInventoryDetailSave: adjustPayload required when hasInventoryChange');
    }
    await ports.postInventoryAdjust(linkedEntityId, adjustPayload);
  }

  const level = await ports.loadInventoryLevel(linkedEntityId);

  let linked: CrudLinkedEntitySnapshot | null = linkedAfterPatch;
  try {
    linked = await ports.loadLinkedEntity(linkedEntityId);
  } catch {
    linked = linkedAfterPatch;
  }

  const movements = await ports.loadMovements(linkedEntityId);
  return { level, linked, movements };
}
