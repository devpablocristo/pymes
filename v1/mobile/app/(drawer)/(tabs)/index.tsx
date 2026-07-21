import { StyleSheet, View } from "react-native";

import { DSLayout } from "@/components/ui/layout";
import { MetricCard } from "@/components/ui/metric-card";
import { Colors, Spacing } from "@/constants/theme";
import { t } from "@/constants/translations";
import { useColorScheme } from "@/hooks/use-color-scheme";

export default function HomeScreen() {
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme];

  // TODO: fetch metrics from API and replace mock data
  return (
    <DSLayout scrollable>
      <View style={styles.grid}>
        <MetricCard
          title={t.home.metrics.sales}
          value="$69K"
          subtitle={`3 ${t.home.metrics.salesSub}`}
          change="+23%"
          positive
          iconName="dollarsign"
          accentColor={colors.primary}
        />
        <MetricCard
          title={t.home.metrics.avgTicket}
          value="$23K"
          subtitle={t.home.metrics.avgTicketSub}
          change="+8%"
          positive
          iconName="doc.text"
          accentColor={colors.success}
        />
        <MetricCard
          title={t.home.metrics.income}
          value="$69K"
          subtitle={t.home.metrics.incomeSub}
          change="+15%"
          positive
          iconName="arrow.up.right"
          accentColor={colors.warning}
        />
        <MetricCard
          title={t.home.metrics.expenses}
          value="$0"
          subtitle={t.home.metrics.expensesSub}
          change="+12%"
          positive={false}
          iconName="arrow.down.right"
          accentColor={colors.danger}
        />
      </View>
    </DSLayout>
  );
}

const styles = StyleSheet.create({
  grid: {
    gap: Spacing.lg,
  },
});
