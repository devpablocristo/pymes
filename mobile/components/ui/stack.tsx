import { View, StyleSheet, ViewProps } from 'react-native';

import { Spacing } from '@/constants/theme';

type Align = 'left' | 'center' | 'right';
type SpacingKey = keyof typeof Spacing;

type StackProps = ViewProps & {
  align?: Align;
  gap?: SpacingKey;
};

const alignItems: Record<Align, 'flex-start' | 'center' | 'flex-end'> = {
  left: 'flex-start',
  center: 'center',
  right: 'flex-end',
};

export function DSVStack({ align, gap, style, children, ...props }: StackProps) {
  return (
    <View
      style={[
        styles.vstack,
        align && { alignItems: alignItems[align] },
        gap && { gap: Spacing[gap] },
        style,
      ]}
      {...props}>
      {children}
    </View>
  );
}

export function DSHStack({ align, gap, style, children, ...props }: StackProps) {
  return (
    <View
      style={[
        styles.hstack,
        align && { justifyContent: alignItems[align] },
        gap && { gap: Spacing[gap] },
        style,
      ]}
      {...props}>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  vstack: {
    flexDirection: 'column',
  },
  hstack: {
    flexDirection: 'row',
    alignItems: 'center',
  },
});
