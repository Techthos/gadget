// Protocol constants and types for the MCP Apps extension
// (io.modelcontextprotocol/ui), spec 2026-01-26. Method names live in
// spec-constants.json, shared with the Go side (package uispec cross-checks).
import spec from "./spec-constants.json";

export const SPEC_VERSION: string = spec.specVersion;
export const M = spec.methods;

export const RUNTIME_NAME = "gadget";
export const RUNTIME_VERSION = "0.1.0";

// --- JSON-RPC 2.0 ---

export type RequestID = number | string;

export interface JsonRpcError {
  code: number;
  message: string;
  data?: unknown;
}

export interface JsonRpcMessage {
  jsonrpc: "2.0";
  id?: RequestID;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: JsonRpcError;
}

export const METHOD_NOT_FOUND = -32601;

// --- MCP Apps shapes ---

export interface ContentBlock {
  type: string;
  [key: string]: unknown;
}

export interface CallToolResult {
  content?: ContentBlock[];
  structuredContent?: Record<string, unknown>;
  isError?: boolean;
  _meta?: Record<string, unknown>;
}

export type HostTheme = "light" | "dark";

export interface HostStyles {
  variables?: Record<string, string>;
  css?: { fonts?: string };
}

export interface HostContext {
  theme?: HostTheme;
  styles?: HostStyles;
  displayMode?: string;
  availableDisplayModes?: string[];
  containerDimensions?: Record<string, number>;
  locale?: string;
  timeZone?: string;
  platform?: "web" | "desktop" | "mobile";
  [key: string]: unknown;
}

export interface InitializeResult {
  protocolVersion?: string;
  hostInfo?: unknown;
  hostCapabilities?: unknown;
  hostContext?: HostContext;
}

// Methods a view accepts from the host. The bridge ignores incoming traffic
// outside this set: it keeps direction discipline per spec and makes the
// bridge safe in same-window test environments where its own outgoing
// messages echo back.
export const HOST_TO_VIEW_METHODS: ReadonlySet<string> = new Set([
  M.toolInput,
  M.toolInputPartial,
  M.toolResult,
  M.toolCancelled,
  M.hostContextChanged,
  M.resourceTeardown,
  M.ping,
  M.sandboxProxyReady,
  M.sandboxResourceReady,
]);
