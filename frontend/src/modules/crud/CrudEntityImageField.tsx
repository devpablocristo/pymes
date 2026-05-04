import { useId, useMemo, type ChangeEvent } from 'react';
import type { CrudFieldValue } from '@devpablocristo/modules-crud-ui';
import { CrudEntityMediaCarousel } from './CrudEntityMediaCarousel';
import { parseCrudLinkedEntityImageUrlList } from './crudLinkedEntityImageUrls';
import './CrudEntityImageField.css';

function asImageFieldString(value: CrudFieldValue | undefined): string {
  if (Array.isArray(value)) {
    return value
      .filter((entry): entry is string => typeof entry === 'string')
      .join('\n');
  }
  return typeof value === 'string' ? value : '';
}

function stringifyImageField(urls: string[]): string {
  return urls.join('\n');
}

async function fileToDataUrl(file: File): Promise<string> {
  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result ?? ''));
    reader.onerror = () => reject(reader.error ?? new Error('upload_failed'));
    reader.readAsDataURL(file);
  });
}

export function CrudEntityImageField({
  value,
  setValue,
  readOnly = false,
  label = 'Cargar imágenes',
}: {
  value: CrudFieldValue | undefined;
  setValue: (nextValue: CrudFieldValue) => void;
  readOnly?: boolean;
  label?: string;
}) {
  const urls = useMemo(() => parseCrudLinkedEntityImageUrlList(asImageFieldString(value)), [value]);
  const inputId = useId();

  const handleUpload = async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? []);
    if (!files.length) return;
    try {
      const encoded = await Promise.all(files.map(fileToDataUrl));
      setValue(stringifyImageField([...urls, ...encoded]));
    } finally {
      event.currentTarget.value = '';
    }
  };

  const handleRemoveAt = (removeIndex: number) => {
    const nextUrls = urls.filter((_, indexToKeep) => indexToKeep !== removeIndex);
    setValue(stringifyImageField(nextUrls));
  };

  return (
    <div className="crud-entity-image-field">
      {!readOnly ? (
        <>
          <input
            id={inputId}
            className="crud-entity-image-field__input"
            type="file"
            accept="image/*"
            multiple
            onChange={(event) => {
              void handleUpload(event);
            }}
          />
          <label htmlFor={inputId} className="crud-entity-image-field__button">
            {label}
          </label>
        </>
      ) : null}
      {urls.length ? (
        <CrudEntityMediaCarousel
          urls={urls}
          variant={readOnly ? 'read' : 'edit'}
          onRequestRemoveAt={readOnly ? undefined : handleRemoveAt}
        />
      ) : null}
    </div>
  );
}
