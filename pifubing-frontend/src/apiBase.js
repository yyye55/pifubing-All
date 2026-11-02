// 后端接口基址统一从这里读取，避免在每个页面里写死服务器地址。
//
// 本地开发在 .env.development 里配置 VITE_API_BASE=http://localhost:9094；
// 部署时在构建环境变量里配置成线上后端地址即可，不需要改代码。
export const API_BASE = import.meta.env.VITE_API_BASE || 'http://localhost:9094'
