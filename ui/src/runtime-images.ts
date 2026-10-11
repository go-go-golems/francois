export const imageLimits = Object.freeze({
  maxCount: 4,
  maxInputBytes: 8 * 1024 * 1024,
  mediaTypes: Object.freeze([
    "image/jpeg",
    "image/png",
    "image/webp",
    "image/gif",
  ]),
});
export function captureImages(input: unknown): Blob[] {
  const files =
    input instanceof Blob
      ? [input]
      : input instanceof FileList
        ? Array.from(input)
        : Array.isArray(input)
          ? input
          : [];
  if (
    !files.length ||
    files.length > imageLimits.maxCount ||
    files.some(
      (b) => !(b instanceof Blob) || !imageLimits.mediaTypes.includes(b.type),
    )
  )
    throw {
      code: "image_rejected",
      message: "Supply 1–4 JPEG, PNG, WebP or GIF images",
    };
  if (files.reduce((n, b) => n + b.size, 0) > imageLimits.maxInputBytes)
    throw { code: "image_rejected", message: "Images exceed 8 MiB" };
  return [...files];
}
export async function prepareImages(files: Blob[], signal?: AbortSignal) {
  const images: { mimeType: string; data: string }[] = [];
  const hashes: string[] = [];
  for (const blob of files) {
    if (signal?.aborted) throw { code: "cancelled", message: "Stopped" };
    const source = await blob.arrayBuffer();
    const hash = await crypto.subtle.digest("SHA-256", source);
    hashes.push(
      Array.from(new Uint8Array(hash), (b) =>
        b.toString(16).padStart(2, "0"),
      ).join(""),
    );
    let bitmap: ImageBitmap;
    try {
      bitmap = await createImageBitmap(blob);
    } catch {
      throw { code: "image_rejected", message: "Image could not be decoded" };
    }
    try {
      const scale = Math.min(
        1,
        Math.sqrt(1200000 / (bitmap.width * bitmap.height)),
      );
      const canvas = document.createElement("canvas");
      canvas.width = Math.max(1, Math.round(bitmap.width * scale));
      canvas.height = Math.max(1, Math.round(bitmap.height * scale));
      const ctx = canvas.getContext("2d")!;
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
      const encoded = await new Promise<Blob>((resolve, reject) =>
        canvas.toBlob(
          (b) =>
            b
              ? resolve(b)
              : reject({
                  code: "image_rejected",
                  message: "Image encoding failed",
                }),
          "image/jpeg",
          0.85,
        ),
      );
      const array = new Uint8Array(await encoded.arrayBuffer());
      let binary = "";
      for (let i = 0; i < array.length; i += 8192)
        binary += String.fromCharCode(...array.subarray(i, i + 8192));
      images.push({ mimeType: "image/jpeg", data: btoa(binary) });
    } finally {
      bitmap.close();
    }
  }
  if (signal?.aborted) throw { code: "cancelled", message: "Stopped" };
  return { images, hashes };
}
