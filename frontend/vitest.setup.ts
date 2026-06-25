// vitest setup: provide localStorage and navigator mocks for jsdom environment
if (typeof globalThis.localStorage === 'undefined' || typeof globalThis.localStorage.getItem !== 'function') {
  const store: Record<string, string> = {}
  globalThis.localStorage = {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => { store[key] = String(value) },
    removeItem: (key: string) => { delete store[key] },
    clear: () => { for (const k in store) delete store[k] },
    get length() { return Object.keys(store).length },
    key: (index: number) => Object.keys(store)[index] ?? null,
  } as Storage
}

if (typeof globalThis.navigator === 'undefined') {
  (globalThis as unknown as { navigator: Navigator }).navigator = { language: 'en', languages: ['en'] } as Navigator
}
