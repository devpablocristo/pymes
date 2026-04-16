/* eslint-disable react-refresh/only-export-components -- archivo de configuración CRUD, no se hot-reloads */
import { defineCrudDomain } from './defineCrudDomain';

// Beauty ya no registra recursos propios: equipo vive en core (/v1/parties?role=employee)
// y servicios viven en core (/v1/services con metadata.vertical=beauty).
export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({});
