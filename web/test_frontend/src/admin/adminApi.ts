// 管理后台独立的接口客户端。
// 它使用单独的登录态（admin_token），与普通用户的 im_token 完全隔离。
const API_BASE = '';

export interface AdminResult<T = any> {
  code: number;
  msg: string;
  data: T;
}

export const ADMIN_TOKEN_KEY = 'admin_token';

export function getAdminToken(): string {
  return localStorage.getItem(ADMIN_TOKEN_KEY) || '';
}

export function setAdminToken(token: string) {
  localStorage.setItem(ADMIN_TOKEN_KEY, token);
}

export function clearAdminToken() {
  localStorage.removeItem(ADMIN_TOKEN_KEY);
}

export async function adminApi<T = any>(method: string, path: string, body?: unknown): Promise<AdminResult<T>> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  const token = getAdminToken();
  if (token) (opts.headers as Record<string, string>)['Authorization'] = 'Bearer ' + token;
  if (body) opts.body = JSON.stringify(body);
  try {
    const res = await fetch(API_BASE + path, opts);
    if (res.status === 401) return { code: 401, msg: '登录已过期', data: null as T };
    return await res.json();
  } catch (e: any) {
    return { code: -1, msg: '网络异常', data: e.message };
  }
}

export const ADMIN_ROLE_LABELS: Record<string, string> = {
  super: '超级管理员',
  ops: '运营',
  reviewer: '审核员',
};

export const ADMIN_STATUS_LABELS: Record<number, string> = {
  0: '正常',
  1: '已停用',
};

export interface AdminProfile {
  admin_id: string;
  account: string;
  name: string;
  role: string;
  last_login_at: number;
}

export interface AdminAccount {
  admin_id: string;
  account: string;
  name: string;
  role: string;
  status: number;
  last_login_at: number;
}
