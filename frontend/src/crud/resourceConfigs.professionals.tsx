/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
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
