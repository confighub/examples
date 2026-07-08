import { configureStore } from '@reduxjs/toolkit';
import { confighubApi } from '@confighub/rtk-query';

// Standard RTK Query store wiring: mount the ConfigHub api's reducer and
// middleware. Endpoints ship injected in the package; the base URL and token
// source are set once via configureConfigHub() in main.tsx.
export const store = configureStore({
  reducer: {
    [confighubApi.reducerPath]: confighubApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(confighubApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
