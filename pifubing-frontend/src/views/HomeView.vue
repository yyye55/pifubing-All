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
  <header style="position: fixed;">
    <div><img src="../assets/t.png" alt="logo" class="logo1" /></div>
  </header>

  <div class="box1">
    <div class="wrapper">
      <div class="form-box login">
        <h2>登录</h2>
        <!-- 表单 -->
        <form @submit.prevent="handleLogin">
          <!-- 账号输入框 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="phone-portrait-outline"></ion-icon></span>
            <input type="text" v-model="account" required />
            <label>账号</label>
          </div>

          <!-- 密码输入框 -->
          <div class="input-box">
            <span class="icon"><ion-icon name="bag-outline"></ion-icon></span>
            <input type="password" v-model="password" required />
            <label>密码</label>
          </div>

          <!-- 表单错误提示 -->
          <p v-if="formError" class="error">{{ formError }}</p>

          <!-- 提交按钮 -->
          <button type="submit" class="btn" :disabled="loading">
            {{ loading ? '登录中...' : '登录' }}
          </button>

          <div class="remember-forgot">
            <RouterLink to="/Forgot password">忘记密码?</RouterLink>
          </div>

          <!-- 注册链接 -->
          <div class="login-register">
            <p>还没有账号? <RouterLink to="/enrolll">注册</RouterLink></p>
          </div>
        </form>
      </div>
    </div>
  </div>

  <!-- 弹框 -->
  <div v-if="showAlert" class="alert-box">{{ alertMessage }}</div>

  <bottom>
    <nav class="navigation">
      <div class="zi-nav">
        <p class="zi"></p>
      </div>
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


/* 样式调整 */
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
  color: #333;
  padding: 10px 20px;
  border-radius: 5px;
  z-index: 1000;
}
.error {
  color: #fff;
  font-size: 14px;
  margin: 10px 0;
}
</style>
