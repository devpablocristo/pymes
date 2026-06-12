import { View, ScrollView, StyleSheet, ViewProps, ScrollViewProps } from 'react-native';

import { useColorScheme } from '@/hooks/use-color-scheme';
import { Colors, Spacing } from '@/constants/theme';

type Props = ViewProps & {
  disablePadding?: boolean;
  scrollable?: boolean;
  contentStyle?: ScrollViewProps['contentContainerStyle'];
  scrollProps?: Omit<ScrollViewProps, 'style' | 'contentContainerStyle'>;
};

export function DSLayout({ disablePadding = false, scrollable = false, contentStyle, scrollProps, style, children, ...props }: Props) {
  const colorScheme = useColorScheme() ?? 'light';
  const colors = Colors[colorScheme];

  const paddingStyle = disablePadding ? styles.noPadding : null;

  if (scrollable) {
    return (
      <ScrollView
        contentInsetAdjustmentBehavior="automatic"
        keyboardShouldPersistTaps="handled"
        showsVerticalScrollIndicator={false}
        {...scrollProps}
        style={[styles.layout, { backgroundColor: colors.background }, style]}
        contentContainerStyle={[styles.scrollContent, { backgroundColor: colors.background }, paddingStyle, contentStyle]}>
        {children}
      </ScrollView>
    );
  }

  return (
    <View
      style={[styles.layout, { backgroundColor: colors.background }, paddingStyle, style]}
      {...props}>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  layout: {
    flex: 1,
  },
  scrollContent: {
    flexGrow: 1,
    paddingHorizontal: Spacing.lg,
  },
  noPadding: {
    paddingHorizontal: 0,
  },
});
