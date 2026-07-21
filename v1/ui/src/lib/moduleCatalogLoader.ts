let moduleCatalogPromise: Promise<typeof import('./moduleCatalog')> | null = null;

export function loadModuleCatalog() {
  if (moduleCatalogPromise == null) {
    moduleCatalogPromise = import('./moduleCatalog');
  }
  return moduleCatalogPromise;
}
