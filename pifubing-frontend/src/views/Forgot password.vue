<script setup>
import { ref } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';

// 定义响应式数据
const account = ref('');
const password = ref('');
const confirmPassword = ref('');
const email = ref('');
const verificationCode = ref('');
const sendingCode = ref(false);
const passwordError = ref('');
const formError = ref('');
const alertMessage = ref('');
const showAlert = ref(false);
const router = useRouter();

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
    const response = await axios.post(`http://8.137.81.23:9094/send_email`, {
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
  // 清空错误提示
  formError.value = '';

  // 验证表单数据完整性
  if (!account.value) {
    showAlertBox2('请输入账号');
    return;
  }
  if (!email.value) {
    showAlertBox2('请输入邮箱地址');
    return;
  }
  if (!verificationCode.value) {
    showAlertBox2('请输入验证码');
    return;
  }
  if (!password.value || !confirmPassword.value) {
    showAlertBox2('请填写密码和确认密码');
    return;
  }
  if (passwordError.value) {
    showAlertBox2(passwordError.value);
    return;
  }

  try {
    // 提交表单
    const response = await axios.post(`http://8.137.81.23/users/forget/password`, {
      account: account.value,
      password: password.value,
      email: email.value,
      code: verificationCode.value,
    });

    if (response.data.code === 200) {
      showAlertBox2('密码重置成功！');
      setTimeout(() => {
        router.push('/'); // 跳转到登录页面
      }, 2000); // 2秒延迟
    } else {
      showAlertBox2(response.data.message || '提交失败，请稍后重试');
    }
  } catch (error) {
    console.error('提交请求错误:', error);
    showAlertBox2('网络错误，请稍后重试');
  }
};
</script>

<template>
  <header style="position: fixed;">
    <div>
      <img src="../assets/t.png" alt="logo" class="logo1" />
    </div>
    <nav class="navigation2">
      <RouterLink to="/HomeView">
        <span class="icon-tu">
          <ion-icon name="arrow-back-outline"></ion-icon>
        </span>
      </RouterLink>
    </nav>
  </header>

  <div class="box1">
    <div class="wrapper2">
      <span class="icon-close"></span>
      <div class="form-box login">
        <h2>忘记密码</h2>
        <form @submit.prevent="handleSubmit">
          <div class="input-box">
            <span class="icon"><ion-icon name="person-outline"></ion-icon></span>
            <input type="text" v-model="account" required />
            <label>账号</label>
          </div>
          <div class="input-box">
            <span class="icon"><ion-icon name="lock-closed-outline"></ion-icon></span>
            <input
              type="password"
              v-model="password"
              @input="validatePasswords"
              required
            />
            <label>密码</label>
          </div>
          <div class="input-box">
            <span class="icon"><ion-icon name="lock-open-outline"></ion-icon></span>
            <input
              type="password"
              v-model="confirmPassword"
              @input="validatePasswords"
              required
            />
            <label>确认密码</label>
          </div>
          <div class="input-box">
            <span class="icon"><ion-icon name="mail-outline"></ion-icon></span>
            <input type="email" v-model="email" required />
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
          <div class="input-box">
            <span class="icon"><ion-icon name="shield-checkmark-outline"></ion-icon></span>
            <input type="text" v-model="verificationCode" required />
            <label>请输入验证码</label>
          </div>
          <RouterLink to="/HomeView">
          <button type="submit" class="btn">确认</button>
         </RouterLink>
        </form>
      </div>
    </div>
  </div>

  <!-- 全局弹框 -->
  <div v-if="showAlert" class="alert-box">
    <p>{{ alertMessage }}</p>
  </div>

  <bottom>
    <nav class="navigation">
      <p class="zi"></p>
    </nav>
  </bottom>
</template>

<style scoped>
header{
  width: 100%;
  padding: 20px 100px;
z-index:998;
/* background: linear-gradient(90deg, #4da8b7 40% ,#55ceb2e4); */
background-color: #12405d;
  display: flex;
  justify-content: space-between;
  align-items: center;
  max-height: 10vh;
  height: 10vh;
}

bottom{
  position: fixed;
  bottom:0;
  width: 100%;
  padding: 20px 100px;
/* background-color:rgba(92, 90, 90, 0.7); */
/* background-color: #b0b4b4; */
/* background: linear-gradient(90deg, #4da8b7 40% ,#55ceb2e4); */
background-color: #12405d;
  display: flex;
  justify-content: space-between;
  align-items: center;
  z-index: 99;
  max-height: 4vw;
}

.alert-box {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #fff;
  color: #0e0b0b;
  padding: 15px 20px;
  border-radius: 5px;
  box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
  z-index: 1000;
}
</style>
