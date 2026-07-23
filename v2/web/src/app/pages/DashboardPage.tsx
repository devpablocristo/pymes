import { useI18n } from "../providers/I18nProvider";
import { EmptyState } from "../states/ContentStates";

export function DashboardPage() {
  const { t } = useI18n();

  return (
    <div className="dashboard-page">
      <header className="page-intro">
        <div>
          <p className="page-intro__eyebrow">{t("dashboard.eyebrow")}</p>
          <h1>{t("dashboard.title")}</h1>
          <p>{t("dashboard.description")}</p>
        </div>
        <span className="readiness-tag">
          <span aria-hidden="true" />
          {t("dashboard.emptyTag")}
        </span>
      </header>

      <div className="dashboard-canvas">
        <div className="dashboard-canvas__rule" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>
        <EmptyState title={t("dashboard.emptyTitle")} body={t("dashboard.emptyBody")} />
      </div>
    </div>
  );
}
