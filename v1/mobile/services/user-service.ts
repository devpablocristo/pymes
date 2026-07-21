import { api } from './api';
import { UpdateProfileBody, UserProfile } from './types';

export const userService = {
  getMe: () => api.get<UserProfile>('/users/me'),
  updateProfile: (body: UpdateProfileBody) => api.patch<UserProfile>('/users/me/profile', body),
};
