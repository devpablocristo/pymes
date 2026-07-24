import { useI18n } from "../providers/I18nProvider";
import { SectionHeader } from "../shell/SectionChrome";
import { EmptyState } from "../states/ContentStates";

export function DashboardPage() {
  const { t } = useI18n();

  return (
    <div className="dashboard-page">
      <SectionHeader title={t("dashboard.eyebrow")} subtitle="Dashboard" />

      <div className="dashboard-canvas">
        <div className="dashboard-canvas__header">
          <div>
            <h2>{t("dashboard.title")}</h2>
            <p>{t("dashboard.description")}</p>
          </div>
        </div>
        <EmptyState title={t("dashboard.emptyTitle")} body={t("dashboard.emptyBody")} />
      </div>
    </div>
  );
}
