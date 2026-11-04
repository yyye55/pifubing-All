import './assets/main.css'

import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css'; // 引入 Element Plus 的样式文件
import './assets/theme.css' // 医疗顾问视觉系统：必须放在 Element Plus 之后，才能统一覆盖组件样式
import axios from 'axios'
import { API_BASE } from './apiBase'

// 所有相对路径的 axios 请求都会自动拼上后端地址。
// 这样页面里只写 /users/login 这类路径，部署时改环境变量即可。
axios.defaults.baseURL = API_BASE

const app = createApp(App)

app.use(router)
app.use(ElementPlus);
app.mount('#app')
