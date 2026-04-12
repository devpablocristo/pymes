import { describe, expect, it } from 'vitest';
import {
  createIntakesCrudConfig,
  createProfessionalsCrudConfig,
  createSessionsCrudConfig,
  createSpecialtiesCrudConfig,
  schedulingSpecialtiesToText,
} from './schedulingHelpers';

describe('schedulingHelpers', () => {
  it('formats specialties text', () => {
    expect(schedulingSpecialtiesToText([{ name: 'Psicologia' }, 'Coaching'])).toBe('Psicologia, Coaching');
    expect(schedulingSpecialtiesToText([])).toBe('---');
  });

  it('builds professionals config with list mode', () => {
    const config = createProfessionalsCrudConfig();
    expect(config.label).toBe('teacher');
    expect(config.viewModes?.[0]?.id).toBe('list');
    expect(config.columns).toHaveLength(4);
  });

  it('builds specialties, intakes and sessions configs', () => {
    expect(createSpecialtiesCrudConfig().labelPlural).toBe('especialidades');
    expect(createIntakesCrudConfig().labelPlural).toBe('ingresos');
    expect(createSessionsCrudConfig().labelPlural).toBe('sesiones');
  });
});
