import { Link } from "react-router-dom";
import { ArrowLeftIcon } from "../icons";
import { useI18n } from "../providers/I18nProvider";

export function NotFoundPage() {
  const { t } = useI18n();

  return (
    <div className="not-found">
      <p className="not-found__code">{t("notFound.code")}</p>
      <h1>{t("notFound.title")}</h1>
      <p>{t("notFound.body")}</p>
      <Link className="button button--quiet" to="/dashboard">
        <ArrowLeftIcon aria-hidden="true" />
        {t("notFound.action")}
      </Link>
    </div>
  );
}
