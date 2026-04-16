import type { CrudViewModeId } from '../components/CrudPage';
import { ConfiguredCrudSection } from '../crud/configuredCrudViews';

export function ConfiguredCrudSectionPage({
  resourceId,
  baseRoute,
  contextPatternByModeId,
  actionLink,
  includeCanonicalMissing,
}: {
  resourceId: string;
  baseRoute: string;
  contextPatternByModeId?: Partial<Record<CrudViewModeId, string>>;
  actionLink?: {
    to: string;
    label: string;
    hideWhenActivePattern?: string;
    activeReplacement?: {
      to: string;
      label: string;
    };
  };
  includeCanonicalMissing?: boolean;
}) {
  return (
    <ConfiguredCrudSection
      resourceId={resourceId}
      baseRoute={baseRoute}
      contextPatternByModeId={contextPatternByModeId}
      actionLink={actionLink}
      includeCanonicalMissing={includeCanonicalMissing}
    />
  );
}
