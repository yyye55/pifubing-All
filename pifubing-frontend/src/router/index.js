import { createRouter, createWebHashHistory } from 'vue-router'
import Homene from '../views/Homene.vue';
import Picture from '../views/Picture.vue';

const router = createRouter({
  history: createWebHashHistory(import.meta.env.BASE_URL),
  routes: [
    //概览
   {
    path:'/',
    name:'home',
    component:Homene,
   },

   //登录
    {
      path: '/HomeView',
      name: '登录',
      component:()=>import('../views/HomeView.vue')
    },
   
   //注册
   {
    path:'/enrolll',
    name:'注册',
    component:()=>import('../views/enrolll.vue')
  },
  //首页
  {
    path:'/home',
    name:'首页',
    component:()=>import('../views/home.vue')
  },
  //忘记密码
  {
    path:'/Forgot password',
    name:'忘记密码',
    component:()=>import('../views/Forgot password.vue')
  },
  //图片廊
  {
    path:'/Picture',
    name:'图片廊',
    component:()=>import('../views/Picture.vue')
  },

  {
    path:'/Summarize',
    name:'个人账户',
    component:()=>import('../views/Summarize.vue')
  },
  {
    path:'/care',
    name:'关怀模式',
    component:()=>import('../views/care.vue')
  },

  {
    path:'/doctor',
    name:'病例',
    component:()=>import('../views/doctor.vue')
  },

  {
    path:'/AboutUs',
    name:'关于我们',
    component:()=>import('../views/AboutUs.vue')
  }


 
  ],
})

export default router
