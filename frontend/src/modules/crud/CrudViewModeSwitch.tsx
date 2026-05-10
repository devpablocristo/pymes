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
  return null;
}
