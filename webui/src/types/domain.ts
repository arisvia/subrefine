// Shared UI types. The proxy/pipeline shapes mirror the Go JSON emitted by
// internal/core/model.ProxyNode and internal/storage.Subscription.

export type BadgeTone =
  | 'green' | 'amber' | 'red' | 'gray' | 'blue' | 'purple'
  | 'cyan' | 'pink' | 'violet' | 'teal' | 'rose' | 'orange' | 'indigo'

/** Canonical protocol ids produced by internal/core/model. */
export type Protocol =
  | 'shadowsocks' | 'vmess' | 'vless' | 'trojan' | 'hysteria2' | 'wireguard'

export type PipelineType = 'deduplicate' | 'filter' | 'rename' | 'decorate' | 'sort'

export interface ProxyNode {
  tag: string
  type: Protocol
  server: string
  port: number
  password?: string
  uuid?: string
  method?: string
  network?: string
  path?: string
  host?: string
  headers?: Record<string, string>
  service_name?: string
  security?: string
  server_name?: string
  insecure?: boolean
  alpn?: string[]
  fingerprint?: string
  public_key?: string
  short_id?: string
  spider_x?: string
  flow?: string
  up_mbps?: number
  down_mbps?: number
  obfs_type?: string
  obfs_pass?: string
  private_key?: string
  peer_public_key?: string
  pre_shared_key?: string
  local_addr?: string[]
  mtu?: number
  reserved?: number[]
  detour?: string
  extra?: Record<string, string>
}

export interface PipelineStep {
  type: PipelineType
  pattern?: string
  repl?: string
  include?: string
  exclude?: string
  types?: Protocol[]
  fingerprint?: string
  detour?: string
  by?: 'tag' | 'protocol'
  order?: 'asc' | 'desc'
}

/** One delivery credential: its own link plus an optional node blacklist. */
export interface SubKey {
  id: string
  name: string
  /** Empty means "generate on save" (server-side crypto/rand). */
  token: string
  /** ProxyNode identity hashes withheld from this key's deliveries. */
  disabled?: string[]
}

export interface Subscription {
  id: string
  name: string
  token: string
  /** Delivery keys; the legacy single token is keys[0]. */
  keys?: SubKey[]
  sources: string[]
  pipeline: PipelineStep[]
  enabled: boolean
  /** Full-config template mode names; empty/absent = fragment delivery. */
  singbox_mode?: string
  clash_mode?: string
  created_at: number
  updated_at: number
}

export interface TemplateModes {
  singbox: string[]
  clash: string[]
}

export interface NodePreview {
  id: string
  count: number
  nodes: ProxyNode[]
  /** Wire-optional (older builds omit it), index-aligned with nodes via the store. */
  uris?: string[]
  /** Stable node identities, index-aligned with nodes; keys blacklist these. */
  ids?: string[]
}

export interface AdminInfo {
  status: string
  version: string
  host: string
  port: number
}

/** Shorthand spellings users type; resolved server-side by the pipeline alias table. */
export const PROTOCOLS: Protocol[] = ['shadowsocks', 'vmess', 'vless', 'trojan', 'hysteria2', 'wireguard']

export const PROTOCOL_LABELS: Record<Protocol, string> = {
  shadowsocks: 'Shadowsocks',
  vmess: 'VMess',
  vless: 'VLESS',
  trojan: 'Trojan',
  hysteria2: 'Hysteria2',
  wireguard: 'WireGuard',
}

export const PROTOCOL_TONES: Record<Protocol, BadgeTone> = {
  shadowsocks: 'blue',
  vmess: 'violet',
  vless: 'indigo',
  trojan: 'amber',
  hysteria2: 'rose',
  wireguard: 'teal',
}

export const PIPELINE_TONES: Record<PipelineType, BadgeTone> = {
  deduplicate: 'cyan',
  filter: 'green',
  rename: 'purple',
  decorate: 'orange',
  sort: 'gray',
}

/** uTLS fingerprints offered by the decorate operator (sing-box / mihomo set). */
export const FINGERPRINTS = ['chrome', 'firefox', 'safari', 'edge', 'ios', 'android']
