import axios from 'axios'

// 配置 VITE_UNION_API_BASE 时，union 请求走独立的 union-svc 外挂服务。
// 未配置时 fallback 到主站后端（VITE_API_BASE），保持向后兼容。
// 与主站 client.ts 的区别：不包含 401 自动刷新逻辑（union-svc 用 OAuth2 token）。
const unionClient = axios.create({
  baseURL: import.meta.env.VITE_UNION_API_BASE || import.meta.env.VITE_API_BASE || '',
  withCredentials: true,
})

export default unionClient
