import { Suspense, lazy, useEffect } from 'react';
import { ToastContainer } from 'react-toastify';

// Código del template en JSX (sin tsc estricto); el bundle se separa en `wowdash-template`.
const WowdashApp = lazy(() => import('#wowdash/App'));

const LAB_HEAD_ATTR = 'data-pymes-wowdash-lab';

/**
 * Plantilla Wowdash completa bajo `/console/wowdash/*` (canónico). `/labs/wowdash/*` redirige aquí.
 * El CSS se sirve encapsulado en `#wowdash-template-root` vía `wowdash-assets/css/pymes-scoped.css`.
 * Quill/KaTeX del editor van en ese bundle (no import global de react-quill).
 */
export function WowdashTemplateLabsPage() {
  useEffect(() => {
    document.documentElement.classList.add('wowdash-lab-active');

    const appendHead = (el: HTMLElement) => {
      el.setAttribute(LAB_HEAD_ATTR, 'true');
      document.head.appendChild(el);
    };

    const scopedId = 'wowdash-pymes-scoped-css';
    if (!document.getElementById(scopedId)) {
      const link = document.createElement('link');
      link.id = scopedId;
      link.rel = 'stylesheet';
      link.href = '/wowdash-assets/css/pymes-scoped.css';
      appendHead(link);
    }

    const interId = 'wowdash-lab-font-inter';
    if (!document.getElementById(interId)) {
      const preG = document.createElement('link');
      preG.rel = 'preconnect';
      preG.href = 'https://fonts.googleapis.com';
      appendHead(preG);
      const preS = document.createElement('link');
      preS.rel = 'preconnect';
      preS.href = 'https://fonts.gstatic.com';
      preS.crossOrigin = 'anonymous';
      appendHead(preS);
      const inter = document.createElement('link');
      inter.id = interId;
      inter.rel = 'stylesheet';
      inter.href =
        'https://fonts.googleapis.com/css2?family=Inter:ital,opsz,wght@0,14..32,100..900;1,14..32,100..900&display=swap';
      appendHead(inter);
    }

    void import('bootstrap/dist/js/bootstrap.bundle.min.js');
    void import('react-toastify/dist/ReactToastify.css');
    void import('react-modal-video/css/modal-video.min.css');
    void import('jquery').then((mod) => {
      const $ = mod.default;
      const w = window as unknown as { jQuery: typeof $; $: typeof $ };
      w.jQuery = $;
      w.$ = $;
    });
    return () => {
      document.documentElement.classList.remove('wowdash-lab-active');
      document.querySelectorAll(`[${LAB_HEAD_ATTR}="true"]`).forEach((el) => el.remove());
    };
  }, []);

  return (
    <div id="wowdash-template-root" className="wowdash-template-root">
      <Suspense
        fallback={
          <div style={{ padding: '2rem', fontFamily: 'system-ui' }} role="status">
            Cargando plantilla Wowdash…
          </div>
        }
      >
        <WowdashApp />
      </Suspense>
      <ToastContainer position="top-right" autoClose={5000} theme="light" />
    </div>
  );
}
