import { ActivityIndicator, Pressable, StyleSheet, Text, PressableProps } from 'react-native';

import { useColorScheme } from '@/hooks/use-color-scheme';
import { Colors, FontSize, FontWeight } from '@/constants/theme';

type Props = PressableProps & {
  title: string;
  loading?: boolean;
  variant?: 'solid' | 'link';
};

export function DSButton({ title, loading = false, disabled, style, variant = 'solid', ...props }: Props) {
  const colorScheme = useColorScheme() ?? 'light';
  const colors = Colors[colorScheme];

  const isDisabled = disabled || loading;
  const isLink = variant === 'link';

  const bgColor = isDisabled && !isLink ? colors.muted : colors.primary;

  return (
    <Pressable
      style={({ pressed }) => [
        styles.button,
        isLink ? styles.buttonLink : { backgroundColor: bgColor },
        pressed && !isDisabled && styles.pressed,
        style as object,
      ]}
      disabled={isDisabled}
      {...props}>
      {loading ? (
        <ActivityIndicator color={isLink ? colors.muted : colors.buttonText} />
      ) : (
        <Text style={[
          styles.label,
          { color: isLink ? colors.alert : colors.buttonText },
          isDisabled && isLink && { color: colors.muted },
        ]}>
          {title}
        </Text>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    height: 52,
    borderRadius: 10,
    borderCurve: 'continuous',
    alignItems: 'center',
    justifyContent: 'center',
  },
  buttonLink: {
    height: 52,
    alignItems: 'center',
    justifyContent: 'center',
  },
  label: {
    fontSize: FontSize.md,
    fontWeight: FontWeight.semibold,
  },
  pressed: {
    opacity: 0.8,
  },
});
