import type { AppShellNavSection } from "@devpablocristo/platform-ui-page-shell";
import { PageShellFrame } from "@devpablocristo/platform-ui-page-shell";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { HomeIcon, MoonIcon, SunIcon } from "../icons";
import { useI18n } from "../providers/I18nProvider";
import { useTheme } from "../providers/ThemeProvider";

export function ProductShell() {
  const location = useLocation();
  const { language, setLanguage, t } = useI18n();
  const { theme, toggleTheme } = useTheme();

  const sections: AppShellNavSection[] = [
    {
      label: t("nav.section"),
      items: [
        {
          to: "/dashboard",
          label: t("nav.home"),
          icon: <HomeIcon aria-hidden="true" />,
          end: true,
        },
      ],
    },
  ];

  return (
    <PageShellFrame
      brandTitle={
        <span className="product-wordmark">
          <img className="product-wordmark__light" src="/assets/logo.svg" alt={t("app.name")} />
          <img className="product-wordmark__dark" src="/assets/logo-dark.svg" alt={t("app.name")} />
        </span>
      }
      brandIcon={<img src="/assets/iso.svg" alt="" />}
      brandSubtitle={t("app.subtitle")}
      sections={sections}
      pathname={`${location.pathname}${location.search}`}
      formatLabel={(label) => label}
      renderLink={(item, className) => (
        <NavLink
          key={item.to}
          className={({ isActive }) => `${className}${isActive ? " active" : ""}`}
          end={item.end}
          to={item.to}
        >
          {item.icon}
          <span>{item.label}</span>
        </NavLink>
      )}
      searchPlaceholder={t("nav.search")}
      shellLabels={{
        clearSearch: t("shell.clearSearch"),
        noSearchResults: t("shell.noResults"),
        collapseSidebar: t("shell.collapse"),
        expandSidebar: t("shell.expand"),
        openNavigation: t("shell.open"),
        closeNavigation: t("shell.close"),
        navigation: t("shell.navigation"),
      }}
      skipLinkLabel={t("shell.skip")}
      footerContent={
        <div className="shell-preferences">
          <button
            type="button"
            className="shell-preference"
            aria-label={t("preferences.language")}
            onClick={() => setLanguage(language === "es" ? "en" : "es")}
          >
            <span className="shell-preference__code">{language.toUpperCase()}</span>
          </button>
          <button
            type="button"
            className="shell-preference"
            aria-label={theme === "light" ? t("preferences.themeDark") : t("preferences.themeLight")}
            onClick={toggleTheme}
          >
            {theme === "light" ? <MoonIcon aria-hidden="true" /> : <SunIcon aria-hidden="true" />}
          </button>
        </div>
      }
    >
      <Outlet />
    </PageShellFrame>
  );
}
