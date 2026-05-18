import { useContext } from 'react';
import { HeaderMenuItemsContext } from './HeaderMenuContext.model';

export function useHeaderMenuItems() {
  return useContext(HeaderMenuItemsContext);
}
