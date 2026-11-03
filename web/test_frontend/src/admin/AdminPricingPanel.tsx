import { useCallback, useEffect, useState } from 'react'
import { adminApi, type AdminProfile } from './adminApi.ts'

interface Props {
  profile: AdminProfile | null
}

interface ModelPrice {
  id: string
  provider: string
  model: string
  input_per_1k: number
  output_per_1k: number
  currency: string
  enabled: boolean
  created_at: number
  updated_at: number
}

interface PriceForm {
  id: string
  provider: string
  model: string
  input: string
  output: string
  currency: string
}

const EMPTY_FORM: PriceForm = { id: '', provider: '', model: '', input: '', output: '', currency: 'CNY' }

function formatTime(ts: number): string {
  if (!ts) return '—'
  return new Date(ts).toLocaleString('zh-CN', { hour12: false })
}

export default function AdminPricingPanel({ profile }: Props) {
  const [items, setItems] = useState<ModelPrice[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [form, setForm] = useState<PriceForm>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    const res = await adminApi('GET', '/api/admin/billing/prices?limit=200')
    if (res.code === 0) {
      setItems(res.data?.prices || [])
    } else {
      setError(res.msg || '加载失败')
    }
    setLoading(false)
  }, [])

  useEffect(() => { load() }, [load])

  const startEdit = (item: ModelPrice) => {
    setForm({
      id: item.id,
      provider: item.provider,
      model: item.model,
      input: `${item.input_per_1k}`,
      output: `${item.output_per_1k}`,
      currency: item.currency || 'CNY',
    })
  }

  const save = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.model) { setError('请填写模型名'); return }
    const input = Number(form.input || '0')
    const output = Number(form.output || '0')
    if (!Number.isFinite(input) || !Number.isFinite(output) || input < 0 || output < 0) {
      setError('单价需为非负数字')
      return
    }
    setSaving(true)
    setError('')
    const res = await adminApi('POST', '/api/admin/billing/prices', {
      id: form.id,
      provider: form.provider,
      model: form.model,
      input_per_1k: input,
      output_per_1k: output,
      currency: form.currency || 'CNY',
      enabled: true,
    })
    if (res.code === 0) {
      setForm(EMPTY_FORM)
      await load()
    } else {
      setError(res.data || res.msg || '保存失败')
    }
    setSaving(false)
  }

  const remove = async (item: ModelPrice) => {
    setError('')
    const res = await adminApi('POST', '/api/admin/billing/prices/delete', { id: item.id })
    if (res.code === 0) {
      if (form.id === item.id) setForm(EMPTY_FORM)
      await load()
    } else {
      setError(res.data || res.msg || '删除失败')
    }
  }

  return (
    <div className="admin-panel">
      <div className="admin-panel-head">
        <h2>参考单价</h2>
        <button className="admin-btn ghost" onClick={load} disabled={loading}>刷新</button>
      </div>

      {error && <div className="admin-alert">{error}</div>}

      <table className="admin-table">
        <thead>
          <tr>
            <th>模型</th>
            <th>提供方</th>
            <th>输入 / 1K</th>
            <th>输出 / 1K</th>
            <th>币种</th>
            <th>状态</th>
            <th>更新时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          {items.map(item => (
            <tr key={item.id}>
              <td>{item.model}</td>
              <td>{item.provider || '—'}</td>
              <td>{item.input_per_1k}</td>
              <td>{item.output_per_1k}</td>
              <td>{item.currency || 'CNY'}</td>
              <td>
                <span className={item.enabled ? 'admin-tag ok' : 'admin-tag off'}>
                  {item.enabled ? '启用' : '停用'}
                </span>
              </td>
              <td>{formatTime(item.updated_at)}</td>
              <td>
                <button className="admin-btn ghost" onClick={() => startEdit(item)}>编辑</button>
                <button className="admin-btn danger" onClick={() => remove(item)}>删除</button>
              </td>
            </tr>
          ))}
          {items.length === 0 && !loading && (
            <tr><td colSpan={8} className="admin-empty">暂无参考单价</td></tr>
          )}
        </tbody>
      </table>

      <form className="admin-inline-form" onSubmit={save}>
        <h3>{form.id ? `编辑：${form.model}` : '新增参考单价'}</h3>
        <div className="admin-form-row">
          <input
            placeholder="提供方（可选）"
            value={form.provider}
            onChange={e => setForm({ ...form, provider: e.target.value })}
          />
          <input
            placeholder="模型名，如 gpt-4o"
            value={form.model}
            onChange={e => setForm({ ...form, model: e.target.value })}
          />
          <input
            placeholder="输入 / 1K"
            value={form.input}
            onChange={e => setForm({ ...form, input: e.target.value })}
          />
          <input
            placeholder="输出 / 1K"
            value={form.output}
            onChange={e => setForm({ ...form, output: e.target.value })}
          />
          <input
            placeholder="币种"
            value={form.currency}
            onChange={e => setForm({ ...form, currency: e.target.value })}
          />
          <button type="submit" className="admin-btn primary" disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </button>
          {form.id && (
            <button type="button" className="admin-btn ghost" onClick={() => setForm(EMPTY_FORM)}>取消</button>
          )}
        </div>
      </form>

      <p className="admin-hint">
        当前管理员：{profile?.name || '—'}。参考单价仅用于估算用户自带 apikey 的 Token 成本，按每 1K Token 单价手工维护；
        未配置单价的模型只记录 Token 用量，不产生成本。
      </p>
    </div>
  )
}
