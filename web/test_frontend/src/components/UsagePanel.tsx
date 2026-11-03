import { useEffect, useState } from 'react'
import { api } from '../api/client.ts'
import './UsagePanel.css'

interface TrendPoint {
  biz_date: string
  prompt_tokens: number
  completion_tokens: number
  cost: number
  request_count: number
}

interface GroupStat {
  key: string
  prompt_tokens: number
  completion_tokens: number
  cost: number
  request_count: number
}

interface UsageSummary {
  prompt_tokens: number
  completion_tokens: number
  cost: number
  request_count: number
  trend: TrendPoint[]
  by_bot: GroupStat[]
  by_model: GroupStat[]
  missing_price_count: number
}

type RangeKey = 'today' | 'week' | 'month'

const RANGES: { key: RangeKey; label: string; days: number }[] = [
  { key: 'today', label: '今日', days: 1 },
  { key: 'week', label: '近 7 天', days: 7 },
  { key: 'month', label: '近 30 天', days: 30 },
]

const EMPTY_SUMMARY: UsageSummary = {
  prompt_tokens: 0,
  completion_tokens: 0,
  cost: 0,
  request_count: 0,
  trend: [],
  by_bot: [],
  by_model: [],
  missing_price_count: 0,
}

// 后端按东八区切分自然日；这里以本地时区生成同样的 yyyy-mm-dd。
function formatDate(date: Date): string {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

function rangeDates(days: number): { start: string; end: string } {
  const end = new Date()
  const start = new Date()
  start.setDate(end.getDate() - (days - 1))
  return { start: formatDate(start), end: formatDate(end) }
}

function formatNumber(value: number): string {
  return value.toLocaleString('zh-CN')
}

function formatCost(value: number): string {
  return `¥${value.toFixed(4)}`
}

function totalTokens(stat: { prompt_tokens: number; completion_tokens: number }): number {
  return stat.prompt_tokens + stat.completion_tokens
}

export default function UsagePanel() {
  const [range, setRange] = useState<RangeKey>('week')
  const [summary, setSummary] = useState<UsageSummary>(EMPTY_SUMMARY)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    const days = RANGES.find((item) => item.key === range)?.days ?? 7
    const { start, end } = rangeDates(days)
    setLoading(true)
    setError('')
    api('GET', `/api/billing/usage/summary?start_date=${start}&end_date=${end}`).then((res) => {
      if (cancelled) return
      setLoading(false)
      if (res.code !== 0) {
        setSummary(EMPTY_SUMMARY)
        setError(res.msg || '加载用量失败')
        return
      }
      const data = res.data ?? {}
      setSummary({
        prompt_tokens: data.prompt_tokens ?? 0,
        completion_tokens: data.completion_tokens ?? 0,
        cost: data.cost ?? 0,
        request_count: data.request_count ?? 0,
        trend: data.trend ?? [],
        by_bot: data.by_bot ?? [],
        by_model: data.by_model ?? [],
        missing_price_count: data.missing_price_count ?? 0,
      })
    })
    return () => {
      cancelled = true
    }
  }, [range])

  const modelMax = summary.by_model.reduce((max, item) => Math.max(max, totalTokens(item)), 0)
  const botMax = summary.by_bot.reduce((max, item) => Math.max(max, totalTokens(item)), 0)
  const trendMax = summary.trend.reduce((max, item) => Math.max(max, totalTokens(item)), 0)

  return (
    <div className="bots-panel usage-panel">
      <div className="bots-header">
        <h3>用量</h3>
      </div>
      <div className="usage-range">
        <div className="usage-segmented">
          {RANGES.map((item) => (
            <button
              key={item.key}
              className={`usage-seg ${range === item.key ? 'active' : ''}`}
              onClick={() => setRange(item.key)}
            >
              {item.label}
            </button>
          ))}
        </div>
        <span className="usage-hint">预估成本，仅供参考</span>
      </div>

      <div className="bots-content">
        {error && <div className="usage-error">{error}</div>}
        {loading ? (
          <div className="usage-empty">加载中…</div>
        ) : (
          <>
            <div className="usage-cards">
              <div className="usage-card">
                <div className="usage-card-label">总 Token</div>
                <div className="usage-card-value">{formatNumber(totalTokens(summary))}</div>
                <div className="usage-card-sub">
                  输入 {formatNumber(summary.prompt_tokens)} · 输出 {formatNumber(summary.completion_tokens)}
                </div>
              </div>
              <div className="usage-card">
                <div className="usage-card-label">请求次数</div>
                <div className="usage-card-value">{formatNumber(summary.request_count)}</div>
              </div>
              <div className="usage-card">
                <div className="usage-card-label">预估成本</div>
                <div className="usage-card-value">{formatCost(summary.cost)}</div>
              </div>
            </div>

            {summary.missing_price_count > 0 && (
              <div className="usage-notice">
                有 {formatNumber(summary.missing_price_count)} 次调用未配置参考单价，成本未计入。
              </div>
            )}

            <div className="usage-section">
              <div className="usage-section-title">每日趋势</div>
              {summary.trend.length === 0 ? (
                <div className="usage-empty">该时间段暂无用量</div>
              ) : (
                <div className="usage-trend">
                  {summary.trend.map((point) => (
                    <div key={point.biz_date} className="usage-trend-row">
                      <span className="usage-trend-date">{point.biz_date}</span>
                      <div className="usage-trend-bar">
                        <div
                          className="usage-trend-fill"
                          style={{ width: `${trendMax > 0 ? (totalTokens(point) / trendMax) * 100 : 0}%` }}
                        />
                      </div>
                      <span className="usage-trend-value">{formatNumber(totalTokens(point))}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="usage-section">
              <div className="usage-section-title">按模型占比</div>
              {summary.by_model.length === 0 ? (
                <div className="usage-empty">暂无数据</div>
              ) : (
                <div className="usage-groups">
                  {summary.by_model.map((item) => (
                    <div key={item.key} className="usage-group-row">
                      <span className="usage-group-key" title={item.key}>{item.key || '未知模型'}</span>
                      <div className="usage-group-bar">
                        <div
                          className="usage-group-fill"
                          style={{ width: `${modelMax > 0 ? (totalTokens(item) / modelMax) * 100 : 0}%` }}
                        />
                      </div>
                      <span className="usage-group-value">{formatNumber(totalTokens(item))}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="usage-section">
              <div className="usage-section-title">按 Bot 占比</div>
              {summary.by_bot.length === 0 ? (
                <div className="usage-empty">暂无数据</div>
              ) : (
                <div className="usage-groups">
                  {summary.by_bot.map((item) => (
                    <div key={item.key} className="usage-group-row">
                      <span className="usage-group-key" title={item.key}>Bot {item.key}</span>
                      <div className="usage-group-bar">
                        <div
                          className="usage-group-fill"
                          style={{ width: `${botMax > 0 ? (totalTokens(item) / botMax) * 100 : 0}%` }}
                        />
                      </div>
                      <span className="usage-group-value">{formatNumber(totalTokens(item))}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}
