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

export interface UploadResult {
  url: string;
  file_name: string;
  file_size: number;
  media_type: 'image' | 'voice' | 'file';
}

export async function uploadFile(file: File): Promise<{ code: number; msg: string; data?: UploadResult }> {
  const token = localStorage.getItem('im_token');
  const formData = new FormData();
  formData.append('file', file);
  try {
    const res = await fetch(API_BASE + '/api/files', {
      method: 'POST',
      headers: token ? { 'Authorization': 'Bearer ' + token } : {},
      body: formData,
    });
    if (res.status === 401) return { code: 1, msg: '身份鉴权失败' };
    return await res.json();
  } catch (e: any) {
    return { code: 1, msg: '上传失败' };
  }
}

export function parseMessageContent(content: string): { type: string; text?: string; url?: string; filename?: string; size?: number; width?: number; height?: number; duration?: number } {
  try {
    const parsed = JSON.parse(content);
    if (parsed && parsed.type) return parsed;
  } catch {}
  return { type: 'text', text: content };
}

export function buildTextContent(text: string): string {
  return JSON.stringify({ type: 'text', text });
}

export function buildMediaContent(mediaType: 'image' | 'file' | 'voice', url: string, extra: Record<string, any> = {}): string {
  return JSON.stringify({ type: mediaType, url, ...extra });
}
