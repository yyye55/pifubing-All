import { createRouter, createWebHashHistory } from 'vue-router';
import { ElMessage } from 'element-plus';
import Homene from '../views/Homene.vue';
import Picture from '../views/Picture.vue';

// 需要登录才能访问的路由路径（白名单）
const REQUIRES_AUTH_PATHS = ['/home', '/Summarize', '/doctor'];

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    //平台介绍（未登录用户打开前端默认进入的介绍页）
    {
      path: '/',
      name: '平台介绍',
      component: Homene,
    },

    //登录
    {
      path: '/HomeView',
      name: '登录',
      component: () => import('../views/HomeView.vue'),
    },

    //注册
    {
      path: '/enrolll',
      name: '注册',
      component: () => import('../views/enrolll.vue'),
    },
    //首页（AI 问诊）
    {
      path: '/home',
      name: '首页',
      component: () => import('../views/home.vue'),
      meta: { requiresAuth: true },
    },
    //忘记密码
    {
      path: '/Forgot password',
      name: '忘记密码',
      component: () => import('../views/Forgot password.vue'),
    },
    //图片廊（首页）
    {
      path: '/Picture',
      name: '图片廊',
      component: Picture,
    },

    {
      path: '/Summarize',
      name: '个人账户',
      component: () => import('../views/Summarize.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/care',
      name: '关怀模式',
      component: () => import('../views/care.vue'),
    },

    {
      path: '/doctor',
      name: '病例',
      component: () => import('../views/doctor.vue'),
      meta: { requiresAuth: true },
    },

    {
      path: '/AboutUs',
      name: '关于我们',
      component: () => import('../views/AboutUs.vue'),
    },
  ],
});

// 全局路由守卫：未登录访问受保护路由 → ElMessage 提示 + 跳登录页
router.beforeEach((to, _from, next) => {
  const token = localStorage.getItem('token');
  const needsAuth = to.meta?.requiresAuth || REQUIRES_AUTH_PATHS.includes(to.path);

  if (needsAuth && !token) {
    ElMessage.warning('请先登录后再访问此页面');
    next({ path: '/HomeView', query: { redirect: to.fullPath } });
    return;
  }
  next();
});

export default router;
