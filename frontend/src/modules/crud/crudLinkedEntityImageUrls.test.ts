import { describe, expect, it } from 'vitest';
import { parseCrudLinkedEntityImageUrlList, pickGalleryHeroCrudImageSrc } from './crudLinkedEntityImageUrls';

describe('parseCrudLinkedEntityImageUrlList', () => {
  it('preserva data URLs completas (coma después de base64)', () => {
    const data = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
    expect(parseCrudLinkedEntityImageUrlList(data)).toEqual([data]);
  });

  it('separa solo por líneas', () => {
    const a = 'https://a.example/a.png';
    const b = 'data:image/png;base64,QQ==';
    expect(parseCrudLinkedEntityImageUrlList(`${a}\n${b}`)).toEqual([a, b]);
  });

  it('elimina duplicados conservando orden', () => {
    expect(parseCrudLinkedEntityImageUrlList('https://x/z\nhttps://x/z')).toEqual(['https://x/z']);
  });
});

describe('pickGalleryHeroCrudImageSrc', () => {
  it('elige la última URL https válida', () => {
    expect(
      pickGalleryHeroCrudImageSrc({
        image_urls: ['https://a.example/old.png', 'https://b.example/new.png'],
      }),
    ).toBe('https://b.example/new.png');
  });

  it('prefiere la última data URL renderizable si está al final', () => {
    const first = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==';
    const second = 'data:image/png;base64,QQ==';
    expect(pickGalleryHeroCrudImageSrc({ image_urls: [first, second] })).toBe(second);
  });

  it('ignora entradas no mostrables y toma la anterior', () => {
    expect(
      pickGalleryHeroCrudImageSrc({
        image_urls: ['https://ok.example/x.png', 'not-a-url'],
      }),
    ).toBe('https://ok.example/x.png');
  });
});
