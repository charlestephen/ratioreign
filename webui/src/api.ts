export async function api<T>(path: string, opts?: RequestInit): Promise<T> {
  const resp = await fetch(path, opts);
  const text = await resp.text();
  let body: unknown;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = text;
  }
  if (!resp.ok) {
    const msg =
      body && typeof body === 'object' && 'error' in body
        ? String((body as { error: unknown }).error)
        : typeof body === 'string'
          ? body
          : resp.statusText;
    throw new Error(msg);
  }
  return body as T;
}

export function humanBytes(n: number): string {
  if (n < 1024) return n + ' B';
  const units = ['KB', 'MB', 'GB', 'TB'];
  let u = -1;
  do {
    n /= 1024;
    u++;
  } while (n >= 1024 && u < units.length - 1);
  return n.toFixed(1) + ' ' + units[u];
}

export function jsonPost(body: unknown): RequestInit {
  return {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  };
}
