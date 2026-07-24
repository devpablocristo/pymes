import type { AppShellNavSection } from "@devpablocristo/platform-ui-page-shell";
import { PageShellFrame } from "@devpablocristo/platform-ui-page-shell";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { useProductAuth } from "../../auth/AuthContext";
import { FiscalIcon, HomeIcon, LedgerIcon, TeamIcon } from "../icons";
import { useI18n } from "../providers/I18nProvider";

export function ProductShell() {
  const location = useLocation();
  const auth = useProductAuth();
  const { t } = useI18n();
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
    ...(auth.activeOrganizationId
      ? [
          {
            label: t("nav.teamSection"),
            items: [
              {
                to: "/employees",
                label: t("nav.employees"),
                icon: <TeamIcon aria-hidden="true" />,
              },
            ],
          },
          {
            label: t("nav.managementSection"),
            items: [
              {
                to: "/accounting",
                label: t("nav.accounting"),
                icon: <LedgerIcon aria-hidden="true" />,
              },
              {
                to: "/fiscal",
                label: t("nav.fiscal"),
                icon: <FiscalIcon aria-hidden="true" />,
              },
            ],
          },
        ]
      : []),
    ...(isGlobalOwner
      ? [
          {
            label: "Administración",
            items: [
              {
                to: "/admin/users",
                label: "Usuarios y Tenants",
                icon: <TeamIcon aria-hidden="true" />,
              },
            ],
          },
        ]
      : []),
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
    >
      <Outlet />
    </PageShellFrame>
  );
}
