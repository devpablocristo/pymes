import { create } from 'zustand';

import { sessionService } from '@/services/session-service';
import { SessionAuth } from '@/services/types';

type SessionStore = {
  session: SessionAuth | null;
  isLoading: boolean;
  hydrate: () => Promise<void>;
  clear: () => void;
};

export const useSessionStore = create<SessionStore>((set) => ({
  session: null,
  isLoading: false,
  hydrate: async () => {
    set({ isLoading: true });
    try {
      const { auth } = await sessionService.get();
      set({ session: auth });
    } finally {
      set({ isLoading: false });
    }
  },
  clear: () => set({ session: null }),
}));
