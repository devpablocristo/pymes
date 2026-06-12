import { Text as RNText, StyleSheet, TextProps } from 'react-native';

import { useColorScheme } from '@/hooks/use-color-scheme';
import { Colors, FontSize, FontWeight } from '@/constants/theme';

type TextVariant = 'title' | 'label' | 'paragraph' | 'caption';
type TextColor = 'default' | 'muted' | 'primary' | 'danger' | 'success' | 'warning';

type Props = TextProps & {
  variant?: TextVariant;
  color?: TextColor;
};

export function DSText({ variant = 'paragraph', color = 'default', style, ...props }: Props) {
  const colorScheme = useColorScheme() ?? 'light';
  const colors = Colors[colorScheme];

  const colorMap: Record<TextColor, string> = {
    default: colors.text,
    muted: colors.muted,
    primary: colors.primary,
    danger: colors.danger,
    success: colors.success,
    warning: colors.warning,
  };
  const colorValue = colorMap[color];

  return (
    <RNText
      style={[styles.base, styles[variant], { color: colorValue }, style]}
      {...props}
    />
  );
}

const styles = StyleSheet.create({
  base: {
    flexShrink: 1,
  },
  title: {
    fontSize: FontSize.xxl,
    fontWeight: FontWeight.bold,
  },
  label: {
    fontSize: FontSize.sm,
    fontWeight: FontWeight.semibold,
  },
  paragraph: {
    fontSize: FontSize.md,
    fontWeight: FontWeight.regular,
  },
  caption: {
    fontSize: FontSize.xs,
    fontWeight: FontWeight.semibold,
    letterSpacing: 0.8,
  },
});
