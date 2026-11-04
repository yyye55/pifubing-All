<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElAvatar, ElRow, ElCol, ElCard, ElForm, ElFormItem, ElInput, ElButton, ElDialog, ElMessage } from 'element-plus';

import { useRouter } from 'vue-router';
import AppNavbar from '../components/AppNavbar.vue';


// 获取 token
const token = localStorage.getItem('token');
const router = useRouter();
// 上传头像后保存到 LocalStorage


// 初始化 avatarUrl 和 backgroundImage
const avatarUrl = ref('');
const backgroundImage = ref('');
//const backgroundImage = ref(require('../assets/images'));  // 使用 require 来加载本地图片

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

// 编辑状态
const isEditable = ref(false);  // 用于控制个人信息的编辑状态
const isMedicalEditable = ref(false); // 用于控制病史和过敏史的编辑状态

// 获取头像 input 的 ref
const avatarInput = ref(null);


// 获取隐藏的文件输入框
const backgroundImageInput = ref(null);

// 选择背景图片按钮的处理函数
const selectBackgroundImage = () => {
  // 模拟点击文件选择器
  backgroundImageInput.value.click();
};

const medicalHistoryData = ref([
  { type: '高血压', message: '多吃蔬菜水果，避免油腻食物，多运动。', ask: '头晕，胸闷', confidence: 85, url: '' },
  { type: '糖尿病', message: '控制饮食，定期检查血糖。', ask: '口渴，乏力', confidence: 90, url: '' },
  // 更多数据...
]);

const dialogVisible = ref(false);  // 控制弹框的显示与隐藏
const selectedItem = ref(null);  // 存储点击的健康历史记录

// 截断长消息，限制显示长度
const truncateMessage = (message) => {
  // 限制显示长度为 80 个字符，超过则加上 "..."
  return message.length > 80 ? message.slice(0, 80) + '...' : message;
};

// 显示完整建议的函数
const showFullMessage = (item) => {
  selectedItem.value = item;  // 设置点击的记录为选中的项
  dialogVisible.value = true;  // 显示弹框
}

// 用户查找所有个人信息
const loadUserData = async () => {  // renamed function
  try {
    if (!token) {
      useRouter().push('/');  // 如果没有 token，跳转到登录页面
      console.error('未登录，请先登录');
      return;
    }
    const response = await axios.get(
      '/users/check_users_information',
      {
        headers: {
          Authorization: `Bearer ${token}`  // 将 Bearer token 包含在请求头中
        }
      }
    );
    console.log('请求数据成功', response.data);

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

      // // 更新背景图片
      // backgroundImage.value = 'https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRDijTs7b1Zmz0DHwcebyQOVdGXfB8eZVDYcg&s';
      backgroundImage.value = '../assets/登.png';
    }
  } catch (error) {
    console.error('请求数据失败', error);
  }
};

// 在页面加载时请求数据
onMounted(() => {
  loadUserData();  // calling the renamed function
});

// 提交个人信息
const submitForm = async () => {
  try {
    if (!token) {
      useRouter().push('/');  // 如果没有 token，跳转到登录页面
      console.error('未登录，请先登录');
      return;
    }

    // 从 formData 获取用户填写的字段
    const formDataToSubmit = {
      account: formData.value.account,  // 用户填写的账号
      email: formData.value.email,      // 用户填写的邮箱
      name: formData.value.name,        // 用户填写的姓名
      //age: formData.value.age,          // 用户填写的年龄
      age: Number(formData.value.age),
      gender: formData.value.gender,    // 用户填写的性别
      phone_number: formData.value.phone_number ,// 用户填写的电话
    };

    console.log('提交的数据：', formDataToSubmit);

    const response = await axios.post(
      '/users/updates_users_information',  // 更新个人信息的接口
      formDataToSubmit,  // 提交的 formData 数据
      {
        headers: {
          Authorization: `Bearer ${token}`  // 将 Bearer token 包含在请求头中
        }
      }
    );
    
    console.log('更新成功', response.data);
    console.log('提交的数据：', formDataToSubmit);

    if (response.data.code === 200) {
      ElMessage.success('个人信息更新成功');
      // 更新成功后重新加载用户信息
      loadUserData();
    } else {
      ElMessage.error('更新失败：' + response.data.message);
    }
  } catch (error) {
    console.error('提交失败', error);
    ElMessage.error('提交失败');
  }
};

// 提交病史和过敏史
const submitDiseaseInfo = async () => {
  try {
    const response = await axios.post(
      '/users/create_disease_information',
      {
        medical_history: formData.value.medicalHistory,
        allergies: formData.value.allergyHistory
      },
      {
        headers: {
          Authorization: `Bearer ${token}`  // 将 Bearer token 包含在请求头中
        }
      }
    );
    console.log('病史和过敏史更新成功', response.data);

    if (response.data.code === 200) {
      ElMessage.success('病史和过敏史更新成功');
      // 更新成功后重新加载用户信息
      loadUserData();
    } else {
      ElMessage.error('更新失败：' + response.data.message);
    }
  } catch (error) {
    console.error('提交病史和过敏史失败', error);
    ElMessage.error('提交失败，请稍后再试');
  }
};

// 处理头像上传
const handleAvatarChange = async (event) => {
  const file = event.target.files[0]; // 获取用户选择的文件

  if (file) {
    // 如果需要进行上传至服务器的操作，可以如下发送请求
    const formData = new FormData();
    formData.append('avatar', file);  // 将文件添加到表单数据中

    try {
      if (!token) {
        ElMessage.warning('请先登录后再上传头像');
        router.push('/HomeView');
        return;
      }

      const response = await axios.post(
        '/users/updates_avatar', // 头像更新的接口
        formData,
        {
          headers: {
            Authorization: `Bearer ${token}`,  // 将 Bearer token 包含在请求头中
            'Content-Type': 'multipart/form-data'  // 指定内容类型为文件上传
          }
        }
      );
      console.log('头像上传成功', response.data);
      if (response.data.code === 200) {
        // 更新头像 URL
        avatarUrl.value = response.data.data; // 使用服务器返回的 URL
        localStorage.setItem('avatarUrl', response.data.data); // 更新 localStorage 中的头像 URL
        ElMessage.success('头像上传成功！');
        console.log('头像更新成功', response.data.data);
      } else {
        console.error('头像更新失败:', response.data.message);
        ElMessage.error('头像更新失败：' + response.data.message);
      }
    } catch (error) {
      console.error('请求失败', error);
      ElMessage.error('上传失败，请稍后再试');
    }
  }
};



// 修改按钮点击事件
const enableEdit = () => {
  isEditable.value = true;  // 启用编辑模式
};

// 修改病史和过敏史按钮点击事件
const enableMedicalEdit = () => {
  isMedicalEditable.value = true;  // 启用病史和过敏史的编辑模式
};


const loadMedicalHistory = async () => {
  try {
    if (!token) {
      useRouter().push('/');  // If there's no token, redirect to login page
      console.error('未登录，请先登录');
      return;
    }
    const response = await axios.get(
      '/users/check_users_history',  // New API endpoint
      {
        headers: {
          Authorization: `Bearer ${token}`,  // Include token in the request header
        }
      }
    );
    console.log('个人健康历史数据请求成功', response.data);
    
    if (response.data.code === 200) {
      medicalHistoryData.value = response.data.data || [];  // Store the response data
    } else {
      console.error('查询失败：', response.data.message);
    }
  } catch (error) {
    console.error('请求失败', error);
  }
};

onMounted(() => {
  loadUserData();  // Existing function to load user info
  loadMedicalHistory();  // Fetch medical history data
});






</script>


<template>
  <div class="profile-container">
    <AppNavbar />

    <div class="box">
  
  <div class="lishi-pel-container">
    

    <!-- 左侧：个人健康历史 -->
    <div class="lishi">
      <div class="history-image-container">
    <div class="history-text">
      你一定不知道的皮肤小知识</div>
    <div class="text">点击进入 &nbsp;&nbsp;&nbsp;  →</div>
  </div>
      <el-col>
        <div class="t">
  <h3>个人健康历史</h3>
  <span><ion-icon name="search-outline"></ion-icon></span>
</div>
        <el-card class="content-card1">
          
          <div v-if="medicalHistoryData.length">
            <div v-for="(item, index) in medicalHistoryData" :key="index">
              <el-row>
                <el-col :span="12" class="jianyi" @click="showFullMessage(item)">
                  <strong>病史类型:</strong> {{ truncateMessage(item.type) }}<br>
                  <strong>用户描述:</strong> {{ truncateMessage(item.ask, 30) }}<br>
                  <strong>置信度:</strong> {{ item.confidence }}%<br>
                </el-col>
                <el-col :span="12" class="jianyis" @click="showFullMessage(item)">
                  <strong>建议:</strong> {{ truncateMessage(item.message) }}
                </el-col>
                <hr>
              </el-row>
            </div>
          </div>
          <div v-else>
            <p>没有健康历史数据可显示。</p>
          </div>
        </el-card>
      </el-col>

      <!-- 弹出框 -->
      <el-dialog v-model="dialogVisible" :title="'完整健康历史'" style="z-index: 999;left: -6%;">
        <div v-if="selectedItem">
          <p><strong>病史类型:</strong> {{ selectedItem.type }}</p>
          <p><strong>建议:</strong> {{ selectedItem.message }}</p>
          <p><strong>用户描述:</strong> {{ selectedItem.ask }}</p>
          <p><strong>图片链接:</strong> <a :href="selectedItem.url" target="_blank">查看图片</a></p>
          <p><strong>置信度:</strong> {{ selectedItem.confidence }}%</p>
        </div>
        <span slot="footer" class="dialog-footer">
          <el-button @click="dialogVisible = false">关闭</el-button>
        </span>
      </el-dialog>
    </div>

    <!-- 右侧：个人信息填写区域 -->
    <div class="pel">
      <div class="profile-info">
        <el-row type="flex" justify="start" align="middle" class="profile-details">
          <!-- 头像显示 -->
          <el-col  class="profile-avatar">
            <el-avatar size="190" :src="avatarUrl" class="avatar" @click="avatarInput.click()"></el-avatar>
            <input type="file" ref="avatarInput" @change="handleAvatarChange" style="display: none;" />
          </el-col>
          <el-col  class="profile-text">
            <h2>{{ formData.account }}</h2>
            <p>{{ formData.email }}</p>
          </el-col>
        </el-row>
      </div>
      <div class="form-container">
  <el-row gutter="200" type="flex" justify="space-between">
    <!-- 个人信息表单上半部分 -->
    <el-col>
      <el-card class="content-card">
        <el-form :model="formData" label-width="70px">
          <el-form-item label="账号：" >
            <el-input v-model="formData.account" placeholder="请输入账号" :disabled="!isEditable" style="width:auto;" class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item label="邮箱：">
            <el-input v-model="formData.email" placeholder="请输入邮箱" :disabled="!isEditable" style="width:auto"class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item label="姓名：">
            <el-input v-model="formData.name" placeholder="请输入姓名" :disabled="!isEditable" style="width:auto;"class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item label="年龄：">
            <el-input v-model="formData.age" placeholder="请输入年龄" :disabled="!isEditable" style="width:auto"class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item label="性别：">
            <el-input v-model="formData.gender" placeholder="请输入性别" :disabled="!isEditable" style="width:auto"class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item label="手机号：">
            <el-input v-model="formData.phone_number" placeholder="请输入号码" :disabled="!isEditable" style="width:auto"class="el-form-item__label"></el-input>
          </el-form-item>
       
          <el-button type="primary" @click="submitForm" :disabled="!isEditable" style="margin-left: 63px;">提交</el-button>
          <el-button type="danger" @click="enableEdit" :disabled="isEditable">修改</el-button>
        </el-form>
      </el-card>
    </el-col>
  </el-row>

  <el-row gutter="200" type="flex" justify="space-between">
    <!-- 个人信息表单下半部分 -->
    <el-col>
      <el-card class="content-cardss">
        <el-form :model="formData" label-width="70px">
          <el-form-item label="病史：">
            <el-input v-model="formData.medicalHistory" placeholder="请说明情况" :disabled="!isMedicalEditable" style="width:auto"class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item label="过敏史：">
            <el-input v-model="formData.allergyHistory" placeholder="没有填‘无’，有则详细填写" :disabled="!isMedicalEditable" style="width:auto"class="el-form-item__label"></el-input>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="submitDiseaseInfo" :disabled="!isMedicalEditable">提交健康信息</el-button>
            <el-button type="danger" @click="enableMedicalEdit" :disabled="isMedicalEditable">修改健康信息</el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </el-col>
  </el-row>
</div>
    </div>
  </div>
</div>
    <footer class="footer">
      <div class="copyright">
        <p>AI 皮肤医疗顾问 · AI 建议仅供参考，如有不适请及时就医</p>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.text{
  position: absolute;
  top: 60%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: white; /* 文字颜色 */
  background-color: #12405d84;
  width: 10vw;
  font-size: 1.2em; /* 文字大小 */
  font-weight: bold; /* 文字粗细 */
  padding: 8px; /* 内边距 */
  border-radius: 20px; /* 圆角 */
  z-index: 10; /* 确保文字在图片上方 */
}
.history-image-container {
  margin-bottom: 10px; /* 与下方标题的间距 */
  width: 100%;
  position: relative;
  height: 20vh;
  margin-top: 13vh;
  display: flex;
  align-items: center;
  justify-items: center;
}

.history-icon {
  width:100%; /* 图片宽度 */
  height: 21vh;
  object-fit: cover; /* 确保图片覆盖整个容器 */
}
.history-text {
  position: absolute;
  top: 30%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: white; /* 文字颜色 */
  font-size: 2em; /* 文字大小 */
  font-weight: bold; /* 文字粗细 */
  padding: 10px; /* 内边距 */
  border-radius: 5px; /* 圆角 */
  z-index: 10; /* 确保文字在图片上方 */
}
.t {
  font-size: 1.2em; 
  margin-bottom: 5px;
  margin-left: 10px;
  color: aliceblue;
  background-color: #12405daf;
  padding: 8px;
  border-radius: 10px;
  width: 20vw;
  display: flex; /* 使用 Flexbox 布局 */
  align-items: center; /* 垂直居中对齐 */
  justify-content: space-between; /* 两端对齐 */
}

.lishi-pel-container {
  display: flex; /* 使用 Flexbox 布局 */
  justify-content: space-between; /* 左右两端对齐 */
  align-items: flex-start; /* 垂直方向顶部对齐 */
  padding: 10px; /* 添加内边距 */
  z-index: 1;
}

.lishi {
  margin-right: 10px; /* 右侧间距 */
  /* margin-top: 32vh; */
  z-index: 999;
  /* background-color: #4da9b74b; */
  padding:10px;
  align-items:center;
  width: 70vw;
}

.pel {
  background-color:rgba(237, 237, 235, 0.339);
  padding: 10px;
  width: 30vw; /* 设置宽度为 45% */
  align-items: center;
  justify-content: center;
  height:auto;
  margin-top: 12vh; /* 顶部外边距 */
  margin-right: 0; /* 右侧外边距 */
  margin-bottom: 0; /* 底部外边距 */
  margin-left: 0; /* 左侧外边距 */
  display: flex;
  flex-direction: column;
  border-radius: 50px;
  z-index: 999;
}

.content-card1 {
  border: none;
  height: auto; /* 高度根据内容自动调整 */
  overflow-y: auto;
  margin-bottom: 10px;
  align-items: center;
  flex-direction: column;
  margin-left: 10px;
  background-color:transparent;
  color: #fff;
  
}

.box {
  padding: 10px;

}

.footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  padding: 20px 100px;
  /* background: linear-gradient(90deg, #4da8b7 40%, #55ceb2e4); */
  background-color: #12405d;
  box-shadow: 0 0 0 2px rgb(255, 255, 255); /* 添加底部白色边框效果 */
  bottom: 0;
}

.navigation ul {
  list-style: none;
  display: flex;
  gap: 20px; /* 调整链接之间的间距 */
}

.navigation ul li a {
  color: #fff;
  text-decoration: none;
}

.slogan {
  color: #fff;
  font-size: 2em;
}

.qrcode img {
  width: 60px; /* 根据需要调整二维码大小 */
  height: 60px;
  margin-left: 20px;
}

.legal-links,
.copyright {
  color: #fff;
  font-size: 14px;
}

.w {
  position: fixed;
  top: 20px; /* 根据需要调整 */
  right: 20px; /* 根据需要调整 */
  z-index: 1000;
  font-size: 1.1em;
  color: #fff;
}

.navigations {
  display: flex;
  align-items: flex-start; /* 左对齐 */
  align-items: center;
  padding: 10px;
}

.navigations .btnLogin-popups {
  width: 130px;
  height: 50px;
  background: transparent;
  border: 2px solid #fff;
  outline: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 1.1em;
  color: #fff;
  font-weight: 500;
  margin-left: 90px;
  margin-top: 49px;
  transition: .5s;
  text-align: center;
  max-height: 4vw;
  z-index: 997;
}

.navigations .btnLogin-popups:hover {
  background: #fff;
  color: #162938;
}

.navigations .icon-tu {
  position: absolute;
  margin-right: 5vw;
  font-size: 1.7em;
  color: #fff;
  line-height: 57px;
}

.language-select option {
  color: #000;
}

header {
  width: 100%;
  padding: 20px 100px;
  z-index: 1000;
   /* background: linear-gradient(90deg, #4da8b7 40%, #55ceb2e4); */
   background-color: #12405d;
  display: flex;
  justify-content: space-between;
  max-height: 20vh;
  height: 12vh;
  box-shadow: 0 2px 0 0 rgb(255, 255, 255); /* 添加底部白色边框效果 */




  
}

.language-select {
  background-color: transparent;
  border: 1px solid rgb(255, 255, 255);
  border-radius: 8px;
  color: #e8ecf0;
  font-size: 1.1em;
  cursor: pointer;
  position: absolute;
  left:420px; /* 根据需要调整 */
  top: 50%;
  transform: translateY(-50%);
  outline: none;
}

.language-select::after {
  border: 1px solid rgb(255, 255, 255);
  color: black;
}

.jianyi {
  background-color: #12405da8;
  overflow-y: hidden;
  height: 10vh;
  cursor: pointer; /* 设置光标为可点击 */
  border-radius: 10px 0 0 10px;
  margin-bottom: 7px;
  width:100%;
  padding: 5px;
}

.jianyis {
  background-color: #12405da8;
  overflow-y: hidden;
  height: 10vh;
  cursor: pointer; /* 设置光标为可点击 */
  border-radius:  0  10px 10px 0;
  width:100%;
  padding: 5px;
}

.el-dialog {
  z-index: 9999!important; /* Ensure it's on top */
}

@media screen and (max-width: 768px) {
  .lishi {
    flex-direction: column; /* 在小屏幕上垂直排列 */
  }
  .pel, .content-card1 {
    width: 100%; /* 在小屏幕上宽度为 100% */
  }
}

/*1*/
.el-input {
  width: 20vw;
}

.profile-container {
  width: 100%;
  background-image: url('../assets/个人.jpg');
  background-position: center;
  background-size: cover;
  background-repeat: no-repeat;
  min-height: 100vh;
}

.profile-info {
  color: rgb(247, 243, 243);
  z-index: 1;
  display: flex;
  justify-content: center;
}

.profile-details {
  display: flex; /* 启用 Flexbox 布局 */
  align-items: center; /* 垂直居中对齐 */
  justify-content: space-between; /* 两端对齐 */
  width: 100%; /* 控制整体宽度 */
}

.profile-avatar {
  flex: 1; /* 使头像区域占据剩余空间 */
  justify-content: center;
  z-index: 100;
}

.avatar {
  border-radius: 50%; /* 头像为圆形 */
  width: 150px; /* 强制设置头像宽度 */
  height: 150px; /* 强制设置头像高度 */
}

.profile-text {
  flex: 2; /* 使文本区域占据更多空间 */
  justify-content: center;
  z-index: 1;
  display: flex;
  flex-direction: column;
  margin-left:25px;
  margin-top: 25px;
}

.profile-text h2 {
  font-size: 24px;
  margin: 5px 0;
}

.profile-text p {
  margin: 5px 0;
}

.form-container {
  margin-top: 8px;
  border: none;
}

.content-card {
  border: none;
  border-radius: 50px;
  background-color: #12405d63;
  color: #fff;
  width:27vw;
  max-height: 35vh;
  overflow-y: auto;
  align-items: center;
  display: flex;
  justify-content: center;
  margin: 0;
  margin-bottom: 5px;
  position: relative; /* 相对定位 */
  z-index: 1; /* 确保卡片的 z-index 较低 */
}

.content-cardss {
  border: none;
  border-radius: 50px;
  width:27vw;
  height:20vh;
  align-items: center;
  display: flex;
  justify-content: center;
  margin: 0 auto;
  background-color: #12405d63;
  
}

/* 自定义滚动条样式 */
.content-card::-webkit-scrollbar,
.content-cardss::-webkit-scrollbar {
  width: 10px; /* 滚动条宽度 */
 ;
}

.content-card::-webkit-scrollbar-track,
.content-cardss::-webkit-scrollbar-track {
  background: rgba(0, 0, 0, 0.1); /* 滚动条轨道背景色 */
}

.content-card::-webkit-scrollbar-thumb,
.content-cardss::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.3); /* 滚动条滑块背景色 */
  border-radius: 4px; /* 滑块圆角 */
}

.content-card::-webkit-scrollbar-thumb:hover,
.content-cardss::-webkit-scrollbar-thumb:hover {
  background: rgba(0, 0, 0, 0.5); /* 滑块悬停时的背景色 */
}

.content-card::-webkit-scrollbar-thumb:window-inactive,
.content-cardss::-webkit-scrollbar-thumb:window-inactive {
  background: rgba(0, 0, 0, 0.2); /* 滑块非活动时的背景色 */
}

.el-form-item {
  margin-bottom: 20px;
}

.el-form-item__label {
    align-items: flex-start;
    box-sizing: border-box;
    display: inline-flex;
    flex: 0 0 auto;
    font-size: var(--el-form-label-font-size);
    height: 32px;
    justify-content: flex-end;
    line-height: 32px;
    padding: 0 12px 0 0;
}

.el-button {
  margin-top: 20px;
}

.icon-tu {
  position: relative;
  margin-left: 1vw;
  font-size: 3em;
  color: #fff;
  margin-top: 30px;
  z-index: 1000;
  position: fixed;
}
/* 媒体查询以适应不同屏幕尺寸 */
@media screen and (max-width: 1200px) {
  .w {
    top: 15px;
    right: 15px;
  }
}

@media screen and (max-width: 992px) {
  .w {
    top: 10px;
    right: 10px;
  }
}

@media screen and (max-width: 768px) {
  .w {
    top: 5px;
    right: 5px;
  }
}

@media screen and (max-width: 576px) {
  .w {
    top: 0;
    right: 0;
  }
}
@media screen and (max-width: 1200px) {
  .language-select {
    margin-left: -100px; /* 调整适合小屏幕的值 */
  }
}
</style>

<style scoped>
/* =========================================================
   个人中心：从深色背景 + 半透明卡片改为医疗健康档案布局
   ========================================================= */
.profile-container {
  min-height: 100vh;
  padding-bottom: 30px;
  background: var(--ai-mint-2);
}

.box {
  width: min(1180px, calc(100% - 40px));
  margin: 0 auto;
  padding: 28px 0 18px !important;
}

.lishi-pel-container {
  display: grid !important;
  grid-template-columns: minmax(0, 1.05fr) minmax(340px, 0.75fr);
  gap: 20px;
  align-items: start !important;
  padding: 0 !important;
}

.lishi {
  width: auto !important;
  margin: 0 !important;
  padding: 0 !important;
}

.pel {
  width: auto !important;
  margin: 0 !important;
  padding: 24px !important;
  border: 1px solid var(--ai-line);
  border-radius: 24px !important;
  background: #fff !important;
  box-shadow: var(--ai-shadow-sm);
  display: block !important;
}

.history-image-container {
  height: 150px !important;
  margin: 0 0 14px !important;
  overflow: hidden;
  border-radius: 20px;
  background: linear-gradient(135deg, var(--ai-primary-deep), var(--ai-primary));
}

.history-icon {
  width: 100% !important;
  height: 150px !important;
  object-fit: cover;
  opacity: 0.55;
}

.history-text {
  top: 36% !important;
  font-size: clamp(16px, 2vw, 22px) !important;
  font-weight: 700;
  text-shadow: 0 2px 12px rgba(0, 0, 0, 0.2);
}

.text {
  top: auto !important;
  bottom: 16px;
  left: 18px !important;
  transform: none !important;
  width: auto !important;
  padding: 7px 12px !important;
  border-radius: 99px !important;
  background: rgba(255, 255, 255, 0.18) !important;
  backdrop-filter: blur(10px);
  color: #fff !important;
  font-size: 12px !important;
  font-weight: 600 !important;
}

.t {
  width: 100% !important;
  margin: 0 0 12px !important;
  padding: 12px 15px !important;
  border: 1px solid var(--ai-line);
  border-radius: 14px !important;
  background: #fff !important;
  color: var(--ai-ink) !important;
  font-size: 15px !important;
  font-weight: 700;
  box-shadow: var(--ai-shadow-sm);
}

.t ion-icon {
  color: var(--ai-primary);
}

.content-card1 {
  margin: 0 !important;
  padding: 0 !important;
  background: transparent !important;
  color: var(--ai-ink) !important;
}

.content-card1 :deep(.el-card__body) {
  padding: 18px !important;
}

.content-card1 .el-row {
  display: block;
}

.jianyi,
.jianyis {
  width: 100% !important;
  height: auto !important;
  min-height: 76px;
  margin-bottom: 8px !important;
  padding: 11px 12px !important;
  border-radius: 12px !important;
  background: #f7fbfa !important;
  color: var(--ai-ink-soft) !important;
  font-size: 12px;
  line-height: 1.7;
  cursor: pointer;
  transition: background 0.2s ease;
}

.jianyi:hover,
.jianyis:hover {
  background: var(--ai-primary-soft) !important;
}

.jianyi strong,
.jianyis strong {
  color: var(--ai-ink);
}

hr {
  margin: 6px 0;
  border: 0;
  border-top: 1px solid var(--ai-line);
}

.profile-info {
  justify-content: flex-start !important;
  color: var(--ai-ink) !important;
  margin-bottom: 16px;
}

.profile-details {
  justify-content: flex-start !important;
  gap: 16px;
}

.profile-avatar {
  flex: 0 0 auto !important;
  display: flex;
  justify-content: flex-start !important;
}

.avatar {
  width: 96px !important;
  height: 96px !important;
  border: 4px solid #fff;
  background: var(--ai-primary-soft);
  box-shadow: var(--ai-shadow);
  cursor: pointer;
}

.profile-text {
  flex: 1 !important;
  margin: 0 0 0 4px !important;
}

.profile-text h2 {
  margin: 0 0 4px !important;
  color: var(--ai-ink) !important;
  font-size: 22px !important;
}

.profile-text p {
  margin: 0 !important;
  color: var(--ai-muted) !important;
  font-size: 13px;
  word-break: break-all;
}

.form-container {
  margin-top: 8px !important;
}

.content-card,
.content-cardss {
  width: 100% !important;
  height: auto !important;
  max-height: none !important;
  margin: 0 0 12px !important;
  padding: 0 !important;
  border: 1px solid var(--ai-line) !important;
  border-radius: 18px !important;
  background: #fff !important;
  color: var(--ai-ink) !important;
  display: block !important;
  box-shadow: var(--ai-shadow-sm);
}

.content-card :deep(.el-card__body),
.content-cardss :deep(.el-card__body) {
  padding: 16px !important;
}

.content-card .el-form-item__label,
.content-cardss .el-form-item__label {
  color: var(--ai-ink-soft) !important;
  font-weight: 600;
}

.content-card :deep(.el-input),
.content-cardss :deep(.el-input) {
  width: 100% !important;
}

.content-card :deep(.el-input__wrapper),
.content-cardss :deep(.el-input__wrapper) {
  width: 100% !important;
}

.el-button {
  margin-top: 8px !important;
}

.footer {
  width: min(1180px, calc(100% - 40px)) !important;
  margin: 18px auto 0 !important;
  padding: 18px 0 !important;
  border-top: 1px solid var(--ai-line) !important;
  border-bottom: 0 !important;
  background: transparent !important;
  box-shadow: none !important;
  color: var(--ai-muted) !important;
  flex-wrap: wrap;
  gap: 14px;
}

.footer .navigation a,
.footer .navigation ul li a {
  color: var(--ai-ink-soft) !important;
  font-size: 12px;
}

.footer .slogan {
  color: var(--ai-ink-soft) !important;
  font-size: 13px !important;
  font-weight: 600;
}

.footer .copyright,
.footer .legal-links {
  color: var(--ai-muted) !important;
  font-size: 11px !important;
}

.qrcode img {
  width: 44px !important;
  height: 44px !important;
  margin-left: 10px !important;
  border-radius: 6px;
}

@media (max-width: 960px) {
  .lishi-pel-container {
    grid-template-columns: 1fr;
  }

  .box {
    padding-top: 22px !important;
  }
}

@media (max-width: 640px) {
  .box {
    width: calc(100% - 24px);
    padding-top: 18px !important;
  }

  .pel {
    padding: 18px !important;
  }

  .footer {
    width: calc(100% - 24px) !important;
    flex-direction: column;
    align-items: flex-start;
  }

  .history-image-container {
    height: 120px !important;
  }

  .history-icon {
    height: 120px !important;
  }
}
</style>

