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
  reducer: { [api.reducerPath]: api.reducer, route: route.reducer, notice: notice.reducer },
  middleware: (g) => g().concat(api.middleware),
});
export type State = ReturnType<typeof store.getState>;
export const { show, clear } = notice.actions;

export function navigate(to: string, replace = false) {
  const u = new URL(to, location.origin);
  if (replace) history.replaceState(null, "", u.pathname + u.search);
  else history.pushState(null, "", u.pathname + u.search);
  store.dispatch(route.actions.navigated({ path: u.pathname, search: u.search }));
  window.scrollTo(0, 0);
}
addEventListener("popstate", () =>
  store.dispatch(route.actions.navigated({ path: location.pathname, search: location.search })),
);
