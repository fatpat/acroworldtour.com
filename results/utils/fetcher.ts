export const fetcher = (...args: Parameters<typeof fetch>) =>
  fetch(...args).then((res) => {
    if (!res.ok) throw new Error(`API returned code ${res.status}`);
    return res.json();
  });
