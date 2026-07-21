import type { CSSProperties } from 'react';

type SkeletonVariant = 'text' | 'rect' | 'circle';

type SkeletonProps = {
  /** "text" (block default 1em), "rect" (block libre) o "circle" (avatar). */
  variant?: SkeletonVariant;
  /** Ancho CSS del shimmer (px, %, em, var). Default: 100%. */
  width?: string | number;
  /** Alto CSS del shimmer. Default según variant. */
  height?: string | number;
  /** Cantidad de líneas a renderizar (solo para `variant="text"`). */
  lines?: number;
  /** Clase adicional sobre `.skeleton`. */
  className?: string;
  /** Estilos inline adicionales. */
  style?: CSSProperties;
};

const defaultHeight: Record<SkeletonVariant, string> = {
  text: '1em',
  rect: '120px',
  circle: '40px',
};

/**
 * Skeleton — placeholder con shimmer mientras carga data.
 *
 * Usa la clase `.skeleton` (animation `skeleton-shimmer`) definida en
 * `styles/components.css`. La forma se controla con `variant` y opcional
 * `width` / `height`. Para múltiples líneas en `variant="text"`, pasar
 * `lines` (default 1).
 *
 * @example
 *   <Skeleton variant="text" lines={3} />
 *   <Skeleton variant="circle" width={48} height={48} />
 *   <Skeleton variant="rect" width="100%" height={180} />
 */
export function Skeleton({
  variant = 'text',
  width,
  height,
  lines = 1,
  className,
  style,
}: SkeletonProps) {
  const cls = `skeleton skeleton--${variant}${className ? ` ${className}` : ''}`;
  const baseStyle: CSSProperties = {
    width: width ?? '100%',
    height: height ?? defaultHeight[variant],
    borderRadius: variant === 'circle' ? '50%' : undefined,
    ...style,
  };

  if (variant === 'text' && lines > 1) {
    return (
      <span aria-hidden="true" style={{ display: 'block' }}>
        {Array.from({ length: lines }, (_, i) => (
          <span
            key={i}
            className={cls}
            style={{
              ...baseStyle,
              display: 'block',
              marginBottom: i < lines - 1 ? '0.5em' : 0,
              // Última línea más corta para ritmo visual.
              width: i === lines - 1 ? '60%' : baseStyle.width,
            }}
          />
        ))}
      </span>
    );
  }

  return <span aria-hidden="true" className={cls} style={baseStyle} />;
}
