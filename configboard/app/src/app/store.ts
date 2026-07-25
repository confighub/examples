import { confighubApi } from '@confighub/rtk-query';
import { configureStore } from '@reduxjs/toolkit';

// Standard RTK Query wiring. The base URL and token source are set once via
// configureConfigHub() in main.tsx; endpoints ship injected in the package.
export const store = configureStore({
  reducer: {
    [confighubApi.reducerPath]: confighubApi.reducer,
  },
  middleware: (getDefaultMiddleware) => getDefaultMiddleware().concat(confighubApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
