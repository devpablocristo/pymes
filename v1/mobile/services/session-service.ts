import { api } from './api';
import { SessionResponse } from './types';

export const sessionService = {
  get: () => api.get<SessionResponse>('/session'),
};
