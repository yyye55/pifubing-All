<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { ElMessage } from 'element-plus';
import AppNavbar from '../components/AppNavbar.vue';

// 响应式数据
const account = ref('');
const password = ref('');
const formError = ref(''); // 表单错误提示（内联，不走 ElMessage）
const loading = ref(false); // 控制加载状态
const showPassword = ref(false); // 密码可见性切换

const router = useRouter();

// 表单验证和登录请求
const handleLogin = async () => {
    if (!account.value) {
        formError.value = '请输入账号';
        return;
    }
    if (!password.value) {
        formError.value = '请输入密码';
        return;
    }
    formError.value = ''; // 清空错误消息
    loading.value = true; // 开始加载
    try {
        const response = await axios.post('/users/login', {
            account: account.value,
            password: password.value,
            headers: {
                'Content-Type': 'application/json',
            },
        });
        if (response.data.code === 200) {
            const token = response.data.token;
            const user_id = response.data.user_id;
            localStorage.setItem('token', token);
            localStorage.setItem('user_id', user_id);
            console.log('Token 存储成功:', token);
            console.log('user_id 存储成功:', user_id);
            ElMessage.success('登录成功，正在跳转...');
            loadUserData(token); // 获取用户数据
            // 若因未登录被拦截过，登录后回到原来想访问的页面，否则进入登录后首页
            const redirectPath = router.currentRoute.value.query.redirect;
            const target =
                typeof redirectPath === 'string' && redirectPath.startsWith('/') ? redirectPath : '/Picture';
            setTimeout(() => {
                router.push(target);
            }, 1200);
        } else {
            ElMessage.error(response.data.error || response.data.message || '请检查你的账号或密码是否正确');
        }
    } catch (error) {
        console.error(error);
        // 后端校验失败返回 401，axios 会进入 catch；
        // 优先展示后端给出的账号/密码错误提示，只有无响应时才提示网络问题。
        const data = error.response?.data;
        if (data) {
            ElMessage.error(data.error || data.message || '登录失败，请检查你的账号或密码');
        } else {
            ElMessage.error('登录失败，请检查你的网络连接');
        }
    } finally {
        loading.value = false; // 结束加载
    }
};

const avatarUrl = ref('');
// 表单数据
const formData = ref({
  account: '',
  email: '',
  name: '',
  age: '',
  gender: '',
  phone_number: '',  // 新增字段
  medicalHistory: '',  // 用于保存病史
  allergyHistory: ''   // 用于保存过敏史
});


// 用户查找所有个人信息
const loadUserData = async (token) => {  // renamed function
  try {
    const response = await axios.get(
      '/users/check_users_information',
      {
        headers: {
          Authorization: `Bearer ${token}`  // 将 Bearer token 包含在请求头中
        }
      }
    );

    if (response.data.code === 200) {
      const data = response.data.data;

      // 提供默认值以避免空值
      formData.value = {
        account: data.account || '暂无账号',
        email: data.email || '暂无邮箱',
        name: data.name || '',
        age: data.age || '',
        gender: data.gender || '',
        phone_number: data.phone_number || '',  // 新增字段
        medicalHistory: data.medical_history || '',
        allergyHistory: data.allergies || ''
      };

      // 更新头像 URL
      avatarUrl.value = data.avatar_url || 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png';
      console.log('请求数据成功', data.avatar_url);
      localStorage.setItem('avatarUrl', avatarUrl.value); // 更新头像 URL
      // // 更新背景图片
      // backgroundImage.value = 'https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRDijTs7b1Zmz0DHwcebyQOVdGXfB8eZVDYcg&s';
      //backgroundImage.value = '../assets/登.png';
    }
  } catch (error) {
    console.error('请求数据失败', error);
  }
};



</script>

<template>
  <div class="auth-page">
    <AppNavbar />

    <main class="auth-main auth-main--single">
      <!-- 登录卡片：简化为单栏居中，去掉左侧品牌区与卡片头图标 -->
      <section class="auth-card form-box login">
        <div class="auth-card-head">
          <h2>欢迎回来</h2>
          <p>登录后继续你的皮肤健康档案</p>
        </div>

        <form @submit.prevent="handleLogin">
          <div class="input-box">
            <label>账号</label>
            <input type="text" v-model="account" required autocomplete="username" />
          </div>

          <div class="input-box">
            <label>密码</label>
            <input
              :type="showPassword ? 'text' : 'password'"
              v-model="password"
              required
              autocomplete="current-password"
            />
            <button
              type="button"
              class="eye-toggle"
              :aria-label="showPassword ? '隐藏密码' : '显示密码'"
              @click="showPassword = !showPassword"
            >
              <ion-icon :name="showPassword ? 'eye-off-outline' : 'eye-outline'"></ion-icon>
            </button>
          </div>

          <p v-if="formError" class="error">{{ formError }}</p>

          <button type="submit" class="btn" :disabled="loading">
            {{ loading ? '登录中...' : '登录' }}
          </button>

          <div class="auth-links">
            <RouterLink to="/Forgot password">忘记密码？</RouterLink>
            <RouterLink to="/enrolll">注册</RouterLink>
          </div>
        </form>

        <p class="auth-legal">登录即代表你同意服务条款与隐私政策。AI 建议不替代线下医生诊断。</p>
      </section>
    </main>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
}
</style>
