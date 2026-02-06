export async function getJson(url) {
  const res = await fetch(url, {
    headers: { "Accept": "application/json" }
  });
  if (!res.ok) {
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

export async function postJson(url, body) {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json", "Accept": "application/json" },
    body: JSON.stringify(body)
  });
  if (!res.ok) {
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

export async function deleteRequest(url) {
  const res = await fetch(url, { method: "DELETE" });
  if (!res.ok) {
    const message = await safeErrorMessage(res);
    throw new Error(message || `Request failed: ${res.status}`);
  }
  return res.json();
}

async function safeErrorMessage(res) {
  try {
    const data = await res.json();
    return data?.detail || data?.message;
  } catch (_err) {
    return "";
  }
}
