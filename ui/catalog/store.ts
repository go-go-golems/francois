import { configureStore, createSlice, PayloadAction } from "@reduxjs/toolkit";
import { api } from "./api";

// Minimal client-side router state; links push history entries.
const route = createSlice({
  name: "route",
  initialState: { path: location.pathname, search: location.search },
  reducers: {
    navigated: (s, a: PayloadAction<{ path: string; search: string }>) => {
      s.path = a.payload.path;
      s.search = a.payload.search;
    },
  },
});

const notice = createSlice({
  name: "notice",
  initialState: { text: "", bad: false },
  reducers: {
    show: (s, a: PayloadAction<{ text: string; bad?: boolean }>) => {
      s.text = a.payload.text;
      s.bad = a.payload.bad ?? false;
    },
    clear: (s) => {
      s.text = "";
    },
  },
});

export const store = configureStore({
  reducer: {
    [api.reducerPath]: api.reducer,
    route: route.reducer,
    notice: notice.reducer,
  },
  middleware: (g) => g().concat(api.middleware),
});
export type State = ReturnType<typeof store.getState>;
export const { show, clear } = notice.actions;

// Keep a position on each SPA entry so a refused traversal can return to
// the current entry without unmounting the form or adding duplicate history.
const positionKey = "francoisRoutePosition";
let position: number = history.state?.[positionKey] ?? 0;
history.replaceState({ ...history.state, [positionKey]: position }, "");
let restoringHistory = false;
let navigationGuard: (() => boolean) | undefined;

export function registerNavigationGuard(guard: () => boolean) {
  navigationGuard = guard;
  return () => {
    if (navigationGuard === guard) navigationGuard = undefined;
  };
}

function publishLocation() {
  store.dispatch(
    route.actions.navigated({
      path: location.pathname,
      search: location.search,
    }),
  );
}

export function navigate(to: string, replace = false) {
  const u = new URL(to, location.origin);
  if (restoringHistory) return;
  if (u.pathname === location.pathname && u.search === location.search) return;
  if (navigationGuard && !navigationGuard()) return;
  if (replace)
    history.replaceState(
      { ...history.state, [positionKey]: position },
      "",
      u.pathname + u.search,
    );
  else
    history.pushState({ [positionKey]: ++position }, "", u.pathname + u.search);
  publishLocation();
  window.scrollTo(0, 0);
}
addEventListener("popstate", (event) => {
  const targetPosition: number = event.state?.[positionKey] ?? position;
  if (restoringHistory) {
    if (targetPosition === position) restoringHistory = false;
    else history.go(position - targetPosition);
    return;
  }
  const current = store.getState().route;
  const routeChanged =
    current.path !== location.pathname || current.search !== location.search;
  if (routeChanged && navigationGuard && !navigationGuard()) {
    restoringHistory = true;
    history.go(position - targetPosition);
    return;
  }
  position = targetPosition;
  publishLocation();
});
