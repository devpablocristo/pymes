import { createContext, useContext } from 'react';

export type ViewModeLink = {
  path: string;
  label: string;
  contextPattern?: string;
};

export const ViewModeTabsCtx = createContext<ViewModeLink[] | null>(null);

export function useViewModes(): ViewModeLink[] | null {
  return useContext(ViewModeTabsCtx);
}
