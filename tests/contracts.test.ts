import { test } from "node:test";
import assert from "node:assert/strict";
import { normalizeInput, parseJSON } from "../ui/src/sample-contract";
import { installDB, validatePath } from "../ui/src/runtime-db";
test("normalization copies turns and merges adjacent roles", () => {
  const input = [
    { role: "user", content: "rules" },
    { role: "user", content: "question" },
  ];
  const output = normalizeInput(input);
  input[0].content = "changed";
  assert.deepEqual(output, [{ role: "user", content: "rules\n\nquestion" }]);
  assert.throws(
    () => normalizeInput("é".repeat(131073)),
    (e: any) => e.code === "prompt_too_large",
  );
  assert.throws(() => normalizeInput([{ role: "system", content: "x" }]));
});
test("JSON reads final fenced values and rejects truncated completions", () => {
  assert.deepEqual(parseJSON('Result:\n```json\n{"ok":true}\n```'), {
    ok: true,
  });
  assert.deepEqual(parseJSON("Here is [1,2] as requested"), [1, 2]);
  assert.throws(
    () => parseJSON('{"ok":true}', true),
    (e: any) => e.code === "invalid_json" && e.text === '{"ok":true}',
  );
});
test("refs validate synchronously and query snapshots retain identity", async () => {
  let refresh: (e: any) => void = () => {};
  let docs = [
    { id: "a", path: "tasks/a", body: { rank: 1 }, exists: true, version: 1 },
    { id: "b", path: "tasks/b", body: { rank: 2 }, exists: true, version: 1 },
  ];
  const db = installDB({
    rpc: async (method) => {
      assert.equal(method, "db.query");
      return structuredClone(docs);
    },
    subscribe: (fn) => {
      refresh = fn;
      return () => {};
    },
  });
  assert.throws(() => db.doc("tasks"), TypeError);
  assert.throws(() => validatePath("a//b", true));
  const snapshots: any[] = [];
  const stop = db
    .collection("tasks")
    .orderBy("rank")
    .onSnapshot((s) => snapshots.push(s));
  await new Promise((r) => setTimeout(r, 10));
  const first = snapshots[0];
  assert(Object.isFrozen(first.docs[0].data()));
  refresh({ type: "db_changed" });
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(snapshots[1].docs[0], first.docs[0]);
  assert.deepEqual(snapshots[1].docChanges(), []);
  docs = [{ ...docs[1], body: { rank: 0 }, version: 2 }, docs[0]];
  refresh({ type: "db_changed" });
  await new Promise((r) => setTimeout(r, 10));
  const moved = snapshots[2].docChanges();
  assert.equal(moved[0].type, "modified");
  assert.equal(moved[0].oldIndex, 1);
  assert.equal(moved[0].newIndex, 0);
  stop();
});

test("local writes emit pending snapshots and roll back rejected changes", async () => {
  let release: (v?: unknown) => void = () => {};
  let failWrite: (e: any) => void = () => {};
  const wire = {
    rpc: async (method: string) => {
      if (method === "db.query") return [];
      if (method === "db.set")
        return new Promise((resolve, reject) => {
          release = resolve;
          failWrite = reject;
        });
      throw new Error("unexpected method " + method);
    },
    subscribe: () => () => {},
  };
  const db = installDB(wire);
  const snapshots: any[] = [];
  const stop = db.collection("tasks").onSnapshot((s) => snapshots.push(s));
  await new Promise((r) => setTimeout(r, 10));
  const write = db.doc("tasks/a").set({ title: "pending" });
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(snapshots.at(-1).metadata.hasPendingWrites, true);
  assert.equal(snapshots.at(-1).docs[0].data().title, "pending");
  failWrite({ code: "unavailable", message: "test failure" });
  await assert.rejects(write);
  await new Promise((r) => setTimeout(r, 10));
  assert.equal(snapshots.at(-1).empty, true);
  assert.equal(snapshots.at(-1).metadata.hasPendingWrites, false);
  stop();
});

test("provider failures distinguish authentication and limits without leaking diagnostics", async () => {
  const { ProviderError } = await import("../worker/src/provider-error");
  const auth = new ProviderError(
    "401 unauthorized; secret=example-provider-key",
  );
  assert.equal(auth.code, "authentication_error");
  assert.equal(auth.message.includes("example-provider-key"), false);
  assert.equal(new ProviderError("429 too many requests").code, "rate_limited");
  assert.equal(new ProviderError("socket disconnected").code, "upstream_error");
});
