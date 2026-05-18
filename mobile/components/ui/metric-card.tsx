import { SymbolViewProps } from 'expo-symbols';
import { StyleSheet, View } from 'react-native';

import { IconSymbol } from '@/components/ui/icon-symbol';
import { DSText } from '@/components/ui/text';
import { Colors, FontSize, FontWeight, Spacing } from '@/constants/theme';
import { t } from '@/constants/translations';
import { useColorScheme } from '@/hooks/use-color-scheme';

export type MetricCardProps = {
  title: string;
  value: string;
  subtitle: string;
  change: string;
  positive: boolean;
  iconName: SymbolViewProps['name'];
  accentColor: string;
};

export function MetricCard({ title, value, subtitle, change, positive, iconName, accentColor }: MetricCardProps) {
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme];
  const changeToken = positive ? 'success' : 'danger' as const;
  const changeColor = colors[changeToken];

  return (
    <View style={[styles.card, { backgroundColor: colors.surface }]}>
      <View style={styles.cardTop}>
        <DSText variant="label" color="muted">{title}</DSText>
        <View style={[styles.iconCircle, { backgroundColor: accentColor + '22' }]}>
          <IconSymbol name={iconName} size="sm" color={accentColor} />
        </View>
      </View>

      <DSText variant="title">{value}</DSText>
      <DSText variant="paragraph" color="muted">{subtitle}</DSText>

      <View style={[styles.badge, { backgroundColor: changeColor + '1A' }]}>
        <IconSymbol
          name={positive ? 'arrow.up' : 'arrow.down'}
          size="sm"
          color={changeColor}
        />
        <DSText style={styles.badgeText} color={changeToken}>
          {change} {t.home.metrics.vsPrev}
        </DSText>
      </View>

      <View style={[styles.accent, { backgroundColor: accentColor }]} />
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderRadius: 14,
    padding: Spacing.lg,
    gap: Spacing.sm,
    overflow: 'hidden',
  },
  cardTop: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: Spacing.sm,
  },
  iconCircle: {
    width: 36,
    height: 36,
    borderRadius: 18,
    alignItems: 'center',
    justifyContent: 'center',
  },
  badge: {
    flexDirection: 'row',
    alignItems: 'center',
    alignSelf: 'flex-start',
    borderRadius: 6,
    paddingHorizontal: Spacing.md,
    paddingVertical: Spacing.xs,
    gap: 4,
    marginTop: Spacing.sm,
  },
  badgeText: {
    fontSize: FontSize.xs,
    fontWeight: FontWeight.medium,
  },
  accent: {
    position: 'absolute',
    bottom: 0,
    left: 0,
    right: 0,
    height: 2,
    opacity: 0.5,
  },
});
