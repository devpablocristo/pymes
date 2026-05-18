import "@/services/reactotron";
import { ClerkProvider, useAuth } from "@clerk/expo";
import {
  DarkTheme,
  DefaultTheme,
  ThemeProvider as NavThemeProvider,
} from "@react-navigation/native";
import { Stack, useRouter } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useEffect } from "react";
import { StyleSheet, View } from "react-native";
import "react-native-reanimated";

import { Colors } from "@/constants/theme";
import { t } from "@/constants/translations";
import { useColorScheme } from "@/hooks/use-color-scheme";
import {
  registerOrgSlug,
  registerSignOut,
  registerTokenGetter,
} from "@/services/token";
import { tokenCache } from "@/services/token-cache";
import { useAuthStore } from "@/stores/auth-store";

export const unstable_settings = {
  anchor: "(drawer)",
};

const CLERK_PUBLISHABLE_KEY = process.env.EXPO_PUBLIC_CLERK_PUBLISHABLE_KEY!;
// TODO: remove mock org slug before release
const DEFAULT_ORG_SLUG = 'medlab';

function RootLayoutNav() {
  const colorScheme = useColorScheme();
  const router = useRouter();
  const { getToken, isSignedIn, signOut, orgSlug } = useAuth();
  const setAuthenticated = useAuthStore((state) => state.setAuthenticated);
  const logout = useAuthStore((state) => state.logout);

  useEffect(() => {
    registerTokenGetter(() => getToken());
    registerSignOut(() => signOut());
  }, [getToken, signOut]);

  useEffect(() => {
    registerOrgSlug(DEFAULT_ORG_SLUG);
  }, [orgSlug]);

  useEffect(() => {
    if (isSignedIn === true) {
      void setAuthenticated();
      router.replace("/(drawer)" as never);
    } else if (isSignedIn === false) {
      void logout();
      router.replace("/(auth)/login");
    }
  }, [isSignedIn, router, setAuthenticated, logout]);

  if (isSignedIn === undefined) {
    return <View style={[styles.splash, { backgroundColor: Colors[colorScheme].background }]} />;
  }

  return (
    <NavThemeProvider value={colorScheme === "dark" ? DarkTheme : DefaultTheme}>
      <StatusBar style={colorScheme === "dark" ? "light" : "dark"} />
      <Stack>
        <Stack.Screen name="(drawer)" options={{ headerShown: false }} />
        <Stack.Screen name="(auth)" options={{ headerShown: false }} />
        <Stack.Screen
          name="notifications"
          options={{
            title: t.drawer.items.notificaciones,
            headerBackButtonDisplayMode: 'minimal',
            headerTintColor: Colors[colorScheme].text,
            headerStyle: { backgroundColor: Colors[colorScheme].background },
          }}
        />
        <Stack.Screen name="modal" options={{ presentation: "modal", title: "Modal" }} />
      </Stack>
    </NavThemeProvider>
  );
}

const styles = StyleSheet.create({
  splash: {
    flex: 1,
  },
});

export default function RootLayout() {
  return (
    <ClerkProvider
      publishableKey={CLERK_PUBLISHABLE_KEY}
      tokenCache={tokenCache}
    >
      <RootLayoutNav />
    </ClerkProvider>
  );
}
