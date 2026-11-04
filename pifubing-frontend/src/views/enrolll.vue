<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';

// 响应式数据
const account = ref('');
const password = ref('');
const confirmPassword = ref('');
const email = ref('');
const verificationCode = ref('');
const sendingCode = ref(false);
const alertMessage = ref(''); // 弹框消息内容
const showAlert = ref(false); // 是否显示弹框
const router = useRouter();
const name = ref('');
const gender = ref('');
const age = ref('');
const phone_number = ref('');

// 显示弹框的函数
const showAlertBox2 = (message) => {
  alertMessage.value = message;
  showAlert.value = true;
  setTimeout(() => {
    showAlert.value = false; // 3秒后自动隐藏弹框
  }, 3000);
};

// 发送验证码
const handleSendVerificationCode = async () => {
  if (!email.value) {
    showAlertBox2('请输入有效的邮箱地址');
    return;
  }

  sendingCode.value = true;

  try {
    const response = await axios.post(`/send_email`, {
      to: email.value,
    });

    if (response.data.code === 200) {
      showAlertBox2('验证码已发送，请检查您的邮箱');
    } else {
      showAlertBox2(response.data.message || '发送验证码失败');
    }
  } catch (error) {
    console.error('验证码发送错误:', error);
    showAlertBox2('发送验证码时出现错误');
  } finally {
    sendingCode.value = false;
  }
};

// 提交注册表单
const handleSubmit = async () => {
  if (!account.value || !password.value || !confirmPassword.value || !email.value || !verificationCode.value) {
    showAlertBox2('请完整填写所有必填字段');
    return;
  }

  if (password.value !== confirmPassword.value) {
    showAlertBox2('两次输入的密码不一致');
    return;
  }

  try {
    const response = await axios.post(`/users/register`, {
      account: account.value,
      password: password.value,
      email: email.value,
      code: verificationCode.value,
      name: name.value,
      gender: gender.value,
      // 后端 age 是 int，输入框即使为空也必须传数字，否则 JSON 绑定会报错。
      age: age.value === '' ? 0 : Number(age.value),
      phone_number:phone_number.value,  // 新增字段

    });

    if (response.data.code === 200) {
      showAlertBox2('注册成功！');
      setTimeout(() => {
        router.push('/'); // 跳转到登录页面
      }, 2000);
    } else {
      showAlertBox2(response.data.message || '注册失败，请重试');
    }
  } catch (error) {
    console.error('注册请求错误:', error);
    showAlertBox2('注册时发生错误，请稍后重试');
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
      <RouterLink to="/HomeView" class="auth-top-link">已有账号？返回登录</RouterLink>
    </header>

    <main class="auth-main auth-main--register">
      <section class="auth-brand-panel">
        <span class="ai-eyebrow"><span class="ai-pulse"></span> 建立你的健康档案</span>
        <h1>一次注册，<br><em>长期守护</em></h1>
        <p class="auth-lead">
          填写基础信息后，AI 能结合你的年龄、性别与既往病史，给出更贴合个人情况的皮肤健康建议。
        </p>

        <ul class="auth-points">
          <li>
            <span class="point-icon"><ion-icon name="id-card-outline"></ion-icon></span>
            <span><strong>个人健康档案</strong>病史、过敏史与问诊记录集中管理</span>
          </li>
          <li>
            <span class="point-icon"><ion-icon name="chatbubbles-outline"></ion-icon></span>
            <span><strong>连续对话</strong>每次问诊保留上下文，追问更自然</span>
          </li>
          <li>
            <span class="point-icon"><ion-icon name="lock-closed-outline"></ion-icon></span>
            <span><strong>仅用于健康分析</strong>不向第三方出售或公开你的信息</span>
          </li>
        </ul>

        <div class="auth-stat-row">
          <div class="auth-stat"><strong>1 分钟</strong><span>完成注册</span></div>
          <div class="auth-stat"><strong>加密</strong><span>账号与密码保护</span></div>
        </div>
      </section>

      <!-- 注册卡片：输入顺序与原有 e2e/业务保持一致 -->
      <section class="auth-card auth-card--wide form-box login">
        <div class="auth-card-head">
          <span class="auth-icon"><ion-icon name="person-add-outline"></ion-icon></span>
          <h2>创建账号</h2>
          <p>带 * 的信息用于生成更准确的健康建议</p>
        </div>

        <form @submit.prevent="handleSubmit">
          <div class="auth-grid">
            <div class="input-box">
              <span class="icon"><ion-icon name="person-outline"></ion-icon></span>
              <input type="text" v-model="account" required autocomplete="username" />
              <label>账号</label>
            </div>

            <div class="input-box">
              <span class="icon"><ion-icon name="id-card-outline"></ion-icon></span>
              <input type="text" v-model="name" required autocomplete="name" />
              <label>姓名</label>
            </div>

            <div class="input-box">
              <span class="icon"><ion-icon name="male-female-outline"></ion-icon></span>
              <input type="text" v-model="gender" required />
              <label>性别</label>
            </div>

            <div class="input-box">
              <span class="icon"><ion-icon name="calendar-outline"></ion-icon></span>
              <input type="number" v-model.number="age" min="0" max="150" required />
              <label>年龄</label>
            </div>

            <div class="input-box">
              <span class="icon"><ion-icon name="call-outline"></ion-icon></span>
              <input type="tel" v-model="phone_number" required autocomplete="tel" />
              <label>手机号</label>
            </div>

            <div class="input-box">
              <span class="icon"><ion-icon name="lock-closed-outline"></ion-icon></span>
              <input type="password" v-model="password" required autocomplete="new-password" />
              <label>密码</label>
            </div>

            <div class="input-box">
              <span class="icon"><ion-icon name="shield-checkmark-outline"></ion-icon></span>
              <input type="password" v-model="confirmPassword" required autocomplete="new-password" />
              <label>确认密码</label>
            </div>

            <div class="input-box input-box--full">
              <span class="icon"><ion-icon name="mail-outline"></ion-icon></span>
              <input type="email" v-model="email" required autocomplete="email" />
              <label>邮箱</label>
              <button
                type="button"
                class="btn-verify"
                :disabled="sendingCode"
                @click="handleSendVerificationCode"
              >
                {{ sendingCode ? '发送中...' : '发送验证码' }}
              </button>
            </div>

            <div class="input-box input-box--full">
              <span class="icon"><ion-icon name="key-outline"></ion-icon></span>
              <input type="text" v-model="verificationCode" required inputmode="numeric" />
              <label>请输入邮箱验证码</label>
            </div>
          </div>

          <button type="submit" class="btn">注册并开始使用</button>
        </form>

        <div class="login-register">
          <p>已经有账号？<RouterLink to="/HomeView">直接登录</RouterLink></p>
        </div>

        <p class="auth-legal">注册即代表你同意服务条款与隐私政策。AI 建议不替代线下医生诊断。</p>
      </section>
    </main>

    <div v-if="showAlert" class="alert-box">
      <p>{{ alertMessage }}</p>
    </div>
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

.auth-main--register {
  align-items: start;
  padding-top: 34px;
}

.auth-main--register .auth-brand-panel {
  position: sticky;
  top: 36px;
}

@media (max-width: 960px) {
  .auth-main--register .auth-brand-panel {
    position: static;
  }
}
</style>
