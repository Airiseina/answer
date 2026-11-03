import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ReactFlow,
  Background,
  Controls,
  addEdge,
  useNodesState,
  useEdgesState,
  Handle,
  Position,
  MarkerType,
  type Connection,
  type Edge,
  type Node,
  type NodeProps,
  type NodeTypes,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import { api } from '../api/client.ts'
import './AgentGraphPanel.css'

interface BotOption {
  bot_id: string
  name: string
  is_system?: boolean
}

// 与后端 pkg/agentgraph 的 schema 保持一致：start / agent / end 三类节点。
type GraphNodeKind = 'start' | 'agent' | 'end'

interface GraphNodeData {
  nodeType: GraphNodeKind
  name: string
  systemPrompt: string
  model: string
  tools: string[]
  maxSteps: number
  [key: string]: unknown
}

type GraphNode = Node<GraphNodeData>
type GraphEdge = Edge

interface WireNode {
  id: string
  type: string
  name?: string
  system_prompt?: string
  model?: string
  tools?: string[]
  max_steps?: number
  x?: number
  y?: number
}

interface WireGraph {
  nodes?: WireNode[]
  edges?: { from: string; to: string }[]
}

interface GraphView {
  graph_id: string
  bot_id: string
  name: string
  graph: WireGraph | null
  status: number
  version: number
  created_at: string
  updated_at: string
}

interface HistoryItem {
  id: string
  graph_id: string
  version: number
  graph: WireGraph | null
  operator_id: string
  created_at: string
}

const NODE_KIND_RF: Record<GraphNodeKind, string> = {
  start: 'startNode',
  agent: 'agentNode',
  end: 'endNode',
}

const NODE_KIND_LABEL: Record<GraphNodeKind, string> = {
  start: '开始',
  agent: 'Agent',
  end: '结束',
}

// 编排图状态机：0 草稿 / 1 已发布 / 2 已停用
const STATUS_LABELS: Record<number, string> = {
  0: '草稿',
  1: '已发布',
  2: '已停用',
}

const TOOL_OPTIONS = [
  { value: 'knowledge', label: '知识库' },
  { value: 'memory', label: '记忆' },
]

type ToolMode = 'inherit' | 'none' | 'custom'

// toolMode 把后端工具白名单还原为面板上的三态选择。
function toolMode(tools: string[]): ToolMode {
  if (tools.includes('none')) return 'none'
  if (tools.length > 0) return 'custom'
  return 'inherit'
}

const LAYER_GAP = 230
const ROW_GAP = 112

function StartNodeView({ data, selected }: NodeProps<GraphNode>) {
  return (
    <div className={`ag-node ag-node-start ${selected ? 'ag-selected' : ''}`}>
      <div className="ag-node-head">{data.name || '开始'}</div>
      <div className="ag-node-sub">透传用户输入</div>
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

function AgentNodeView({ data, selected }: NodeProps<GraphNode>) {
  const tools = data.tools.length > 0 ? data.tools.join(' / ') : '继承默认'
  return (
    <div className={`ag-node ag-node-agent ${selected ? 'ag-selected' : ''}`}>
      <Handle type="target" position={Position.Left} />
      <div className="ag-node-head">{data.name || '未命名 Agent'}</div>
      <div className="ag-node-sub">{data.model || '继承 Bot 模型'}</div>
      <div className="ag-node-tag">{tools}</div>
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

function EndNodeView({ data, selected }: NodeProps<GraphNode>) {
  return (
    <div className={`ag-node ag-node-end ${selected ? 'ag-selected' : ''}`}>
      <Handle type="target" position={Position.Left} />
      <div className="ag-node-head">{data.name || '结束'}</div>
      <div className="ag-node-sub">汇聚结果并回复</div>
    </div>
  )
}

const nodeTypes: NodeTypes = {
  startNode: StartNodeView,
  agentNode: AgentNodeView,
  endNode: EndNodeView,
}

// buildNode 由画布节点构造 React Flow 节点，位置缺省时由后续分层布局补齐。
function buildNode(wire: WireNode): GraphNode {
  const kind = (wire.type === 'start' || wire.type === 'end' ? wire.type : 'agent') as GraphNodeKind
  return {
    id: wire.id,
    type: NODE_KIND_RF[kind],
    position: { x: wire.x ?? 0, y: wire.y ?? 0 },
    data: {
      nodeType: kind,
      name: wire.name ?? (kind === 'start' ? '开始' : kind === 'end' ? '结束' : ''),
      systemPrompt: wire.system_prompt ?? '',
      model: wire.model ?? '',
      tools: wire.tools ?? [],
      maxSteps: wire.max_steps ?? 0,
    },
  }
}

// layerPositions 按拓扑深度做分层布局，用于历史版本或缺少坐标的图。
function layerPositions(nodes: GraphNode[], edges: { from: string; to: string }[]): GraphNode[] {
  const ids = nodes.map((n) => n.id)
  const depth = new Map<string, number>()
  ids.forEach((id) => depth.set(id, 0))
  const adj = new Map<string, string[]>()
  edges.forEach((e) => {
    const list = adj.get(e.from) ?? []
    list.push(e.to)
    adj.set(e.from, list)
  })
  const start = nodes.find((n) => n.data.nodeType === 'start')
  const queue: string[] = start ? [start.id] : [...ids]
  // 草稿里可能出现环，限制迭代次数避免死循环。
  let guard = ids.length * ids.length + ids.length + 1
  while (queue.length > 0 && guard-- > 0) {
    const cur = queue.shift() as string
    const d = depth.get(cur) ?? 0
    for (const next of adj.get(cur) ?? []) {
      if ((depth.get(next) ?? 0) < d + 1) {
        depth.set(next, d + 1)
        queue.push(next)
      }
    }
  }
  const rowInLayer = new Map<number, number>()
  return nodes.map((n) => {
    const d = depth.get(n.id) ?? 0
    const row = rowInLayer.get(d) ?? 0
    rowInLayer.set(d, row + 1)
    return { ...n, position: { x: 60 + d * LAYER_GAP, y: 60 + row * ROW_GAP } }
  })
}

// toFlow 把后端画布 JSON 还原为 React Flow 的节点与连线。
function toFlow(wire: WireGraph | null): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const wireNodes = wire?.nodes ?? []
  const wireEdges = wire?.edges ?? []
  const nodes = layerPositions(wireNodes.map(buildNode), wireEdges)
  const edges: GraphEdge[] = wireEdges.map((e) => ({
    id: `e_${e.from}_${e.to}`,
    source: e.from,
    target: e.to,
  }))
  return { nodes, edges }
}

function formatTime(ms: string): string {
  const value = Number(ms)
  if (!value) return ''
  const date = new Date(value)
  const pad = (n: number) => (n < 10 ? `0${n}` : `${n}`)
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export default function AgentGraphPanel({
  bots,
  botId,
  onSelectBot,
}: {
  bots: BotOption[]
  botId: string
  onSelectBot: (id: string) => void
}) {
  const [nodes, setNodes, onNodesChange] = useNodesState<GraphNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<GraphEdge>([])
  const [selectedId, setSelectedId] = useState('')
  const [name, setName] = useState('')
  const [status, setStatus] = useState(0)
  const [version, setVersion] = useState(0)
  const [exists, setExists] = useState(false)
  const [errors, setErrors] = useState<string[]>([])
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [history, setHistory] = useState<HistoryItem[]>([])
  const [showHistory, setShowHistory] = useState(false)
  const seqRef = useRef(1)

  const selectableBots = useMemo(() => bots.filter((b) => !b.is_system), [bots])

  const flash = (msg: string) => {
    setMessage(msg)
    window.setTimeout(() => setMessage(''), 3000)
  }

  const nextNodeId = () => {
    let id = ''
    do {
      id = `agent_${seqRef.current}`
      seqRef.current += 1
    } while (nodes.some((n) => n.id === id))
    return id
  }

  const loadGraph = useCallback(async (targetId: string) => {
    if (!targetId) return
    setBusy(true)
    const res = await api('GET', `/api/bot/agent-graph?bot_id=${targetId}`)
    setBusy(false)
    if (res.code !== 0) {
      setErrors([])
      flash(res.msg || '加载编排图失败')
      return
    }
    const graph: GraphView | undefined = res.data?.graph
    setShowHistory(false)
    setHistory([])
    if (!res.data?.exists || !graph) {
      setExists(false)
      setNodes([])
      setEdges([])
      setSelectedId('')
      setName('')
      setStatus(0)
      setVersion(0)
      setErrors([])
      return
    }
    const flow = toFlow(graph.graph)
    setNodes(flow.nodes)
    setEdges(flow.edges)
    setSelectedId('')
    setName(graph.name || '')
    setStatus(graph.status)
    setVersion(graph.version)
    setExists(true)
    setErrors([])
  }, [setNodes, setEdges])

  useEffect(() => {
    loadGraph(botId)
  }, [botId, loadGraph])

  const onConnect = useCallback(
    (connection: Connection) =>
      setEdges((eds) => addEdge({ ...connection, id: `e_${connection.source}_${connection.target}` }, eds)),
    [setEdges],
  )

  // 开始节点不接受入边，结束节点不接受出边，且禁止自连。
  const isValidConnection = useCallback(
    (connection: Connection | GraphEdge) => {
      const { source, target } = connection
      if (!source || !target || source === target) return false
      const sourceNode = nodes.find((n) => n.id === source)
      const targetNode = nodes.find((n) => n.id === target)
      if (!sourceNode || !targetNode) return false
      if (sourceNode.data.nodeType === 'end') return false
      if (targetNode.data.nodeType === 'start') return false
      return true
    },
    [nodes],
  )

  const updateNode = (id: string, patch: Partial<GraphNodeData>) => {
    setNodes((nds) => nds.map((n) => (n.id === id ? { ...n, data: { ...n.data, ...patch } } : n)))
  }

  const addAgentNode = () => {
    if (nodes.length >= 12) {
      flash('节点数已达上限 12')
      return
    }
    const id = nextNodeId()
    const index = id.split('_')[1] || `${seqRef.current}`
    const node: GraphNode = {
      id,
      type: NODE_KIND_RF.agent,
      position: { x: 290, y: 60 + nodes.length * ROW_GAP },
      data: { nodeType: 'agent', name: `Agent ${index}`, systemPrompt: '', model: '', tools: [], maxSteps: 0 },
    }
    setNodes((nds) => [...nds, node])
    setSelectedId(id)
  }

  const createDefaultGraph = () => {
    const start: GraphNode = {
      id: 'start',
      type: NODE_KIND_RF.start,
      position: { x: 60, y: 60 },
      data: { nodeType: 'start', name: '开始', systemPrompt: '', model: '', tools: [], maxSteps: 0 },
    }
    const end: GraphNode = {
      id: 'end',
      type: NODE_KIND_RF.end,
      position: { x: 520, y: 60 },
      data: { nodeType: 'end', name: '结束', systemPrompt: '', model: '', tools: [], maxSteps: 0 },
    }
    setNodes([start, end])
    setEdges([{ id: 'e_start_end', source: 'start', target: 'end' }])
    setSelectedId('')
  }

  const deleteNode = (id: string) => {
    if (id === 'start') {
      flash('开始节点不可删除')
      return
    }
    setNodes((nds) => nds.filter((n) => n.id !== id))
    setEdges((eds) => eds.filter((e) => e.source !== id && e.target !== id))
    if (selectedId === id) setSelectedId('')
  }

  // buildWire 序列化为后端 schema；x/y 仅用于画布回显，后端透传不参与校验。
  const buildWire = () => {
    const wireNodes: WireNode[] = nodes.map((n) => {
      const item: WireNode = {
        id: n.id,
        type: n.data.nodeType,
        name: n.data.name,
        x: Math.round(n.position.x),
        y: Math.round(n.position.y),
      }
      if (n.data.nodeType !== 'start') {
        item.system_prompt = n.data.systemPrompt
        item.model = n.data.model
      }
      if (n.data.nodeType === 'agent') {
        item.tools = n.data.tools
        item.max_steps = n.data.maxSteps
      }
      return item
    })
    return JSON.stringify({ nodes: wireNodes, edges: edges.map((e) => ({ from: e.source, to: e.target })) })
  }

  const handleValidate = async () => {
    if (!botId) {
      flash('请先选择 Bot')
      return
    }
    setBusy(true)
    const res = await api('POST', '/api/bot/agent-graph/validate', { bot_id: botId, graph_json: buildWire() })
    setBusy(false)
    if (res.code !== 0) {
      flash(res.msg || '校验失败')
      return
    }
    const list: string[] = res.data?.errors ?? []
    setErrors(list)
    flash(res.data?.valid ? '校验通过' : `校验未通过，共 ${list.length} 项问题`)
  }

  const handleSave = async () => {
    if (!botId) {
      flash('请先选择 Bot')
      return
    }
    setBusy(true)
    const res = await api('POST', '/api/bot/agent-graph/save', {
      bot_id: botId,
      name: name || '未命名编排',
      graph_json: buildWire(),
    })
    setBusy(false)
    if (res.code !== 0) {
      flash(res.msg || '保存失败')
      return
    }
    if (!res.data?.success) {
      setErrors(res.data?.errors ?? [])
      flash('编排图无法保存，请检查内容')
      return
    }
    setErrors(res.data?.errors ?? [])
    setVersion(res.data?.version ?? version)
    setStatus(res.data?.status ?? 0)
    setExists(true)
    flash('已保存为草稿，发布后生效')
  }

  const handlePublish = async () => {
    if (!botId || !exists) {
      flash('请先保存编排图')
      return
    }
    setBusy(true)
    const res = await api('POST', '/api/bot/agent-graph/publish', { bot_id: botId })
    setBusy(false)
    if (res.code !== 0 || !res.data?.success) {
      setErrors(res.data?.errors ?? [])
      flash(res.data?.errors?.length ? '校验未通过，无法发布' : res.msg || '发布失败')
      return
    }
    setStatus(1)
    setErrors([])
    flash('已发布，该 Bot 的对话将按编排执行')
  }

  const loadHistory = async () => {
    if (!botId) return
    setBusy(true)
    const res = await api('GET', `/api/bot/agent-graph/history?bot_id=${botId}`)
    setBusy(false)
    if (res.code !== 0) {
      flash(res.msg || '加载历史失败')
      return
    }
    setHistory(res.data?.items ?? [])
    setShowHistory(true)
  }

  const rollback = (item: HistoryItem) => {
    const flow = toFlow(item.graph)
    setNodes(flow.nodes)
    setEdges(flow.edges)
    setSelectedId('')
    flash(`已载入 v${item.version}，保存后生效`)
  }

  const selected = nodes.find((n) => n.id === selectedId)

  if (selectableBots.length === 0) {
    return (
      <div className="ag-panel">
        <div className="contacts-empty" style={{ padding: '24px 0' }}>还没有可编排的 Bot，先到「创建 Bot」新建一个</div>
      </div>
    )
  }

  return (
    <div className="ag-panel">
      {message && <div className="bots-toast">{message}</div>}

      <div className="ag-toolbar">
        <select
          className="ag-select"
          value={botId}
          onChange={(e) => onSelectBot(e.target.value)}
        >
          <option value="">-- 请选择 Bot --</option>
          {selectableBots.map((b) => (
            <option key={b.bot_id} value={b.bot_id}>{b.name}</option>
          ))}
        </select>
        {exists && (
          <span className={`ag-status ag-status-${status}`}>{STATUS_LABELS[status] ?? '未知'} · v{version}</span>
        )}
      </div>

      {!botId ? (
        <div className="contacts-empty" style={{ padding: '24px 0' }}>选择一个 Bot 后开始编排</div>
      ) : (
        <>
          <div className="ag-actions">
            <button className="ag-btn" onClick={addAgentNode} disabled={busy}>+ Agent</button>
            <button className="ag-btn" onClick={createDefaultGraph} disabled={busy}>默认图</button>
            <button className="ag-btn" onClick={handleValidate} disabled={busy}>校验</button>
            <button className="ag-btn primary" onClick={handleSave} disabled={busy}>保存草稿</button>
            <button className="ag-btn primary" onClick={handlePublish} disabled={busy}>发布</button>
            <button className="ag-btn" onClick={loadHistory} disabled={busy}>历史</button>
          </div>

          {errors.length > 0 && (
            <div className="ag-errors">
              {errors.map((err, idx) => <div key={idx}>· {err}</div>)}
            </div>
          )}

          <div className="ag-workbench">
            <div className="ag-canvas">
              {nodes.length === 0 ? (
                <div className="ag-canvas-empty">
                  <div>画布还没有节点</div>
                  <button className="ag-btn primary" onClick={createDefaultGraph}>创建默认编排</button>
                </div>
              ) : (
                <ReactFlow
                  nodes={nodes}
                  edges={edges}
                  nodeTypes={nodeTypes}
                  onNodesChange={onNodesChange}
                  onEdgesChange={onEdgesChange}
                  onConnect={onConnect}
                  isValidConnection={isValidConnection}
                  onNodeClick={(_, node) => setSelectedId(node.id)}
                  onPaneClick={() => setSelectedId('')}
                  defaultEdgeOptions={{ markerEnd: { type: MarkerType.ArrowClosed } }}
                  fitView
                  proOptions={{ hideAttribution: true }}
                >
                  <Background />
                  <Controls showInteractive={false} />
                </ReactFlow>
              )}
            </div>

            <div className="ag-config">
              {selected ? (
                <>
                  <div className="ag-config-head">
                    <span>{NODE_KIND_LABEL[selected.data.nodeType]}节点</span>
                    {selected.data.nodeType !== 'start' && (
                      <button className="ag-link danger" onClick={() => deleteNode(selected.id)}>删除</button>
                    )}
                  </div>
                  <label className="ag-field">
                    <span>名称</span>
                    <input
                      value={selected.data.name}
                      onChange={(e) => updateNode(selected.id, { name: e.target.value })}
                      placeholder="节点名称"
                    />
                  </label>
                  {selected.data.nodeType !== 'start' && (
                    <>
                      <label className="ag-field">
                        <span>系统提示词</span>
                        <textarea
                          rows={5}
                          value={selected.data.systemPrompt}
                          onChange={(e) => updateNode(selected.id, { systemPrompt: e.target.value })}
                          placeholder="该节点的角色与任务"
                        />
                      </label>
                      <label className="ag-field">
                        <span>模型</span>
                        <input
                          value={selected.data.model}
                          onChange={(e) => updateNode(selected.id, { model: e.target.value })}
                          placeholder="留空继承 Bot 模型"
                        />
                      </label>
                    </>
                  )}
                  {selected.data.nodeType === 'agent' && (
                    <>
                      <div className="ag-field">
                        <span>工具</span>
                        <select
                          value={toolMode(selected.data.tools)}
                          onChange={(e) => {
                            const mode = e.target.value as ToolMode
                            if (mode === 'inherit') {
                              updateNode(selected.id, { tools: [] })
                              return
                            }
                            if (mode === 'none') {
                              updateNode(selected.id, { tools: ['none'] })
                              return
                            }
                            const picked = selected.data.tools.filter((t) => t !== 'none')
                            updateNode(selected.id, { tools: picked.length > 0 ? picked : ['knowledge'] })
                          }}
                        >
                          <option value="inherit">继承 Bot 默认工具</option>
                          <option value="none">禁用全部工具</option>
                          <option value="custom">自定义</option>
                        </select>
                        {toolMode(selected.data.tools) === 'custom' &&
                          TOOL_OPTIONS.map((opt) => {
                            const picked = selected.data.tools.filter((t) => t !== 'none')
                            return (
                              <label className="ag-check" key={opt.value}>
                                <input
                                  type="checkbox"
                                  checked={picked.includes(opt.value)}
                                  onChange={() => {
                                    const next = picked.includes(opt.value)
                                      ? picked.filter((t) => t !== opt.value)
                                      : [...picked, opt.value]
                                    updateNode(selected.id, { tools: next.length > 0 ? next : ['none'] })
                                  }}
                                />
                                {opt.label}
                              </label>
                            )
                          })}
                      </div>
                      <label className="ag-field">
                        <span>最大步数（0 表示默认，上限 6）</span>
                        <input
                          type="number"
                          min={0}
                          max={6}
                          value={selected.data.maxSteps}
                          onChange={(e) => {
                            const value = Number(e.target.value)
                            updateNode(selected.id, { maxSteps: Number.isFinite(value) ? Math.min(6, Math.max(0, value)) : 0 })
                          }}
                        />
                      </label>
                    </>
                  )}
                  <label className="ag-field">
                    <span>节点 ID</span>
                    <input value={selected.id} readOnly />
                  </label>
                </>
              ) : (
                <div className="ag-config-empty">点选一个节点编辑配置，拖动右侧圆点连线</div>
              )}
            </div>
          </div>

          {showHistory && (
            <div className="ag-history">
              <div className="ag-history-head">
                <span>历史版本</span>
                <button className="ag-link" onClick={() => setShowHistory(false)}>收起</button>
              </div>
              {history.length === 0 ? (
                <div className="ag-config-empty">暂无历史版本</div>
              ) : (
                history.map((item) => (
                  <div className="ag-history-row" key={item.id}>
                    <span className="ag-history-version">v{item.version}</span>
                    <span className="ag-history-time">{formatTime(item.created_at)}</span>
                    <button className="ag-link" onClick={() => rollback(item)}>载入</button>
                  </div>
                ))
              )}
            </div>
          )}
        </>
      )}
    </div>
  )
}
