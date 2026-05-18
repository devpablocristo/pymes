/**
 * Below are the colors that are used in the app. The colors are defined in the light and dark mode.
 * There are many other ways to style your app. For example, [Nativewind](https://www.nativewind.dev/), [Tamagui](https://tamagui.dev/), [unistyles](https://reactnativeunistyles.vercel.app), etc.
 */

import { Platform } from "react-native";

export const Colors = {
  light: {
    text: "#1A1A2E",
    background: "#F3F4F6",
    surface: "#FFFFFF",
    secondary: "#F59E0B",
    icon: "#6B7280",
    tabIconDefault: "#6B7280",
    tabIconSelected: "#3B82F6",
    inputBackground: "#E5E7EB",
    buttonText: "#FFFFFF",
    muted: "#9CA3AF",
    success: "#10B981",
    alert: "#F43F5E",
    primary: "#3B82F6",
    warning: "#F6B047",
    danger: "#FF7171",
    purple: "#A855F7",
  },
  dark: {
    text: "#E5E7EB",
    background: "#121212",
    surface: "#1F1F1F",
    secondary: "#F59E0B",
    icon: "#9CA3AF",
    tabIconDefault: "#9CA3AF",
    tabIconSelected: "#3B82F6",
    inputBackground: "#1F1F1F",
    buttonText: "#FFFFFF",
    muted: "#6B7280",
    success: "#86EF9B",
    alert: "#F43F5E",
    primary: "#3B82F6",
    warning: "#F6B047",
    danger: "#FF7171",
    purple: "#A855F7",
  },
};

export const Spacing = {
  xs: 2,
  sm: 4,
  md: 8,
  lg: 16,
  xl: 32,
} as const;

export const FontSize = {
  xs: 12,
  sm: 14,
  md: 16,
  lg: 20,
  xl: 24,
  xxl: 32,
} as const;

export const IconSize = {
  sm: 16,
  md: 22,
  lg: 28,
  xl: 36,
} as const;

export const FontWeight = {
  regular: "400" as const,
  medium: "500" as const,
  semibold: "600" as const,
  bold: "700" as const,
};

export const Fonts = Platform.select({
  ios: {
    /** iOS `UIFontDescriptorSystemDesignDefault` */
    sans: "system-ui",
    /** iOS `UIFontDescriptorSystemDesignSerif` */
    serif: "ui-serif",
    /** iOS `UIFontDescriptorSystemDesignRounded` */
    rounded: "ui-rounded",
    /** iOS `UIFontDescriptorSystemDesignMonospaced` */
    mono: "ui-monospace",
  },
  default: {
    sans: "normal",
    serif: "serif",
    rounded: "normal",
    mono: "monospace",
  },
  web: {
    sans: "system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif",
    serif: "Georgia, 'Times New Roman', serif",
    rounded:
      "'SF Pro Rounded', 'Hiragino Maru Gothic ProN', Meiryo, 'MS PGothic', sans-serif",
    mono: "SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
  },
});
