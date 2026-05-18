const API_BASE = '';

export async function api(method: string, path: string, body?: unknown) {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  const token = localStorage.getItem('im_token');
  if (token) (opts.headers as Record<string, string>)['Authorization'] = 'Bearer ' + token;
  if (body) opts.body = JSON.stringify(body);
  try {
    const res = await fetch(API_BASE + path, opts);
    if (res.status === 401) return { code: 1, msg: '身份鉴权失败', data: 'Token 无效或已过期' };
    return await res.json();
  } catch (e: any) {
    return { code: 1, msg: '请求失败', data: e.message };
  }
}
