import { createOccupationalHealthExamsCrudConfig } from '../modules/medical/occupationalHealthExamCrudConfig';
import { defineCrudDomain } from './defineCrudDomain';

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({
  occupationalHealthExams: createOccupationalHealthExamsCrudConfig(),
});
