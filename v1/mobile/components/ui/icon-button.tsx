import { SymbolViewProps } from 'expo-symbols';
import { Pressable, StyleSheet, PressableProps } from 'react-native';

import { IconSymbol } from '@/components/ui/icon-symbol';
import { Colors, IconSize } from '@/constants/theme';
import { useColorScheme } from '@/hooks/use-color-scheme';

type IconSizeToken = keyof typeof IconSize;

type Props = PressableProps & {
  name: SymbolViewProps['name'];
  size?: IconSizeToken | number;
};

export function DSIconButton({ name, size = 'md', ...props }: Props) {
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme];

  return (
    <Pressable
      style={({ pressed }) => [styles.button, pressed && styles.dimmed]}
      {...props}>
      <IconSymbol name={name} size={size} color={colors.icon} />
    </Pressable>
  );
}

const styles = StyleSheet.create({
  button: {
    paddingHorizontal: 16,
    paddingVertical: 8,
  },
  dimmed: {
    opacity: 0.5,
  },
});
