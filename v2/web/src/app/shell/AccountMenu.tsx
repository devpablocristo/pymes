import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { useProductAuth } from "../../auth/AuthContext";
import { LogOutIcon, MoonIcon, SessionsIcon, SunIcon } from "../icons";
import { useI18n } from "../providers/I18nProvider";
import { useTheme } from "../providers/ThemeProvider";

export function AccountMenu() {
  const auth = useProductAuth();
  const { language, setLanguage, t } = useI18n();
  const { theme, toggleTheme } = useTheme();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const activeOrganization = auth.organizations.find(
    (organization) => organization.id === auth.activeOrganizationId,
  );
  const userName = auth.user?.displayName || t("account.userFallback");
  const initial = userName.trim().slice(0, 1).toUpperCase() || "P";

  useEffect(() => {
    if (!open) return;

    function closeOnOutsideClick(event: PointerEvent) {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    }

    function closeOnEscape(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      setOpen(false);
      triggerRef.current?.focus();
    }

    document.addEventListener("pointerdown", closeOnOutsideClick);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsideClick);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [open]);

  function avatar(className = "account-menu__avatar") {
    return (
      <span className={className} aria-hidden="true">
        <span>{initial}</span>
        {auth.user?.avatarUrl ? (
          <img
            src={auth.user.avatarUrl}
            alt=""
            onError={(event) => {
              event.currentTarget.hidden = true;
            }}
          />
        ) : null}
      </span>
    );
  }

  return (
    <div className="account-menu" ref={rootRef}>
      <button
        ref={triggerRef}
        type="button"
        className="account-menu__trigger"
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label={t("account.openMenu")}
        onClick={() => setOpen((value) => !value)}
      >
        {avatar()}
        <span className="account-menu__trigger-copy">
          <strong>{userName}</strong>
          <small>{activeOrganization?.name || t("account.organizationFallback")}</small>
        </span>
        <span className="account-menu__chevron" aria-hidden="true">⌄</span>
      </button>

      {open ? (
        <div className="account-menu__panel" role="menu" aria-label={t("account.profile")}>
          <div className="account-menu__profile">
            {avatar("account-menu__avatar account-menu__avatar--large")}
            <span>
              <small>{t("account.profile")}</small>
              <strong>{userName}</strong>
              {auth.user?.email ? <span>{auth.user.email}</span> : null}
            </span>
          </div>

          <div className="account-menu__actions">
            <Link role="menuitem" to="/settings/sessions" onClick={() => setOpen(false)}>
              <SessionsIcon aria-hidden="true" />
              <span>{t("nav.sessions")}</span>
            </Link>
            <button
              type="button"
              role="menuitem"
              onClick={() => setLanguage(language === "es" ? "en" : "es")}
            >
              <span className="account-menu__language" aria-hidden="true">
                {language.toUpperCase()}
              </span>
              <span>{t("preferences.language")}</span>
            </button>
            <button type="button" role="menuitem" onClick={toggleTheme}>
              {theme === "light" ? <MoonIcon aria-hidden="true" /> : <SunIcon aria-hidden="true" />}
              <span>
                {theme === "light" ? t("preferences.themeDark") : t("preferences.themeLight")}
              </span>
            </button>
          </div>

          <button
            type="button"
            role="menuitem"
            className="account-menu__sign-out"
            onClick={() => void auth.signOut()}
          >
            <LogOutIcon aria-hidden="true" />
            <span>{t("account.signOut")}</span>
          </button>
        </div>
      ) : null}
    </div>
  );
}
