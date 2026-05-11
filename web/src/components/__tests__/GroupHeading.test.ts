import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/svelte';
import GroupHeading from '../GroupHeading.svelte';

describe('GroupHeading', () => {
  it('renders the label and count', () => {
    const { getByText } = render(GroupHeading, { props: { label: 'Today', count: 7 } });
    expect(getByText('Today')).toBeTruthy();
    expect(getByText('7')).toBeTruthy();
  });

  it('renders count of 0 explicitly (not blank)', () => {
    const { getByText } = render(GroupHeading, { props: { label: 'Yesterday', count: 0 } });
    expect(getByText('0')).toBeTruthy();
  });

  it('renders a rule element', () => {
    const { container } = render(GroupHeading, { props: { label: 'Earlier', count: 3 } });
    expect(container.querySelector('.rule, .ts-group-rule')).toBeTruthy();
  });
});
