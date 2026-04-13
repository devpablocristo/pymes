import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

const { useUser } = vi.hoisted(() => ({
  useUser: vi.fn(() => {
    throw new Error('useUser no debería ejecutarse sin Clerk');
  }),
}));

vi.mock('./auth', () => ({
  clerkEnabled: false,
}));

vi.mock('@clerk/react', () => ({
  useUser,
}));

import { useCrudListCreatedByMerge } from './useCrudListCreatedByMerge';

describe('useCrudListCreatedByMerge', () => {
  it('no toca Clerk cuando el runtime está sin Clerk', () => {
    const { result } = renderHook(() => useCrudListCreatedByMerge());

    expect(result.current).toEqual({});
    expect(useUser).not.toHaveBeenCalled();
  });
});
