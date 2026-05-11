<script lang="ts">
  import SetSection from './SetSection.svelte';
  import SetRow from './SetRow.svelte';
  import Segmented from '../../components/Segmented.svelte';
  import { theme, font, density, measure } from '../../lib/preferences.svelte';

  type Theme = 'system' | 'light' | 'dark' | 'sepia';
  type Font = 'serif' | 'sans';
  type Density = 'compact' | 'comfortable' | 'cosy';
  type Measure = 'narrow' | 'comfortable' | 'wide';

  const themeOptions: { value: Theme; label: string }[] = [
    { value: 'system', label: 'System' },
    { value: 'light',  label: 'Light' },
    { value: 'dark',   label: 'Dark' },
    { value: 'sepia',  label: 'Sepia' },
  ];
  const fontOptions: { value: Font; label: string }[] = [
    { value: 'serif', label: 'Serif' },
    { value: 'sans',  label: 'Sans' },
  ];
  const densityOptions: { value: Density; label: string }[] = [
    { value: 'compact',     label: 'Compact' },
    { value: 'comfortable', label: 'Default' },
    { value: 'cosy',        label: 'Comfortable' },
  ];
  const measureOptions: { value: Measure; label: string }[] = [
    { value: 'narrow',      label: 'Narrow' },
    { value: 'comfortable', label: 'Comfortable' },
    { value: 'wide',        label: 'Wide' },
  ];
</script>

<SetSection num="01" title="Appearance">
  <SetRow
    label="Theme"
    desc="Follow the OS, or pin Tap to a single palette."
  >
    {#snippet control()}
      <Segmented value={theme.stored} options={themeOptions} onChange={(v) => (theme.stored = v)} ariaLabel="Theme" />
    {/snippet}
  </SetRow>
  <SetRow
    label="Reading font"
    desc="Applies to article bodies and entry titles. UI stays sans-serif."
  >
    {#snippet control()}
      <Segmented value={font.value} options={fontOptions} onChange={(v) => (font.value = v)} ariaLabel="Font" />
    {/snippet}
  </SetRow>
  <SetRow
    label="Density"
    desc="Compact hides summaries; comfortable adds breathing room."
  >
    {#snippet control()}
      <Segmented value={density.value} options={densityOptions} onChange={(v) => (density.value = v)} ariaLabel="Density" />
    {/snippet}
  </SetRow>
  <SetRow
    label="Measure"
    desc="Article body width: narrow / comfortable / wide."
  >
    {#snippet control()}
      <Segmented value={measure.value} options={measureOptions} onChange={(v) => (measure.value = v)} ariaLabel="Measure" />
    {/snippet}
  </SetRow>
</SetSection>
