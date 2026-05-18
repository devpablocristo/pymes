import { SymbolView, SymbolViewProps, SymbolWeight } from 'expo-symbols';
import { StyleProp, ViewStyle } from 'react-native';

import { IconSize } from '@/constants/theme';

type IconSizeToken = keyof typeof IconSize;

export function IconSymbol({
  name,
  size = 'md',
  color,
  style,
  weight = 'regular',
}: {
  name: SymbolViewProps['name'];
  size?: IconSizeToken | number;
  color: string;
  style?: StyleProp<ViewStyle>;
  weight?: SymbolWeight;
}) {
  const resolvedSize = typeof size === 'string' ? IconSize[size] : size;

  return (
    <SymbolView
      weight={weight}
      tintColor={color}
      resizeMode="scaleAspectFit"
      name={name}
      style={[{ width: resolvedSize, height: resolvedSize }, style]}
    />
  );
}
