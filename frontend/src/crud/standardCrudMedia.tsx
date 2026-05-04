import { useMemo, useRef } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import type { CrudEditorModalFieldConfig } from '../components/CrudPage';
import { asCrudString } from '../lib/formPresets';
import { CrudEntityMediaCarousel } from '../modules/crud/CrudEntityMediaCarousel';
import { parseCrudLinkedEntityImageUrlList } from '../modules/crud/crudLinkedEntityImageUrls';

function appendHttpsUrlFromPrompt(
  value: CrudFieldValue | undefined,
  setValue: (nextValue: CrudFieldValue) => void,
): void {
  const raw = window.prompt('Pegá la URL de la imagen (https:// o http://):')?.trim();
  if (!raw) return;
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    return;
  }
  if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return;
  const normalized = parsed.href;
  const current = parseCrudLinkedEntityImageUrlList(asCrudString(value));
  if (current.includes(normalized)) return;
  setValue([...current, normalized].join('\n'));
}

/** Editor único: vista previa, archivos locales, enlace por prompt. Componente aparte por reglas de Hooks. */
export function StandardCrudImageUrlsEditor(props: {
  value: CrudFieldValue | undefined;
  setValue: (nextValue: CrudFieldValue) => void;
}) {
  const text = asCrudString(props.value);
  const urls = useMemo(() => parseCrudLinkedEntityImageUrlList(text), [text]);
  const fileRef = useRef<HTMLInputElement>(null);

  const removeAt = (index: number) => {
    const next = urls.filter((_, j) => j !== index);
    props.setValue(next.join('\n'));
  };

  return (
    <div className="crud-image-urls-editor">
      {urls.length > 0 ? (
        <CrudEntityMediaCarousel
          urls={urls}
          variant="edit"
          ariaLabel="Vista previa de imágenes adjuntas"
          onRequestRemoveAt={removeAt}
        />
      ) : null}

      <div className="crud-image-urls-editor__pick-row">
        <button type="button" className="btn btn-primary" onClick={() => fileRef.current?.click()}>
          Seleccionar imágenes del equipo…
        </button>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => appendHttpsUrlFromPrompt(props.value, props.setValue)}
        >
          Añadir enlace…
        </button>
        <input
          ref={fileRef}
          type="file"
          accept="image/*"
          multiple
          hidden
          aria-label="Seleccionar imágenes desde la computadora (varias a la vez)"
          onChange={(event) => {
            const input = event.target as HTMLInputElement;
            void (async () => {
              const files = Array.from(input.files ?? []);
              if (!files.length) return;
              try {
                const encoded = await Promise.all(
                  files.map(
                    (file) =>
                      new Promise<string>((resolve, reject) => {
                        const reader = new FileReader();
                        reader.onload = () => resolve(String(reader.result ?? ''));
                        reader.onerror = () => reject(reader.error ?? new Error('upload_failed'));
                        reader.readAsDataURL(file);
                      }),
                  ),
                );
                const current = parseCrudLinkedEntityImageUrlList(asCrudString(props.value));
                props.setValue([...current, ...encoded].join('\n'));
              } finally {
                input.value = '';
              }
            })();
          }}
        />
      </div>
    </div>
  );
}

export function buildStandardCrudImageUrlsModalFieldConfig(
  overrides?: Partial<CrudEditorModalFieldConfig>,
): CrudEditorModalFieldConfig {
  return {
    fullWidth: true,
    editControl: ({ value, setValue }) => <StandardCrudImageUrlsEditor value={value} setValue={setValue} />,
    ...overrides,
  };
}
