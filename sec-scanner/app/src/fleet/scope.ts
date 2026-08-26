// This app's analysis scope. The store is created once here so every page and the
// settings dialog read and write the same key.

import { createScopeStore } from '@confighub/examples-webkit/fleet';

export const scopeStore = createScopeStore('sec-scanner-scope');
