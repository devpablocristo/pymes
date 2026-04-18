import { useI18n } from '../lib/i18n';
import { useBranchSelection } from '../lib/useBranchSelection';

export function BranchSwitcher() {
  const { language } = useI18n();
  const { availableBranches, selectedBranchId, setSelectedBranchId, isLoading, isError } = useBranchSelection();

  if (isLoading || isError || availableBranches.length <= 1) {
    return null;
  }

  const copy =
    language === 'en'
      ? {
          label: 'Location',
          aria: 'Active location',
        }
      : {
          label: 'Sucursal',
          aria: 'Sucursal activa',
        };

  return (
    <div className="shell-branch-switcher">
      <label className="shell-branch-switcher-label" htmlFor="shell-branch-switcher">
        {copy.label}
      </label>
      <select
        id="shell-branch-switcher"
        className="shell-branch-switcher-input"
        value={selectedBranchId ?? ''}
        onChange={(event) => setSelectedBranchId(event.target.value || null)}
        aria-label={copy.aria}
      >
        {availableBranches.map((branch) => (
          <option key={branch.id} value={branch.id}>
            {branch.name}
          </option>
        ))}
      </select>
    </div>
  );
}
