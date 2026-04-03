import { describe, it, expect, vi, beforeEach } from 'vitest';

const mockStorage = vi.hoisted(() => ({
  getJSON: vi.fn(),
  setJSON: vi.fn(),
  remove: vi.fn(),
  getString: vi.fn(),
  setString: vi.fn(),
}));

vi.mock('@devpablocristo/core-browser/storage', () => ({
  createBrowserStorageNamespace: () => mockStorage,
}));

// vocab imports tenantProfile which uses the mocked storage
import { vocab } from './vocabulary';

beforeEach(() => {
  vi.clearAllMocks();
});

describe('vocab', () => {
  it('returns text unchanged when no profile', () => {
    mockStorage.getJSON.mockReturnValue(null);
    expect(vocab('Lista de clientes')).toBe('Lista de clientes');
  });

  it('returns text unchanged when clientLabel is clientes (default)', () => {
    mockStorage.getJSON.mockReturnValue({ clientLabel: 'clientes' });
    expect(vocab('Lista de clientes')).toBe('Lista de clientes');
  });

  it('replaces clientes with pacientes (lowercase)', () => {
    mockStorage.getJSON.mockReturnValue({ clientLabel: 'pacientes' });
    expect(vocab('Lista de clientes')).toBe('Lista de pacientes');
  });

  it('replaces Clientes with Pacientes (capitalized)', () => {
    mockStorage.getJSON.mockReturnValue({ clientLabel: 'pacientes' });
    expect(vocab('Clientes activos')).toBe('Pacientes activos');
  });

  it('replaces singular cliente with paciente', () => {
    mockStorage.getJSON.mockReturnValue({ clientLabel: 'pacientes' });
    expect(vocab('Nuevo cliente')).toBe('Nuevo paciente');
  });

  it('replaces singular Cliente with Paciente', () => {
    mockStorage.getJSON.mockReturnValue({ clientLabel: 'pacientes' });
    expect(vocab('Cliente encontrado')).toBe('Paciente encontrado');
  });

  it('replaces with alumnos', () => {
    mockStorage.getJSON.mockReturnValue({ clientLabel: 'alumnos' });
    expect(vocab('Buscar clientes')).toBe('Buscar alumnos');
    expect(vocab('Editar cliente')).toBe('Editar alumno');
  });
});
