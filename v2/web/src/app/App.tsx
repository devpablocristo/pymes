import { useI18n } from "./providers/I18nProvider";
import { useTheme } from "./providers/ThemeProvider";

export function App() {
  const { locale, setLocale, text } = useI18n();
  const { theme, toggleTheme } = useTheme();

  return (
    <div className="app-shell">
      <header className="topbar">
        <a className="brand" href="#main" aria-label="Pymes v2">
          <span className="brand-mark" aria-hidden="true">P</span>
          <span>Pymes</span>
          <span className="version-pill">v2</span>
        </a>

        <nav className="primary-nav" aria-label={text.navigation}>
          <a href="#runtime">{text.runtime}</a>
          <a href="#principles">{text.principles}</a>
        </nav>

        <div className="shell-actions">
          <label className="language-control">
            <span className="visually-hidden">{text.language}</span>
            <select
              aria-label={text.language}
              value={locale}
              onChange={(event) => setLocale(event.target.value === "en" ? "en" : "es")}
            >
              <option value="es">ES</option>
              <option value="en">EN</option>
            </select>
          </label>
          <button className="theme-toggle" type="button" onClick={toggleTheme}>
            {theme === "light" ? text.darkTheme : text.lightTheme}
          </button>
        </div>
      </header>

      <main id="main" className="main-content">
        <section id="runtime" className="hero" aria-labelledby="runtime-title">
          <div className="hero-copy">
            <p className="eyebrow">V2-RUNTIME-01</p>
            <h1 id="runtime-title">{text.title}</h1>
            <p className="hero-description">{text.description}</p>
          </div>

          <dl className="runtime-status" aria-label={text.statusLabel}>
            <div>
              <dt>API</dt>
              <dd><span className="status-dot" aria-hidden="true" /> Go</dd>
            </div>
            <div>
              <dt>Database</dt>
              <dd><span className="status-dot" aria-hidden="true" /> PostgreSQL</dd>
            </div>
            <div>
              <dt>Web</dt>
              <dd><span className="status-dot" aria-hidden="true" /> React</dd>
            </div>
          </dl>
        </section>

        <section id="principles" className="principles" aria-labelledby="principles-title">
          <p className="section-number">01</p>
          <div>
            <h2 id="principles-title">{text.principlesTitle}</h2>
            <p>{text.principlesDescription}</p>
          </div>
        </section>
      </main>

      <footer className="footer">
        <span>{text.foundation}</span>
        <span aria-hidden="true">·</span>
        <span>{text.noLegacy}</span>
      </footer>
    </div>
  );
}
