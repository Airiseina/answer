import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { api } from '../api/client.ts'
import './SummariesPanel.css'

interface SummaryReport {
  id: string
  user_id: string
  type: string
  period_start: number
  period_end: number
  content: string
  model: string
  status: number
  error: string
  created_at: number
}

interface ScheduleInfo {
  daily_enabled: boolean
  weekly_enabled: boolean
}

// 总结状态：0 待生成 / 1 生成中 / 2 完成 / 3 失败
const STATUS_LABELS: Record<number, string> = {
  0: '待生成',
  1: '生成中',
  2: '已完成',
  3: '生成失败',
}

function pad(value: number): string {
  return value < 10 ? `0${value}` : `${value}`
}

function formatPeriod(type: string, start: number, end: number): string {
  const startDate = new Date(start)
  if (type === 'weekly') {
    const endDate = new Date(end)
    return `${startDate.getFullYear()}年${startDate.getMonth() + 1}月${startDate.getDate()}日 - ${endDate.getMonth() + 1}月${endDate.getDate()}日`
  }
  return `${startDate.getFullYear()}年${startDate.getMonth() + 1}月${startDate.getDate()}日`
}

function formatTime(ms: number): string {
  if (!ms) return ''
  const date = new Date(ms)
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export default function SummariesPanel() {
  const [tab, setTab] = useState<'daily' | 'weekly' | 'schedule'>('daily')
  const [reports, setReports] = useState<SummaryReport[]>([])
  const [loading, setLoading] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [toast, setToast] = useState('')
  const [schedule, setSchedule] = useState<ScheduleInfo>({ daily_enabled: true, weekly_enabled: true })
  const [savingSchedule, setSavingSchedule] = useState(false)

  const showToast = (msg: string) => {
    setToast(msg)
    window.setTimeout(() => setToast(''), 2500)
  }

  const loadReports = async (type: 'daily' | 'weekly') => {
    setLoading(true)
    const res = await api('GET', `/api/summary/${type}`)
    setLoading(false)
    if (res.code !== 0) {
      setReports([])
      showToast(res.msg || '加载总结失败')
      return
    }
    setReports(res.data?.reports ?? [])
    setExpandedId(null)
  }

  const loadSchedule = async () => {
    const res = await api('GET', '/api/summary/schedule')
    if (res.code === 0 && res.data) {
      setSchedule({
        daily_enabled: !!res.data.daily_enabled,
        weekly_enabled: !!res.data.weekly_enabled,
      })
    }
  }

  useEffect(() => {
    if (tab === 'schedule') {
      loadSchedule()
    } else {
      loadReports(tab)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tab])

  const handleGenerate = async () => {
    if (tab === 'schedule') return
    setGenerating(true)
    const res = await api('POST', '/api/summary/generate', { type: tab })
    setGenerating(false)
    if (res.code !== 0) {
      showToast(res.msg || '生成总结失败')
      return
    }
    showToast('总结已生成')
    loadReports(tab)
  }

  const handleSaveSchedule = async () => {
    setSavingSchedule(true)
    const res = await api('POST', '/api/summary/schedule', schedule)
    setSavingSchedule(false)
    if (res.code !== 0) {
      showToast(res.msg || '保存订阅失败')
      return
    }
    showToast('订阅设置已保存')
  }

  return (
    <div className="bots-panel summary-panel">
      {toast && <div className="bots-toast">{toast}</div>}
      <div className="bots-header">
        <h3>总结</h3>
      </div>
      <div className="bots-tabs">
        <button className={`ctab ${tab === 'daily' ? 'active' : ''}`} onClick={() => setTab('daily')}>日总结</button>
        <button className={`ctab ${tab === 'weekly' ? 'active' : ''}`} onClick={() => setTab('weekly')}>周总结</button>
        <button className={`ctab ${tab === 'schedule' ? 'active' : ''}`} onClick={() => setTab('schedule')}>订阅设置</button>
      </div>
      <div className="bots-content">
        {tab !== 'schedule' && (
          <>
            <div className="summary-toolbar">
              <span className="summary-hint">每天 21:00 由系统助手发送，周日额外发送周总结。</span>
              <button className="summary-btn" onClick={handleGenerate} disabled={generating}>
                {generating ? '生成中…' : '立即生成'}
              </button>
            </div>
            {loading ? (
              <div className="summary-empty">加载中…</div>
            ) : reports.length === 0 ? (
              <div className="summary-empty">暂无{tab === 'daily' ? '日' : '周'}总结</div>
            ) : (
              <div className="summary-list">
                {reports.map((report) => {
                  const expanded = expandedId === report.id
                  return (
                    <div key={report.id} className="summary-card">
                      <div className="summary-card-head" onClick={() => setExpandedId(expanded ? null : report.id)}>
                        <div className="summary-card-title">{formatPeriod(report.type, report.period_start, report.period_end)}</div>
                        <div className="summary-card-meta">
                          <span className={`summary-status status-${report.status}`}>{STATUS_LABELS[report.status] ?? '未知'}</span>
                          {report.model && <span className="summary-model">{report.model}</span>}
                          <span className="summary-time">{formatTime(report.created_at)}</span>
                        </div>
                      </div>
                      {expanded && (
                        <div className="summary-card-body">
                          {report.status === 3 && report.error ? (
                            <div className="summary-error">{report.error}</div>
                          ) : (
                            <ReactMarkdown remarkPlugins={[remarkGfm]}>{report.content || '（暂无内容）'}</ReactMarkdown>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </>
        )}

        {tab === 'schedule' && (
          <div className="summary-schedule">
            <label className="summary-toggle">
              <input
                type="checkbox"
                checked={schedule.daily_enabled}
                onChange={(e) => setSchedule({ ...schedule, daily_enabled: e.target.checked })}
              />
              <span>每日总结（每天 21:00 发送）</span>
            </label>
            <label className="summary-toggle">
              <input
                type="checkbox"
                checked={schedule.weekly_enabled}
                onChange={(e) => setSchedule({ ...schedule, weekly_enabled: e.target.checked })}
              />
              <span>周总结（每周日 21:00 发送）</span>
            </label>
            <button className="summary-btn" onClick={handleSaveSchedule} disabled={savingSchedule}>
              {savingSchedule ? '保存中…' : '保存设置'}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
