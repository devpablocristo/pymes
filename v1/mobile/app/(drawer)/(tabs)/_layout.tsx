import { DrawerActions } from '@react-navigation/native';
import { Tabs, useNavigation, useRouter } from 'expo-router';
import React from 'react';

import { HapticTab } from '@/components/haptic-tab';
import { DSIconButton } from '@/components/ui/icon-button';
import { IconSymbol } from '@/components/ui/icon-symbol';
import { Colors, IconSize } from '@/constants/theme';
import { t } from '@/constants/translations';
import { useColorScheme } from '@/hooks/use-color-scheme';

export default function TabLayout() {
  const colorScheme = useColorScheme();
  const colors = Colors[colorScheme ?? 'light'];
  const navigation = useNavigation();
  const router = useRouter();

  return (
    <Tabs
      screenOptions={{
        tabBarActiveTintColor: colors.primary,
        tabBarButton: HapticTab,
        // TODO: evaluate whether to keep bottom tabs or navigate only from the drawer
        tabBarStyle: { display: 'none' },
        headerStyle: { backgroundColor: colors.background },
        headerTintColor: colors.text,
        headerShadowVisible: false,
      }}>
      <Tabs.Screen
        name="index"
        options={{
          title: t.drawer.home,
          headerLeft: () => (
            <DSIconButton
              name="line.3.horizontal"
              size="md"
              onPress={() => navigation.dispatch(DrawerActions.openDrawer())}
            />
          ),
          headerRight: () => (
            <DSIconButton
              name="bell"
              onPress={() => router.push('/notifications')}
            />
          ),
          tabBarIcon: ({ color }) => <IconSymbol size={IconSize.lg} name="house.fill" color={color} />,
        }}
      />
<Tabs.Screen
        name="explore"
        options={{
          title: t.drawer.explore,
          headerShown: false,
          tabBarIcon: ({ color }) => <IconSymbol size={IconSize.lg} name="paperplane.fill" color={color} />,
        }}
      />
    </Tabs>
  );
}
