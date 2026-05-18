import {
  DrawerContentComponentProps,
  DrawerContentScrollView,
} from "@react-navigation/drawer";
import Constants from "expo-constants";
import { SymbolViewProps } from "expo-symbols";
import { Pressable, StyleSheet, View } from "react-native";

import { DSButton } from "@/components/ui/button";
import { IconSymbol } from "@/components/ui/icon-symbol";
import { DSText } from "@/components/ui/text";
import { Colors, FontSize, FontWeight, Spacing } from "@/constants/theme";
import { t } from "@/constants/translations";
import { useColorScheme } from "@/hooks/use-color-scheme";
import { useAuthStore } from "@/stores/auth-store";

type DrawerItem = {
  label: string;
  icon: SymbolViewProps["name"];
  active?: boolean;
  disabled?: boolean;
};

type DrawerSection = {
  title: string;
  items: DrawerItem[];
};

const SECTIONS: DrawerSection[] = [
  {
    title: t.drawer.sections.inicio,
    items: [{ label: t.drawer.home, icon: "square.grid.2x2", active: true }],
  },
  {
    title: t.drawer.sections.diaDia,
    items: [
      { label: t.drawer.items.agenda, icon: "calendar", disabled: true },
      { label: t.drawer.items.asistente, icon: "cpu", disabled: true },
      { label: t.drawer.items.notificaciones, icon: "bell", disabled: true },
    ],
  },
  {
    title: t.drawer.sections.comercial,
    items: [
      {
        label: t.drawer.items.facturacion,
        icon: "dollarsign.square",
        disabled: true,
      },
      { label: t.drawer.items.clientes, icon: "person.2", disabled: true },
      { label: t.drawer.items.presupuestos, icon: "doc.text", disabled: true },
      { label: t.drawer.items.productos, icon: "cube", disabled: true },
      { label: t.drawer.items.proveedores, icon: "building.2", disabled: true },
      { label: t.drawer.items.servicios, icon: "scissors", disabled: true },
      { label: t.drawer.items.ventas, icon: "cart", disabled: true },
    ],
  },
  {
    title: t.drawer.sections.whatsapp,
    items: [
      {
        label: t.drawer.items.bandejaWhatsapp,
        icon: "bubble.left.and.bubble.right",
        disabled: true,
      },
      {
        label: t.drawer.items.campanasWhatsapp,
        icon: "megaphone",
        disabled: true,
      },
    ],
  },
  {
    title: t.drawer.sections.operaciones,
    items: [
      { label: t.drawer.items.caja, icon: "chart.bar", disabled: true },
      {
        label: t.drawer.items.compras,
        icon: "arrow.left.arrow.right",
        disabled: true,
      },
      {
        label: t.drawer.items.inventario,
        icon: "cube.transparent",
        disabled: true,
      },
      {
        label: t.drawer.items.reportes,
        icon: "chart.line.uptrend.xyaxis",
        disabled: true,
      },
    ],
  },
];

export function CustomDrawerContent(props: DrawerContentComponentProps) {
  const logout = useAuthStore((state) => state.logout);
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme];

  return (
    <DrawerContentScrollView
      {...props}
      contentContainerStyle={styles.container}
      showsVerticalScrollIndicator={false}
      style={{ backgroundColor: colors.surface }}
    >
      <View style={styles.sections}>
        {SECTIONS.map((section) => (
          <View key={section.title} style={styles.section}>
            <DSText variant="caption" color="muted" style={styles.sectionTitle}>
              {section.title}
            </DSText>
            {section.items.map((item) => (
              <Pressable
                key={item.label}
                disabled={item.disabled}
                style={({ pressed }) => [
                  styles.item,
                  item.active && { backgroundColor: colors.primary + "22" },
                  pressed &&
                    !item.disabled && {
                      backgroundColor: colors.primary + "15",
                    },
                  item.disabled && styles.disabled,
                ]}
              >
                <IconSymbol
                  name={item.icon}
                  size="md"
                  color={item.active ? colors.primary : colors.icon}
                />
                <DSText
                  style={styles.itemLabel}
                  color={
                    item.active
                      ? "primary"
                      : item.disabled
                        ? "muted"
                        : "default"
                  }
                >
                  {item.label}
                </DSText>
              </Pressable>
            ))}
          </View>
        ))}
      </View>

      <View style={[styles.footer, { borderTopColor: colors.muted }]}>
        <DSButton
          title={t.drawer.logout}
          variant="link"
          onPress={() => void logout()}
        />
        <DSText variant="paragraph" color="muted" style={styles.version}>
          v{Constants.expoConfig?.version}
        </DSText>
      </View>
    </DrawerContentScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flexGrow: 1,
  },
  sections: {
    flex: 1,
    paddingTop: Spacing.md,
  },
  section: {
    marginBottom: Spacing.lg,
  },
  sectionTitle: {
    paddingHorizontal: Spacing.lg,
    marginBottom: Spacing.sm,
  },
  item: {
    flexDirection: "row",
    alignItems: "center",
    gap: Spacing.md,
    paddingHorizontal: Spacing.lg,
    paddingVertical: 10,
    marginHorizontal: Spacing.md,
    borderRadius: 8,
  },
  itemLabel: {
    fontSize: FontSize.md,
    fontWeight: FontWeight.medium,
  },
  disabled: {
    opacity: 0.4,
  },
  footer: {
    paddingHorizontal: Spacing.lg,
    paddingVertical: Spacing.xl,
    borderTopWidth: StyleSheet.hairlineWidth,
    alignItems: "center",
  },
  version: {
    marginTop: Spacing.sm,
  },
});
