// Shared by the public inline runner and the React instance page. Only the
// exact child window on the instance origin may request a scoped launch code.
export function bindInstanceLaunch(
  frame: HTMLIFrameElement,
  origin: string,
  exchange: (challenge: string) => Promise<{ code: string }>,
  status: (message: string) => void,
): () => void {
  let active = true;
  const receive = async (event: MessageEvent) => {
    if (event.origin !== origin || event.source !== frame.contentWindow) return;
    const data = event.data;
    if (data?.type === "francois-launch-started") {
      status("");
      return;
    }
    if (
      data?.type === "francois-launch-error" &&
      typeof data.message === "string"
    ) {
      status(data.message);
      return;
    }
    if (
      data?.type !== "francois-launch-ready" ||
      typeof data.challenge !== "string"
    )
      return;
    const target = frame.contentWindow!;
    try {
      const result = await exchange(data.challenge);
      if (!active) return;
      target.postMessage(
        {
          type: "francois-launch-code",
          code: result.code,
          challenge: data.challenge,
        },
        origin,
      );
    } catch (error: any) {
      if (!active) return;
      const message =
        error?.data?.error?.message ??
        error?.message ??
        "This artifact cannot be opened right now.";
      target.postMessage(
        {
          type: "francois-launch-denied",
          code: error?.data?.error?.code ?? error?.code ?? "denied",
          message,
          challenge: data.challenge,
        },
        origin,
      );
      status(message);
    }
  };
  addEventListener("message", receive);
  return () => {
    active = false;
    removeEventListener("message", receive);
  };
}
