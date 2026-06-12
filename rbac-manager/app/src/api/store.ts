import { configureStore } from '@reduxjs/toolkit';

import { confighubApi } from '../sdk/confighubapi';
// Importing the generated module registers all endpoints on confighubApi.
import '../sdk/confighubapi.gen';

export const store = configureStore({
  reducer: {
    [confighubApi.reducerPath]: confighubApi.reducer,
  },
  middleware: (getDefaultMiddleware) =>
    getDefaultMiddleware().concat(confighubApi.middleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
