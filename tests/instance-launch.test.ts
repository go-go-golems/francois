import test from "node:test";
import assert from "node:assert/strict";
import { bindInstanceLaunch } from "../ui/catalog/instance-launch";

test("instance launch only accepts its exact child and origin, and cleans up pending replies", async () => {
  const target = new EventTarget();
  const oldAdd = globalThis.addEventListener;
  const oldRemove = globalThis.removeEventListener;
  globalThis.addEventListener = target.addEventListener.bind(
    target,
  ) as typeof addEventListener;
  globalThis.removeEventListener = target.removeEventListener.bind(
    target,
  ) as typeof removeEventListener;
  const replies: unknown[] = [];
  const child = {
    postMessage: (message: unknown, origin: string) =>
      replies.push({ message, origin }),
  };
  const frame = { contentWindow: child } as unknown as HTMLIFrameElement;
  let exchanges = 0;
  let resolve!: (value: { code: string }) => void;
  const states: string[] = [];
  const cleanup = bindInstanceLaunch(
    frame,
    "https://instance.test",
    async () => {
      exchanges++;
      return new Promise<{ code: string }>((r) => {
        resolve = r;
      });
    },
    (message) => states.push(message),
  );
  const emit = (
    origin: string,
    source: unknown,
    type = "francois-launch-ready",
  ) => {
    const event = new Event("message");
    Object.defineProperties(event, {
      origin: { value: origin },
      source: { value: source },
      data: { value: { type, challenge: "challenge" } },
    });
    target.dispatchEvent(event);
  };
  try {
    emit("https://evil.test", child);
    emit("https://instance.test", {});
    assert.equal(exchanges, 0);
    emit("https://instance.test", child);
    assert.equal(exchanges, 1);
    resolve({ code: "scoped-code" });
    await new Promise(setImmediate);
    assert.deepEqual(replies, [
      {
        message: {
          type: "francois-launch-code",
          code: "scoped-code",
          challenge: "challenge",
        },
        origin: "https://instance.test",
      },
    ]);
    assert.deepEqual(states, []); // Minting a code is not proof that the page loaded.
    emit("https://instance.test", child, "francois-launch-started");
    assert.deepEqual(states, [""]);
    emit("https://instance.test", child);
    cleanup();
    resolve({ code: "stale" });
    await new Promise(setImmediate);
    assert.equal(replies.length, 1);
    emit("https://instance.test", child);
    assert.equal(exchanges, 2);
  } finally {
    cleanup();
    globalThis.addEventListener = oldAdd;
    globalThis.removeEventListener = oldRemove;
  }
});
