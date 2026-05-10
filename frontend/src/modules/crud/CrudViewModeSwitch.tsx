type CrudViewModeLink = {
  path: string;
  label: string;
  contextPattern?: string;
};

type Props = {
  modes: CrudViewModeLink[];
  groupAriaLabel: string;
  description?: string;
  actionLink?: {
    to: string;
    label: string;
    hideWhenActivePattern?: string;
    activeReplacement?: {
      to: string;
      label: string;
    };
  };
};

export function CrudViewModeSwitch(_props: Props) {
  // Las tabs (Lista/Galería/Tablero) se renderizan ahora en la barra blanca
  // (CrudResourceShellHeader), inyectadas desde ViewModeTabsCtx.
  return null;
}
