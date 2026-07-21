import { create } from 'zustand';

import { clerkSignOut } from '@/services/token';
import { useSessionStore } from './session-store';

type AuthStore = {
  isAuthenticated: boolean;
  setAuthenticated: () => Promise<void>;
  logout: () => Promise<void>;
};

export const useAuthStore = create<AuthStore>((set) => ({
  isAuthenticated: false,
  setAuthenticated: async () => {
    set({ isAuthenticated: true });
    await useSessionStore.getState().hydrate();
  },
  logout: async () => {
    await clerkSignOut();
    set({ isAuthenticated: false });
    useSessionStore.getState().clear();
  },
}));
