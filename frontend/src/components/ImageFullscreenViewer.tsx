import { useEffect } from 'react';
import { createPortal } from 'react-dom';
import './ImageFullscreenViewer.css';

export type ImageFullscreenViewerProps = {
  imageUrl: string | null;
  onClose: () => void;
  /** Etiqueta accesible (p. ej. nombre del producto). */
  contentLabel?: string;
};

/**
 * Capa por encima del resto de la UI: imagen lo más grande que quepa en la ventana.
 */
export function ImageFullscreenViewer({ imageUrl, onClose, contentLabel }: ImageFullscreenViewerProps) {
  useEffect(() => {
    if (!imageUrl) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [imageUrl, onClose]);

  if (!imageUrl) return null;

  const node = (
    <div className="img-fs-root">
      <button type="button" className="img-fs-backdrop" aria-label="Cerrar vista ampliada" onClick={onClose} />
      <div
        className="img-fs-frame"
        role="dialog"
        aria-modal="true"
        aria-label={contentLabel ?? 'Imagen ampliada'}
      >
        <button type="button" className="img-fs-close" onClick={onClose} aria-label="Cerrar">
          ×
        </button>
        <img
          src={imageUrl}
          alt=""
          className="img-fs-img"
          onClick={(e) => {
            e.stopPropagation();
            onClose();
          }}
        />
      </div>
    </div>
  );

  return createPortal(node, document.body);
}
