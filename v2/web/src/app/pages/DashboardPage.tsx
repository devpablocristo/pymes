import { useI18n } from "../providers/I18nProvider";
import { EmptyState } from "../states/ContentStates";

export function DashboardPage() {
  const { t } = useI18n();

  return (
    <div className="dashboard-page">
      <header className="page-topbar">
        <div>
          <h1>{t("dashboard.eyebrow")}</h1>
          <small>Dashboard</small>
        </div>
        <span className="readiness-tag">
          <span aria-hidden="true" />
          {t("dashboard.emptyTag")}
        </span>
      </header>

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
