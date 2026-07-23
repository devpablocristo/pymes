const clearers = new Set<() => void>();

export function registerTenantCacheClearer(clear: () => void) {
  clearers.add(clear);
  return () => clearers.delete(clear);
}

export function clearTenantCaches() {
  for (const clear of clearers) {
    clear();
  }
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("pymes:tenant-cache-cleared"));
  }
}
