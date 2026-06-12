import { create } from 'zustand';

type ColorScheme = 'light' | 'dark';

type ThemeStore = {
  colorScheme: ColorScheme;
  setColorScheme: (scheme: ColorScheme) => void;
  toggleColorScheme: () => void;
};

export const useThemeStore = create<ThemeStore>((set) => ({
  colorScheme: 'dark',
  setColorScheme: (colorScheme) => set({ colorScheme }),
  toggleColorScheme: () =>
    set((state) => ({ colorScheme: state.colorScheme === 'dark' ? 'light' : 'dark' })),
}));
