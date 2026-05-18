import type { AxiosRequestConfig } from "axios";
import { create } from "axios";

import { getAuthToken, getOrgSlug } from "./token";

if (!process.env.EXPO_PUBLIC_API_URL) {
  throw new Error("EXPO_PUBLIC_API_URL is not defined in .env");
}

const client = create({
  baseURL: process.env.EXPO_PUBLIC_API_URL,
  headers: {
    "Content-Type": "application/json",
  },
});

// Inject JWT before every request.
client.interceptors.request.use(async (config) => {
  const token = await getAuthToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  const slug = getOrgSlug();
  if (slug) {
    config.headers["x-pymes-tenant-slug"] = slug;
  }
  return config;
});

// Normalize error messages
client.interceptors.response.use(
  (response) => {
    return response;
  },
  (error) => {
    const message =
      error.response?.data?.message ?? error.message ?? "Unknown error";
    return Promise.reject(new Error(message));
  },
);

export const api = {
  get: <T>(path: string, config?: AxiosRequestConfig) =>
    client.get<T>(path, config).then((r) => r.data),

  post: <T>(path: string, body?: unknown, config?: AxiosRequestConfig) =>
    client.post<T>(path, body, config).then((r) => r.data),

  patch: <T>(path: string, body?: unknown, config?: AxiosRequestConfig) =>
    client.patch<T>(path, body, config).then((r) => r.data),

  delete: <T>(path: string, config?: AxiosRequestConfig) =>
    client.delete<T>(path, config).then((r) => r.data),
};
