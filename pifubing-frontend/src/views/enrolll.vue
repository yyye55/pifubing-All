<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { ElMessage } from 'element-plus';
import AppNavbar from '../components/AppNavbar.vue';

// 响应式数据
const account = ref('');
const password = ref('');
const confirmPassword = ref('');
const email = ref('');
const verificationCode = ref('');
const sendingCode = ref(false);
const showPassword = ref(false); // 密码可见性切换
const showConfirmPassword = ref(false); // 确认密码可见性切换
const router = useRouter();

// 发送验证码
const handleSendVerificationCode = async () => {
  if (!email.value) {
    ElMessage.error('请输入有效的邮箱地址');
    return;
  }

  sendingCode.value = true;

  try {
    const response = await axios.post(`/send_email`, {
      to: email.value,
    });

    if (response.data.code === 200) {
      ElMessage.success('验证码已发送，请检查您的邮箱');
    } else {
      ElMessage.error(response.data.message || '发送验证码失败');
    }
  } catch (error) {
    console.error('验证码发送错误:', error);
    ElMessage.error('发送验证码时出现错误');
  } finally {
    sendingCode.value = false;
  }
};

// 提交注册表单
const handleSubmit = async () => {
  if (!account.value || !password.value || !confirmPassword.value || !email.value || !verificationCode.value) {
    ElMessage.error('请完整填写所有必填字段');
    return;
  }

  if (password.value !== confirmPassword.value) {
    ElMessage.error('两次输入的密码不一致');
    return;
  }

  try {
    const response = await axios.post(`/users/register`, {
      account: account.value,
      password: password.value,
      email: email.value,
      code: verificationCode.value,
      // 姓名/性别/年龄/手机号注册时不再填写，先给后端占位值，
      // 用户可以在「个人中心」里补充完整。
      name: '',
      gender: '',
      age: 0,
      phone_number: '',
    });

    if (response.data.code === 200) {
      ElMessage.success('注册成功，正在跳转登录...');
      setTimeout(() => {
        router.push('/HomeView');
      }, 1200);
    } else {
      ElMessage.error(response.data.message || '注册失败，请重试');
    }
  } catch (error) {
    console.error('注册请求错误:', error);
    ElMessage.error('注册时发生错误，请稍后重试');
  }
};
</script>

<template>
  <div class="auth-page">
    <AppNavbar />

    <main class="auth-main auth-main--single auth-main--register">
      <!-- 注册卡片：简化为单栏居中，去掉左侧品牌区与卡片头图标 -->
      <section class="auth-card auth-card--wide form-box login">
        <div class="auth-card-head">
          <h2>创建账号</h2>
        </div>

        <form @submit.prevent="handleSubmit">
          <div class="auth-grid">
            <div class="input-box">
              <label>账号</label>
              <div class="input-field">
                <input type="text" v-model="account" required autocomplete="username" />
              </div>
            </div>

            <div class="input-box">
              <label>密码</label>
              <div class="input-field">
                <input
                  :type="showPassword ? 'text' : 'password'"
                  v-model="password"
                  required
                  autocomplete="new-password"
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
            </div>

            <div class="input-box">
              <label>确认密码</label>
              <div class="input-field">
                <input
                  :type="showConfirmPassword ? 'text' : 'password'"
                  v-model="confirmPassword"
                  required
                  autocomplete="new-password"
                />
                <button
                  type="button"
                  class="eye-toggle"
                  :aria-label="showConfirmPassword ? '隐藏密码' : '显示密码'"
                  @click="showConfirmPassword = !showConfirmPassword"
                >
                  <ion-icon :name="showConfirmPassword ? 'eye-off-outline' : 'eye-outline'"></ion-icon>
                </button>
              </div>
            </div>

            <div class="input-box">
              <label>邮箱</label>
              <div class="input-field">
                <input type="email" v-model="email" required autocomplete="email" />
                <button
                  type="button"
                  class="btn-verify"
                  :disabled="sendingCode"
                  @click="handleSendVerificationCode"
                >
                  {{ sendingCode ? '发送中...' : '发送验证码' }}
                </button>
              </div>
            </div>

            <div class="input-box">
              <label>验证码</label>
              <div class="input-field">
                <input type="text" v-model="verificationCode" required inputmode="numeric" />
              </div>
            </div>
          </div>

          <button type="submit" class="btn">注册</button>

          <div class="auth-links auth-links--left">
            <span class="auth-links-text">已经有账号？</span>
            <RouterLink to="/HomeView">直接登录</RouterLink>
          </div>
        </form>

        <p class="auth-legal">注册即代表你同意服务条款与隐私政策。AI 建议不替代线下医生诊断。</p>
      </section>
    </main>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
}

/* 注册页表单较长，卡片顶部对齐而不是垂直居中 */
.auth-main--register {
  align-content: start;
  padding-top: clamp(24px, 4vh, 48px);
}
</style>
