import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import './CrudImageFullscreenViewer.css';

export function CrudImageFullscreenViewer({
  imageUrl,
  onClose,
  contentLabel,
}: {
  imageUrl: string | null;
  onClose: () => void;
  contentLabel?: string;
}) {
  useEffect(() => {
    if (!imageUrl) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [imageUrl, onClose]);

  if (!imageUrl) return null;

  return createPortal(
    <div className="crud-image-fs-root">
      <button type="button" className="crud-image-fs-backdrop" aria-label="Cerrar vista ampliada" onClick={onClose} />
      <div className="crud-image-fs-frame" role="dialog" aria-modal="true" aria-label={contentLabel ?? 'Imagen ampliada'}>
        <button type="button" className="crud-image-fs-close" onClick={onClose} aria-label="Cerrar">
          ×
        </button>
        <img
          src={imageUrl}
          alt=""
          className="crud-image-fs-img"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
        />
      </div>
    </div>,
    document.body,
  );
}
