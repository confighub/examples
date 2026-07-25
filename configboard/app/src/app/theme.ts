import { ThemeProvider as MuiThemeProvider, createTheme } from '@mui/material/styles';
import useMediaQuery from '@mui/material/useMediaQuery';
import { type ReactNode, createElement, useMemo } from 'react';

import { SURFACE } from '../charts/palette';

/**
 * Dark mode is a selected set of steps validated against the dark surface, not an
 * inversion of the light one — the chart palette carries its own dark column, and the
 * app surface matches it so contrast checks hold.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const prefersDark = useMediaQuery('(prefers-color-scheme: dark)');

  const theme = useMemo(() => {
    const mode = prefersDark ? 'dark' : 'light';
    return createTheme({
      palette: {
        mode,
        background: {
          default: SURFACE[mode],
          paper: mode === 'dark' ? '#212120' : '#ffffff',
        },
      },
      shape: { borderRadius: 6 },
      typography: {
        fontFamily:
          '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
      },
    });
  }, [prefersDark]);

  return createElement(MuiThemeProvider, { theme }, children);
}
