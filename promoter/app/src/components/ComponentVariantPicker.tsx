import { Alert, FormControl, InputLabel, MenuItem, Select, Stack } from '@mui/material';

import { Catalog } from '../data/catalog';
import { ComponentChoice } from '../model/workflow';

/**
 * Two linked selects: a Component label value, then one of that component's
 * available Variant Spaces. Driven entirely by the live catalog.
 */
export function ComponentVariantPicker({
  catalog,
  value,
  onChange,
}: {
  catalog: Catalog;
  value: ComponentChoice;
  onChange: (next: ComponentChoice) => void;
}) {
  const componentInfo = catalog.components.find((c) => c.component === value.component);
  const variants = componentInfo?.variants ?? [];
  const variantMissing = value.variant !== '' && !variants.some((v) => v.variant === value.variant);

  return (
    <Stack spacing={1} sx={{ minWidth: 320 }}>
      <Stack direction='row' spacing={1}>
        <FormControl size='small' sx={{ minWidth: 150 }}>
          <InputLabel>Component</InputLabel>
          <Select
            label='Component'
            value={value.component}
            onChange={(e) => onChange({ component: e.target.value, variant: '' })}
          >
            {catalog.components.map((c) => (
              <MenuItem key={c.component} value={c.component}>
                {c.component}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControl size='small' sx={{ minWidth: 150 }} disabled={value.component === ''}>
          <InputLabel>Variant</InputLabel>
          <Select
            label='Variant'
            value={variantMissing ? '' : value.variant}
            onChange={(e) => onChange({ ...value, variant: e.target.value })}
          >
            {variants.map((v) => (
              <MenuItem key={v.spaceId} value={v.variant}>
                {v.variant}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      </Stack>
      {variantMissing && (
        <Alert severity='warning' sx={{ py: 0 }}>
          Variant “{value.variant}” no longer exists for {value.component}.
        </Alert>
      )}
    </Stack>
  );
}
