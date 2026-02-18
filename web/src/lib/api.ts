/**
 * Performs a GET request and returns JSON response.
 * @template T - The expected response type
 * @param url - The URL to fetch
 * @returns Promise resolving to the JSON response
 * @throws Error if the request fails or returns non-OK status
 */
export async function getJson<T = unknown>(url: string): Promise<T> {
  const res = await fetch(url, {
    headers: { Accept: 'application/json' },
    credentials: 'include',
  });
  if (!res.ok) {
    if (res.status === 401) {
      // Redirect to login with return URL
      const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.href = `#/login?return=$${returnUrl}`;
      throw new Error('Authentication required');
    }
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Performs a POST request with JSON body and returns JSON response.
 * @template T - The expected response type
 * @param url - The URL to post to
 * @param body - The JSON body to send
 * @returns Promise resolving to the JSON response
 * @throws Error if the request fails or returns non-OK status
 */
export async function postJson<T = unknown>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(body),
    credentials: 'include',
  });
  if (!res.ok) {
    if (res.status === 401) {
      // Redirect to login with return URL
      const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.href = `#/login?return=$${returnUrl}`;
      throw new Error('Authentication required');
    }
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Performs a POST request with form data and returns JSON response.
 * @template T - The expected response type
 * @param url - The URL to post to
 * @param formData - The FormData to send
 * @returns Promise resolving to the JSON response
 * @throws Error if the request fails or returns non-OK status
 */
export async function postJsonForm<T = unknown>(url: string, formData: FormData): Promise<T> {
  const res = await fetch(url, {
    method: 'POST',
    body: formData,
    credentials: 'include',
  });
  if (!res.ok) {
    if (res.status === 401) {
      // Redirect to login with return URL
      const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.href = `#/login?return=$${returnUrl}`;
      throw new Error('Authentication required');
    }
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * Performs a DELETE request and returns JSON response.
 * @template T - The expected response type
 * @param url - The URL to delete from
 * @returns Promise resolving to the JSON response
 * @throws Error if the request fails or returns non-OK status
 */
export async function deleteRequest<T = unknown>(url: string): Promise<T> {
  const res = await fetch(url, {
    method: 'DELETE',
    credentials: 'include',
  });
  if (!res.ok) {
    if (res.status === 401) {
      // Redirect to login with return URL
      const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.href = `#/login?return=$${returnUrl}`;
      throw new Error('Authentication required');
    }
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

/**
 * POST with JSON body and stream response as SSE (data: JSON lines).
 * Calls onEvent for each parsed event. Resolves when the stream ends.
 * Rejects on non-OK response (e.g. 401) or if the response has no body.
 *
 * @param url - The URL to post to
 * @param body - The JSON body to send
 * @param onEvent - Callback invoked for each SSE event (parsed from "data: " lines)
 */
export async function postJsonStream<T = unknown>(
  url: string,
  body: unknown,
  onEvent: (event: T) => void
): Promise<void> {
  const res = await fetch(url, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream' },
    body: JSON.stringify(body),
    credentials: 'include',
  });
  if (!res.ok) {
    if (res.status === 401) {
      const returnUrl = encodeURIComponent(window.location.pathname + window.location.search);
      window.location.href = `#/login?return=$${returnUrl}`;
      throw new Error('Authentication required');
    }
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  if (!res.body) {
    throw new Error('Streaming unavailable');
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        const raw = line.slice(6);
        if (raw === '[DONE]' || raw.trim() === '') continue;
        try {
          const event = JSON.parse(raw) as T;
          onEvent(event);
        } catch (_e) {
          // skip malformed lines
        }
      }
    }
    if (buffer.trim()) {
      if (buffer.startsWith('data: ')) {
        try {
          const event = JSON.parse(buffer.slice(6)) as T;
          onEvent(event);
        } catch (_e) {
          // skip
        }
      }
    }
  } finally {
    reader.releaseLock();
  }
}

/**
 * Safely extracts error message from a response.
 * @param res - The response to extract error from
 * @returns The error message if available, empty string otherwise
 */
async function safeErrorMessage(res: Response): Promise<string> {
  try {
    const data = await res.json();
    return data?.detail || data?.message;
  } catch (_err) {
    return '';
  }
}
