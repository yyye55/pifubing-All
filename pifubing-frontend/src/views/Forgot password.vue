<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import { ElMessage } from 'element-plus';
import AppNavbar from '../components/AppNavbar.vue';

// 定义响应式数据
const account = ref('');
const password = ref('');
const confirmPassword = ref('');
const email = ref('');
const verificationCode = ref('');
const sendingCode = ref(false);
const passwordError = ref('');
const showPassword = ref(false); // 新密码可见性切换
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

// 验证密码一致性
const validatePasswords = () => {
  if (password.value && confirmPassword.value && password.value !== confirmPassword.value) {
    passwordError.value = '两次输入的密码不一致';
  } else {
    passwordError.value = '';
  }
};

// 表单提交
const handleSubmit = async () => {
  // 验证表单数据完整性
  if (!account.value) {
    ElMessage.error('请输入账号');
    return;
  }
  if (!email.value) {
    ElMessage.error('请输入邮箱地址');
    return;
  }
  if (!verificationCode.value) {
    ElMessage.error('请输入验证码');
    return;
  }
  if (!password.value || !confirmPassword.value) {
    ElMessage.error('请填写密码和确认密码');
    return;
  }
  if (passwordError.value) {
    ElMessage.error(passwordError.value);
    return;
  }

  try {
    const response = await axios.post(`/users/forget/password`, {
      account: account.value,
      password: password.value,
      email: email.value,
      code: verificationCode.value,
    });

    if (response.data.code === 200) {
      ElMessage.success('密码重置成功，正在跳转登录...');
      setTimeout(() => {
        router.push('/HomeView');
      }, 1200);
    } else {
      ElMessage.error(response.data.message || '提交失败，请稍后重试');
    }
  } catch (error) {
    console.error('提交请求错误:', error);
    ElMessage.error('网络错误，请稍后重试');
  }
};
</script>

<template>
  <div class="auth-page">
    <AppNavbar />

    <main class="auth-main auth-main--single auth-main--forgot">
      <!-- 忘记密码卡片：紧凑间距，矮屏时可整页上下滚动 -->
      <section class="auth-card form-box login">
        <div class="auth-card-head">
          <h2>重置密码</h2>
          <p>验证邮箱后即可设置新密码</p>
        </div>

        <form @submit.prevent="handleSubmit">
          <div class="auth-grid">
            <div class="input-box">
              <label>账号</label>
              <div class="input-field">
                <input type="text" v-model="account" required />
              </div>
            </div>
            <div class="input-box">
              <label>新密码</label>
              <div class="input-field">
                <input
                  :type="showPassword ? 'text' : 'password'"
                  v-model="password"
                  @input="validatePasswords"
                  required
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
              <label>确认新密码</label>
              <div class="input-field">
                <input
                  :type="showConfirmPassword ? 'text' : 'password'"
                  v-model="confirmPassword"
                  @input="validatePasswords"
                  required
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
                <input type="email" v-model="email" required />
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
                <input type="text" v-model="verificationCode" required />
              </div>
            </div>
          </div>

          <p v-if="passwordError" class="error">{{ passwordError }}</p>

          <button type="submit" class="btn">确认重置</button>

          <div class="auth-links">
            <RouterLink to="/HomeView">返回登录</RouterLink>
            <RouterLink to="/enrolll">注册</RouterLink>
          </div>
        </form>
      </section>
    </main>
  </div>
</template>

<style scoped>
/* 重置密码页使用统一 auth 组件样式（auth-card / auth-grid / input-box / btn-verify / auth-links），
   主题定义见 theme.css */
</style>
