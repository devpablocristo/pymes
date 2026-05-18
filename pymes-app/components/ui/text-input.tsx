import { TextInput as RNTextInput, StyleSheet, TextInputProps } from 'react-native';

import { useColorScheme } from '@/hooks/use-color-scheme';
import { Colors, FontSize } from '@/constants/theme';

export function DSTextInput({ style, ...props }: TextInputProps) {
  const colorScheme = useColorScheme() ?? 'light';
  const colors = Colors[colorScheme];

  return (
    <RNTextInput
      style={[
        styles.input,
        {
          color: colors.text,
          borderColor: colors.icon,
          backgroundColor: colors.inputBackground,
        },
        style,
      ]}
      placeholderTextColor={colors.icon}
      {...props}
    />
  );
}

const styles = StyleSheet.create({
  input: {
    height: 50,
    borderWidth: 1,
    borderRadius: 10,
    borderCurve: 'continuous',
    paddingHorizontal: 14,
    fontSize: FontSize.md,
  },
});
