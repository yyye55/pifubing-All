<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';

// 响应式数据
const account = ref('');
const password = ref('');
const formError = ref(''); // 表单错误提示
const alertMessage = ref('');
const showAlert = ref(false);
const loading = ref(false); // 控制加载状态

const router = useRouter();

// 显示弹框的函数
const showAlertBox2 = (message) => {
  alertMessage.value = message;
  showAlert.value = true;
  setTimeout(() => {
    showAlert.value = false; // 3秒后自动隐藏弹框
  }, 3000);
};

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
console.log(response.data);
        if (response.data.code === 200) {
            const token = response.data.token;
            const user_id = response.data.user_id;
            localStorage.setItem('token', token);
            localStorage.setItem('user_id', user_id);
            console.log('Token 存储成功:', token); // 添加调试信息
            console.log('user_id 存储成功:', user_id);
            showAlertBox2('登录成功，正在跳转...');
            loadUserData(token); // 获取用户数据
            setTimeout(() => {
                router.push('/doctor'); // 跳转到目标页面
            }, 2000); // 2秒延迟
        } else {
            showAlertBox2('请检查你的账号或密码是否正确');
        }
    } catch (error) {
        console.error(error);
        showAlertBox2('登录失败，请检查你的网络连接');
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
    <header class="auth-topbar">
      <RouterLink to="/" class="auth-logo">
        <img src="../assets/t.png" alt="SkinCare AI" />
        <span>SkinCare AI</span>
      </RouterLink>
      <RouterLink to="/enrolll" class="auth-top-link">还没有账号？立即注册</RouterLink>
    </header>

    <main class="auth-main">
      <!-- 左侧品牌区：先建立“AI 医疗顾问”的信任感，再让用户登录 -->
      <section class="auth-brand-panel">
        <span class="ai-eyebrow"><span class="ai-pulse"></span> AI 皮肤健康顾问</span>
        <h1>让每一次皮肤困扰<br><em>都被认真看见</em></h1>
        <p class="auth-lead">
          上传患处照片或描述症状，AI 会结合皮肤科知识库给出初步分析与护理建议。
          结果仅作健康参考，帮助你更从容地决定下一步。
        </p>

        <ul class="auth-points">
          <li>
            <span class="point-icon"><ion-icon name="scan-outline"></ion-icon></span>
            <span><strong>图像 + 文字双通道分析</strong>支持患处照片、症状描述与追问</span>
          </li>
          <li>
            <span class="point-icon"><ion-icon name="library-outline"></ion-icon></span>
            <span><strong>皮肤科知识库检索</strong>15 类常见皮肤病，回答有依据</span>
          </li>
          <li>
            <span class="point-icon"><ion-icon name="shield-checkmark-outline"></ion-icon></span>
            <span><strong>隐私优先</strong>账号隔离，图片仅用于本次分析</span>
          </li>
        </ul>

        <div class="auth-stat-row">
          <div class="auth-stat"><strong>15 类</strong><span>常见皮肤病覆盖</span></div>
          <div class="auth-stat"><strong>秒级</strong><span>图像初步分析</span></div>
          <div class="auth-stat"><strong>7×24</strong><span>随时可问</span></div>
        </div>
      </section>

      <!-- 右侧登录卡片：原有 v-model、handleLogin、loading 全部保持不变 -->
      <section class="auth-card form-box login">
        <div class="auth-card-head">
          <span class="auth-icon"><ion-icon name="medkit-outline"></ion-icon></span>
          <h2>欢迎回来</h2>
          <p>登录后继续你的皮肤健康档案</p>
        </div>

        <form @submit.prevent="handleLogin">
          <div class="input-box">
            <span class="icon"><ion-icon name="person-outline"></ion-icon></span>
            <input type="text" v-model="account" required autocomplete="username" />
            <label>账号</label>
          </div>

          <div class="input-box">
            <span class="icon"><ion-icon name="lock-closed-outline"></ion-icon></span>
            <input type="password" v-model="password" required autocomplete="current-password" />
            <label>密码</label>
          </div>

          <p v-if="formError" class="error">{{ formError }}</p>

          <button type="submit" class="btn" :disabled="loading">
            {{ loading ? '登录中...' : '登录' }}
          </button>

          <div class="remember-forgot">
            <RouterLink to="/Forgot password">忘记密码？</RouterLink>
          </div>

          <div class="login-register">
            <p>还没有账号？<RouterLink to="/enrolll">免费注册</RouterLink></p>
          </div>
        </form>

        <p class="auth-legal">登录即代表你同意服务条款与隐私政策。AI 建议不替代线下医生诊断。</p>
      </section>
    </main>

    <div v-if="showAlert" class="alert-box ai-toast">{{ alertMessage }}</div>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
}

.auth-topbar {
  padding-top: 22px;
  padding-bottom: 10px;
}

.auth-brand-panel h1 {
  margin-top: 20px;
}

.auth-card-head h2 {
  letter-spacing: -0.03em;
}

@media (max-width: 960px) {
  .auth-brand-panel h1 {
    margin-top: 16px;
  }
}
</style>
