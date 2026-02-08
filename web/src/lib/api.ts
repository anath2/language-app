export async function getJson<T = unknown>(url: string): Promise<T> {
  const res = await fetch(url, {
    headers: { "Accept": "application/json" },
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

export async function postJson<T = unknown>(url: string, body: unknown): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Accept": "application/json" },
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

export async function postJsonForm<T = unknown>(url: string, formData: FormData): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
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

export async function deleteRequest<T = unknown>(url: string): Promise<T> {
  const res = await fetch(url, {
    method: "DELETE",
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

async function safeErrorMessage(res: Response): Promise<string> {
  try {
    const data = await res.json();
    return data?.detail || data?.message;
  } catch (_err) {
    return "";
  }
}
