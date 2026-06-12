import * as SecureStore from 'expo-secure-store';

// Custom token cache for ClerkProvider that uses expo-secure-store directly.
// Avoids @clerk/clerk-expo/token-cache which requires ExpoCryptoAES (not available in Expo Go).
export const tokenCache = {
  getToken: (key: string) => SecureStore.getItemAsync(key),
  saveToken: (key: string, value: string) => SecureStore.setItemAsync(key, value),
  clearToken: (key: string) => SecureStore.deleteItemAsync(key),
};
