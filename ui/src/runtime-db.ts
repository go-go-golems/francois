type Transport = {
  rpc: (method: string, args?: unknown) => Promise<any>;
  subscribe: (callback: (e: any) => void) => () => void;
};
type Data = Record<string, any>;
const freeze = <T>(value: T): T => {
  if (value && typeof value === "object") {
    Object.values(value).forEach(freeze);
    Object.freeze(value);
  }
  return value;
};
export function validatePath(path: string, doc: boolean) {
  const parts = path.split("/");
  if (
    new TextEncoder().encode(path).length > 1000 ||
    parts.length > 16 ||
    (parts.length % 2 === 0) !== doc ||
    parts.some(
      (p) =>
        !/^[A-Za-z0-9_.~:@+-]+$/.test(p) ||
        new TextEncoder().encode(p).length > 200,
    )
  )
    throw new TypeError("Invalid document or collection path");
}

function merge(base: Data, patch: Data): Data {
  const out = structuredClone(base);
  for (const [key, value] of Object.entries(patch)) {
    out[key] =
      value &&
      typeof value === "object" &&
      !Array.isArray(value) &&
      out[key] &&
      typeof out[key] === "object" &&
      !Array.isArray(out[key])
        ? merge(out[key], value)
        : value;
  }
  return out;
}
function canonical(value: any): string {
  return JSON.stringify(value, (_key, v) =>
    v && typeof v === "object" && !Array.isArray(v)
      ? Object.fromEntries(
          Object.keys(v)
            .sort()
            .map((k) => [k, v[k]]),
        )
      : v,
  );
}
function compare(a: any, b: any): number {
  if (typeof a === typeof b && (typeof a === "number" || typeof a === "string"))
    return a < b ? -1 : a > b ? 1 : 0;
  const x = canonical(a),
    y = canonical(b);
  return x < y ? -1 : x > y ? 1 : 0;
}
function matches(value: any, filter: any): boolean {
  if (value === undefined) return false;
  const equal = (a: any, b: any) => canonical(a) === canonical(b);
  switch (filter.op) {
    case "==":
      return equal(value, filter.value);
    case "!=":
      return !equal(value, filter.value);
    case "<":
      return compare(value, filter.value) < 0;
    case "<=":
      return compare(value, filter.value) <= 0;
    case ">":
      return compare(value, filter.value) > 0;
    case ">=":
      return compare(value, filter.value) >= 0;
    case "in":
      return filter.value.some((v: any) => equal(value, v));
    case "not-in":
      return !filter.value.some((v: any) => equal(value, v));
    case "array-contains":
      return Array.isArray(value) && value.some((v) => equal(v, filter.value));
    default:
      return false;
  }
}

export function installDB(transport: Transport) {
  const identity = new Map<string, any>();
  const subscriptions = new Set<() => void>();
  const queues = new Map<string, Promise<unknown>>();
  const pending = new Map<string, Data | null>();
  const committed = new Map<string, any>();
  transport.subscribe((e) => {
    if (e.type === "db_changed" || (e.type === "connection" && e.up))
      for (const refresh of subscriptions) refresh();
  });
  const docSnapshot = (d: any, hasPending = false) => {
    const key = d.path + "|" + JSON.stringify([d.body, d.exists, hasPending]);
    let snap = identity.get(key);
    if (snap) return snap;
    const data = d.exists ? freeze(structuredClone(d.body)) : undefined;
    snap = Object.freeze({
      id: d.id,
      exists: d.exists,
      data: () => data,
      metadata: Object.freeze({
        fromCache: hasPending,
        hasPendingWrites: hasPending,
      }),
    });
    identity.set(key, snap);
    if (identity.size > 2000) identity.delete(identity.keys().next().value!);
    return snap;
  };
  function subscribe(
    load: () => Promise<any>,
    next: (s: any) => void,
    onError?: (e: any) => void,
  ) {
    if (subscriptions.size >= 64) {
      queueMicrotask(() =>
        onError?.({
          code: "resource_exhausted",
          message: "64 subscriptions allowed",
        }),
      );
      return () => {};
    }
    let alive = true,
      running = false,
      dirty = false;
    const refresh = () => {
      dirty = true;
      if (running || !alive) return;
      running = true;
      void (async () => {
        while (dirty && alive) {
          dirty = false;
          try {
            const snap = await load();
            if (alive) {
              try {
                next(snap);
              } catch (error) {
                console.error("Snapshot callback failed", error);
              }
            }
          } catch (e) {
            if (alive) {
              try {
                onError?.(e);
              } catch (error) {
                console.error("Snapshot error callback failed", error);
              }
            }
          }
        }
        running = false;
      })();
    };
    subscriptions.add(refresh);
    queueMicrotask(refresh);
    return () => {
      alive = false;
      subscriptions.delete(refresh);
    };
  }
  function document(path: string) {
    validatePath(path, true);
    const get = async () => {
      if (pending.has(path)) {
        const body = pending.get(path);
        return docSnapshot(
          { path, id: path.split("/").at(-1), body, exists: body !== null },
          true,
        );
      }
      const d = await transport.rpc("db.get", { path });
      committed.set(path, d);
      if (pending.has(path)) {
        const body = pending.get(path);
        return docSnapshot(
          { ...d, id: path.split("/").at(-1), body, exists: body !== null },
          true,
        );
      }
      return docSnapshot(d);
    };
    function write(operation: string, body?: Data) {
      if (
        operation !== "delete" &&
        (!body || Array.isArray(body) || typeof body !== "object")
      )
        return Promise.reject({
          code: "invalid_argument",
          message: "Body must be a JSON object",
        });
      let copy: Data | undefined;
      try {
        copy = body ? JSON.parse(JSON.stringify(body)) : undefined;
      } catch {
        return Promise.reject({
          code: "invalid_argument",
          message: "Body must be JSON",
        });
      }
      const previous = queues.get(path) ?? Promise.resolve();
      const result = previous
        .catch(() => {})
        .then(async () => {
          if (operation === "set") pending.set(path, copy!);
          else if (operation === "delete") pending.set(path, null);
          else if (committed.get(path)?.exists)
            pending.set(path, merge(committed.get(path).body, copy!));
          for (const refresh of subscriptions) refresh();
          try {
            await transport.rpc("db." + operation, { path, body: copy });
          } finally {
            pending.delete(path);
            for (const refresh of subscriptions) refresh();
          }
        });
      queues.set(path, result);
      void result
        .finally(() => {
          if (queues.get(path) === result) queues.delete(path);
        })
        .catch(() => {});
      return result;
    }
    return Object.freeze({
      id: path.split("/").at(-1),
      path,
      get,
      set: (body: Data) => write("set", body),
      update: (body: Data) => write("update", body),
      delete: () => write("delete"),
      acquire: (options: any) =>
        transport.rpc("db.acquire", { path, ...options }),
      collection: (child: string) => collection(path + "/" + child),
      onSnapshot: (next: (s: any) => void, onError?: (e: any) => void) =>
        subscribe(get, next, onError),
    });
  }
  function collection(
    path: string,
    query: {
      filters: any[];
      order?: string;
      direction?: string;
      limit?: number;
    } = { filters: [] },
  ) {
    validatePath(path, false);
    let cached: any[] | undefined;
    const relevant = () =>
      [...pending.entries()].filter(
        ([key]) =>
          key.startsWith(path + "/") &&
          !key.slice(path.length + 1).includes("/"),
      );
    const get = async () => {
      const local = relevant();
      let docs: any[] =
        local.length && cached
          ? structuredClone(cached)
          : await transport.rpc("db.query", { query: { path, ...query } });
      if (!local.length) {
        cached = structuredClone(docs);
        for (const doc of docs) committed.set(doc.path, doc);
      }
      const overlays = relevant();
      for (const [key, body] of overlays) {
        docs = docs.filter((d) => d.path !== key);
        if (
          body !== null &&
          query.filters.every((f) => matches(body[f.field], f))
        )
          docs.push({
            path: key,
            id: key.split("/").at(-1),
            body,
            exists: true,
          });
      }
      docs.sort((a, b) => {
        if (query.order) {
          const av = a.body[query.order],
            bv = b.body[query.order];
          if (av === undefined && bv !== undefined) return 1;
          if (av !== undefined && bv === undefined) return -1;
          if (av !== undefined) {
            const order = compare(av, bv);
            if (order) return query.direction === "desc" ? -order : order;
          }
        }
        return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
      });
      if (query.limit) docs = docs.slice(0, query.limit);
      const snapshots = docs.map((d: any) =>
        docSnapshot(d, pending.has(d.path)),
      );
      return Object.freeze({
        docs: Object.freeze(snapshots),
        size: snapshots.length,
        empty: !snapshots.length,
        metadata: Object.freeze({
          fromCache: false,
          hasPendingWrites: overlays.length > 0,
        }),
        docChanges: () =>
          Object.freeze(
            snapshots.map((doc: any, newIndex: number) =>
              Object.freeze({ type: "added", doc, oldIndex: -1, newIndex }),
            ),
          ),
      });
    };
    return Object.freeze({
      path,
      doc: (uid = crypto.randomUUID()) => document(path + "/" + uid),
      add: async (body: Data) => {
        const ref = document(path + "/" + crypto.randomUUID());
        await ref.set(body);
        return ref;
      },
      where: (field: string, op: string, value: any) =>
        collection(path, {
          ...query,
          filters: [
            ...query.filters,
            { field, op, value: structuredClone(value) },
          ],
        }),
      orderBy: (field: string, direction = "asc") => {
        if (query.order) throw new TypeError("One orderBy per query");
        return collection(path, { ...query, order: field, direction });
      },
      limit: (n: number) => {
        if (!Number.isInteger(n) || n < 1 || n > 1000)
          throw new TypeError("Limit must be 1–1000");
        return collection(path, { ...query, limit: n });
      },
      get,
      onSnapshot: (next: (s: any) => void, onError?: (e: any) => void) => {
        let old: any[] = [];
        return subscribe(
          async () => {
            const snap = await get();
            const working = old.slice();
            const changes: any[] = [];
            for (let i = working.length - 1; i >= 0; i--) {
              if (!snap.docs.some((d: any) => d.id === working[i].id)) {
                changes.push(
                  Object.freeze({
                    type: "removed",
                    doc: working[i],
                    oldIndex: i,
                    newIndex: -1,
                  }),
                );
                working.splice(i, 1);
              }
            }
            snap.docs.forEach((doc: any, index: number) => {
              const oldIndex = working.findIndex((d) => d.id === doc.id);
              if (oldIndex < 0) {
                working.splice(index, 0, doc);
                changes.push(
                  Object.freeze({
                    type: "added",
                    doc,
                    oldIndex: -1,
                    newIndex: index,
                  }),
                );
              } else if (oldIndex !== index || working[oldIndex] !== doc) {
                working.splice(oldIndex, 1);
                working.splice(index, 0, doc);
                changes.push(
                  Object.freeze({
                    type: "modified",
                    doc,
                    oldIndex,
                    newIndex: index,
                  }),
                );
              }
            });
            old = [...snap.docs];
            return Object.freeze({
              ...snap,
              docChanges: () => Object.freeze(changes),
            });
          },
          next,
          onError,
        );
      },
    });
  }
  return Object.freeze({ doc: document, collection });
}
