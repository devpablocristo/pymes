import { StyleSheet } from 'react-native';

import { DSLayout } from '@/components/ui/layout';
import { DSText } from '@/components/ui/text';
import { t } from '@/constants/translations';

export default function NotificationsScreen() {
  return (
    <DSLayout style={styles.content}>
      <DSText variant="paragraph" color="muted">{t.notifications.empty}</DSText>
    </DSLayout>
  );
}

const styles = StyleSheet.create({
  content: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
});
