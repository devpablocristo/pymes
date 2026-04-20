import { describe, expect, it } from 'vitest';
import { collectCrudImageUrls, parseCrudLinkedEntityImageUrlList } from './crudLinkedEntityImageUrls';

describe('crudLinkedEntityImageUrls', () => {
  it('recomposes split data urls from multiline values', () => {
    const value = 'data:image/jpeg;base64\nAAAA\nhttps://example.com/a.jpg';
    expect(parseCrudLinkedEntityImageUrlList(value)).toEqual([
      'data:image/jpeg;base64,AAAA',
      'https://example.com/a.jpg',
    ]);
  });

  it('preserves regular urls from collected values', () => {
    expect(
      collectCrudImageUrls({
        imageUrls: ['https://example.com/a.jpg', ' https://example.com/a.jpg ', 'data:image/png;base64', 'BBBB'],
      }),
    ).toEqual(['https://example.com/a.jpg', 'data:image/png;base64,BBBB']);
  });

  it('rebuilds bare base64 entries using the detected prefix', () => {
    expect(
      collectCrudImageUrls({
        imageUrls: ['data:image/jpeg;base64,/9j/AAAA', '/9j/BBBB', '/9j/CCCC'],
      }),
    ).toEqual([
      'data:image/jpeg;base64,/9j/AAAA',
      'data:image/jpeg;base64,/9j/BBBB',
      'data:image/jpeg;base64,/9j/CCCC',
    ]);
  });
});
