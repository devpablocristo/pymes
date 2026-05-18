import { api } from './api';
import { setAuthToken } from './token';

type LoginResponse = {
  token: string;
};

export const authService = {
  login: async (email: string, password: string): Promise<void> => {
    const { token } = await api.post<LoginResponse>('/auth/login', { email, password });
    await setAuthToken(token);
  },
};
