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
  <header style="position: fixed;">
    <div><img src="../assets/t.png" alt="logo" class="logo1" /></div>
  </header>

  <div class="q">
    <div class="wrapper1">
      <div class="form-box login">
        <h2>注册</h2>

      
        
        <form @submit.prevent="handleSubmit">
          <!-- 账号 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="person-outline"></ion-icon></span>
            <input type="text" v-model="account" required />
            <label>账号</label>
          </div>

               <!-- 姓名 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="person-remove-outline"></ion-icon></span>
            <input type="name" v-model="name" required />
            <label>姓名</label>
          </div>

                  <!-- 性别 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="male-female-outline"></ion-icon></span>
            <input type="gender" v-model="gender" required />
            <label>性别</label>
          </div>

                     <!-- 年纪-->
          <div class="input-box">
            <span class="icon"><ion-icon name="lock-open-outline"></ion-icon></span>
            <input type="number" v-model.number="age" min="0" max="150" required />
            <label>年龄</label>
          </div>

          <div class="input-box">
            <span class="icon"><ion-icon name="phone-portrait-outline"></ion-icon></span>
            <input type="tel" v-model="phone_number" required />
            <label>手机号</label>
          </div>

          <!-- 密码 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="lock-closed-outline"></ion-icon></span>
            <input type="password" v-model="password" required />
            <label>密码</label>
          </div>

          <!-- 确认密码 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="bag-add-outline"></ion-icon></span>
            <input type="password" v-model="confirmPassword" required />
            <label>确认密码</label>
          </div>

          <!-- 邮箱 -->
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

          <!-- 验证码 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="shield-checkmark-outline"></ion-icon></span>
            <input type="text" v-model="verificationCode" required />
            <label>请输入验证码</label>
          </div>

          <!-- 注册按钮 -->
          <button type="submit" class="btn">注册</button>
        </form>

        <!-- 已有账号链接 -->
        <div class="login-register">
          <p>已经有账号？</p>
          <RouterLink to="/HomeView">
            <a href="#" class="register-link" id="color">登录</a>
          </RouterLink>
        </div>
      </div>
    </div>
  </div>

  <!-- 全局弹框 -->
  <div v-if="showAlert" class="alert-box">
    <p>{{ alertMessage }}</p>
  </div>

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

.alert-box {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #fff;
  color: #100f0f;
  padding: 15px 20px;
  border-radius: 5px;
  box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
  z-index: 1000;
}

#color{
  color: #162938;
}

.q{
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: url('../assets/登录.png')no-repeat;
  background-size: cover;
  /* background-position: center; */
  padding: 20px;

}
</style>
