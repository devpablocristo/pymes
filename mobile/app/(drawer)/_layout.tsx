import { Drawer } from 'expo-router/drawer';

import { CustomDrawerContent } from '@/components/ui/drawer-content';
import { Colors } from '@/constants/theme';
import { t } from '@/constants/translations';
import { useColorScheme } from '@/hooks/use-color-scheme';

export default function DrawerLayout() {
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme];

  return (
    <Drawer
      drawerContent={(props) => <CustomDrawerContent {...props} />}
      screenOptions={{
        headerShown: false,
        drawerActiveTintColor: colors.primary,
        drawerInactiveTintColor: colors.icon,
        drawerStyle: { backgroundColor: colors.surface },
      }}>
      <Drawer.Screen
        name="(tabs)"
        options={{ title: t.drawer.home, drawerLabel: t.drawer.home }}
      />
    </Drawer>
  );
}
