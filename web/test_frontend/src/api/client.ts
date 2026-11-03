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
  return uploadFileTo('/api/files', file);
}

// uploadFileTo 以 multipart 方式把文件提交到任意上传接口。
// 目前用于 /api/files（通用附件）与 /api/skill/upload（个人 Skill 包）两处。
export async function uploadFileTo<T = any>(path: string, file: File): Promise<{ code: number; msg: string; data?: T }> {
  const token = localStorage.getItem('im_token');
  const formData = new FormData();
  formData.append('file', file);
  try {
    const res = await fetch(API_BASE + path, {
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

export interface MentionItem {
  user_id: string
  name: string
}

export function parseMessageContent(content: string): { type: string; text?: string; url?: string; filename?: string; size?: number; width?: number; height?: number; duration?: number; mentions?: MentionItem[] } {
  try {
    const parsed = JSON.parse(content)
    if (parsed && parsed.type) return parsed
  } catch {}
  return { type: 'text', text: content }
}

export function buildTextContent(text: string, mentions?: MentionItem[]): string {
  const obj: any = { type: 'text', text }
  if (mentions && mentions.length > 0) {
    obj.mentions = mentions
  }
  return JSON.stringify(obj)
}

export function buildMediaContent(mediaType: 'image' | 'file' | 'voice', url: string, extra: Record<string, any> = {}): string {
  return JSON.stringify({ type: mediaType, url, ...extra });
}
