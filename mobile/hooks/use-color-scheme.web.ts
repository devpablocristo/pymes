import { useThemeStore } from '@/stores/theme-store';

export function useColorScheme() {
  return useThemeStore((state) => state.colorScheme);
}
