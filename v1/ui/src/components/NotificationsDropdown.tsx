import { useContext, useEffect, useRef, useState } from 'react';
import { QueryClientContext, useQuery } from '@tanstack/react-query';
import { IconBell, IconX } from '@tabler/icons-react';
import { getNotificationsSummary } from '../lib/api';
import { queryKeys } from '../lib/queryKeys';
import { NotificationsCenterPage } from '../pages/NotificationsCenterPage';
import './NotificationsDropdown.css';

/**
 * Campana con panel desplegable de notificaciones.
 *
 * Pensado para superficies como la topbar azul; embebe la
 * NotificationsCenterPage en modo `embedded` y muestra un badge con el
 * conteo de no leídas. Cierra con click fuera del panel o tecla Escape.
 *
 * Wooko cherry-pick (Phase 11): la lógica viene del componente original
 * pero los iconos se cambian a `@tabler/icons-react` (no CDN webfont)
 * para mantener consistencia con el resto del UI kit.
 */
export function NotificationsDropdown() {
  const queryClient = useContext(QueryClientContext);
  if (!queryClient) {
    return (
      <button type="button" className="topbar-icon-btn" aria-label="Notificaciones">
        <IconBell size={18} stroke={1.6} aria-hidden="true" />
      </button>
    );
  }

  return <NotificationsDropdownWithQuery />;
}

function NotificationsDropdownWithQuery() {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  const { data } = useQuery({
    queryKey: queryKeys.notifications.summary,
    queryFn: getNotificationsSummary,
    refetchInterval: 60_000,
  });
  const unreadCount = data?.unread_count ?? 0;

  // Cerrar al hacer clic fuera del panel
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (e: PointerEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => document.removeEventListener('pointerdown', onPointerDown);
  }, [open]);

  // Cerrar con Escape
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open]);

  return (
    <div className="notif-dropdown" ref={rootRef}>
      <button
        type="button"
        className="topbar-icon-btn"
        aria-label="Notificaciones"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((v) => !v)}
      >
        <IconBell size={18} stroke={1.6} aria-hidden="true" />
        {unreadCount > 0 && (
          <span className="notif-dropdown__dot" aria-hidden="true">
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div
          className="notif-dropdown__panel"
          role="dialog"
          aria-label="Notificaciones"
          aria-modal="false"
        >
          <div className="notif-dropdown__header">
            <span className="notif-dropdown__title">
              <IconBell size={18} stroke={1.6} aria-hidden="true" />
              Notificaciones
              {unreadCount > 0 && (
                <span className="notif-dropdown__count">{unreadCount > 99 ? '99+' : unreadCount}</span>
              )}
            </span>
            <button
              type="button"
              className="notif-dropdown__close"
              aria-label="Cerrar notificaciones"
              onClick={() => setOpen(false)}
            >
              <IconX size={18} stroke={1.6} />
            </button>
          </div>

          <div className="notif-dropdown__body">
            <NotificationsCenterPage embedded />
          </div>
        </div>
      )}
    </div>
  );
}
