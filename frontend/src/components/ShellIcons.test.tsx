import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import * as icons from './ShellIcons';

const ICON_EXPORTS = Object.entries(icons).filter(
  ([key]) => key.endsWith('Icon'),
) as [string, React.ReactElement][];

describe('ShellIcons', () => {
  it.each(ICON_EXPORTS)('%s renders an SVG with aria-hidden', (name, icon) => {
    const { container } = render(icon);
    const svg = container.querySelector('svg');
    expect(svg, `${name} should render an <svg>`).not.toBeNull();
    expect(svg!.getAttribute('aria-hidden')).toBe('true');
  });

  it('exports at least 10 icons', () => {
    expect(ICON_EXPORTS.length).toBeGreaterThanOrEqual(10);
  });
});
