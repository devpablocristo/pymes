import type { AppShellNavSection } from "@devpablocristo/platform-ui-page-shell";
import { PageShellFrame } from "@devpablocristo/platform-ui-page-shell";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useProductAuth } from "../../auth/AuthContext";
import {
  HomeIcon,
  LogOutIcon,
  MoonIcon,
  OrganizationIcon,
  SessionsIcon,
  SunIcon,
  TeamIcon,
} from "../icons";
import { useI18n } from "../providers/I18nProvider";
import { useTheme } from "../providers/ThemeProvider";

export function ProductShell() {
  const location = useLocation();
  const auth = useProductAuth();
  const { language, setLanguage, t } = useI18n();
  const { theme, toggleTheme } = useTheme();
  const activeOrganization = auth.organizations.find(
    (organization) => organization.id === auth.activeOrganizationId,
  );
  const userName = auth.user?.displayName || t("account.userFallback");
  const accountInitial = userName.trim().slice(0, 1).toUpperCase() || "P";
  const isGlobalOwner = auth.productRole === "owner";

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
    ...(isGlobalOwner
      ? [
          {
            label: "Administración",
            items: [
            {
              to: "/admin/tenants",
              label: "Tenants",
              icon: <OrganizationIcon aria-hidden="true" />,
            },
            {
              to: "/admin/users",
              label: "Usuarios",
              icon: <TeamIcon aria-hidden="true" />,
            },
            ],
          },
        ]
      : []),
    {
      label: t("nav.settings"),
      items: [
        ...(activeOrganization
          ? [
              {
                to: "/settings/organization",
                label: t("nav.organization"),
                icon: <OrganizationIcon aria-hidden="true" />,
              },
              {
                to: "/settings/team",
                label: t("nav.team"),
                icon: <TeamIcon aria-hidden="true" />,
              },
            ]
          : []),
        {
          to: "/settings/sessions",
          label: t("nav.sessions"),
          icon: <SessionsIcon aria-hidden="true" />,
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
        <div className="shell-footer-stack">
          <div
            className="shell-account"
            role="group"
            aria-label={`${t("account.session")}: ${
              activeOrganization?.name || t("account.organizationFallback")
            }, ${userName}`}
          >
            <span className="shell-account__avatar" aria-hidden="true">
              {auth.user?.avatarUrl ? <img src={auth.user.avatarUrl} alt="" /> : accountInitial}
            </span>
            <span className="shell-account__copy">
              <strong>{activeOrganization?.name || t("account.organizationFallback")}</strong>
              <small>{userName}</small>
            </span>
            <button
              type="button"
              className="shell-account__sign-out"
              aria-label={t("account.signOut")}
              title={t("account.signOut")}
              onClick={() => void auth.signOut()}
            >
              <LogOutIcon aria-hidden="true" />
            </button>
          </div>
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
        </div>
      }
    >
      <Outlet />
    </PageShellFrame>
  );
}
