import {
  createIntakesCrudConfig,
  createProfessionalsCrudConfig,
  createSessionsCrudConfig,
  createSpecialtiesCrudConfig,
} from '../modules/scheduling';
import { defineCrudDomain } from './defineCrudDomain';

const professionalsConfig = createProfessionalsCrudConfig();

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({
  professionals: professionalsConfig,
  teachers: professionalsConfig,
  specialties: createSpecialtiesCrudConfig(),
  intakes: createIntakesCrudConfig(),
  sessions: createSessionsCrudConfig(),
});
