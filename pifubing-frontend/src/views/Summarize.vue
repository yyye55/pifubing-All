<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElAvatar, ElRow, ElCol, ElCard, ElForm, ElFormItem, ElInput, ElButton,ElDialog } from 'element-plus';

import { useRouter } from 'vue-router';


// 获取 token
const token = localStorage.getItem('token');
// 上传头像后保存到 LocalStorage


// 初始化 avatarUrl 和 backgroundImage
const avatarUrl = ref('');
const backgroundImage = ref('');
//const backgroundImage = ref(require('../assets/images'));  // 使用 require 来加载本地图片

const alertMessage = ref('');  // 提示框内容
const showAlert = ref(false);  // 控制提示框显示与否

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
     // showAlertBox2
      ('个人信息更新成功');
      // 更新成功后重新加载用户信息
      loadUserData();  // calling the renamed function
    } else {
      //showAlertBox2
      ('更新失败：' + response.data.message);
    }
  } catch (error) {
    console.error('提交失败', error);
    showAlertBox2('提交失败');
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
      showAlertBox2('病史和过敏史更新成功');
      // 更新成功后重新加载用户信息
      loadUserData();  // calling the renamed function
    } else {
      showAlertBox2('更新失败：' + response.data.message);
    }
  } catch (error) {
    console.error('提交病史和过敏史失败', error);
    showAlertBox2('提交失败，请稍后再试');
  }
};

// showAlertBox2 代码
const showAlertBox2 = (message) => {
  alertMessage.value = message;
  showAlert.value = true;
  setTimeout(() => {
    showAlert.value = false; // 3秒后自动隐藏弹框
  }, 3000);
};



// 处理头像上传
const handleAvatarChange = async (event) => {
  const file = event.target.files[0]; // 获取用户选择的文件

  if (file) {
    // const newAvatarUrl = URL.createObjectURL(file); // 创建临时URL
    // avatarUrl.value = newAvatarUrl;  // 更新头像 URL

    // 保存头像 URL 到 localStorage
    // localStorage.setItem('avatarUrl', newAvatarUrl);

    // 如果需要进行上传至服务器的操作，可以如下发送请求
    const formData = new FormData();
    formData.append('avatar', file);  // 将文件添加到表单数据中

    try {
      if (!token) {
        useRouter().push('/');  // 如果没有 token，跳转到登录页面
        console.error('未登录，请先登录');
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
        showAlertBox2('头像上传成功！');
        console.log('头像更新成功',response.data.data);
      } else {
        console.error('头像更新失败:', response.data.message);
        showAlertBox2('头像更新失败：' + response.data.message);
      }
    } catch (error) {
      console.error('请求失败', error);
      showAlertBox2('上传失败，请稍后再试');
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
    <header style="position: fixed; top: 0; left: 0; width: 100%; z-index: 1000;  display: flex; justify-content: space-between; padding: 20px 100px; box-shadow: 0 2px 0 0 rgb(255, 255, 255);">
      <div>
        <img src="../assets/t.png" alt="logo" class="logo1" />
      </div>
      <select @change="toggleLanguage" class="language-select">
        <option value="zh" :selected="language === 'zh'">中文版</option>
        <option value="en" :selected="language === 'en'">English</option>
      </select>
      <nav class="navigations">
        <RouterLink to="/Picture">
          <button class="btnLogin-popups">首页</button>
        </RouterLink>
        <RouterLink to="/MedicalServices">
          <button class="btnLogin-popups">医疗服务</button>
        </RouterLink>
        <RouterLink to="/AboutUs">
          <button class="btnLogin-popups">关于我们</button>
        </RouterLink>
        <RouterLink to="/home">
          <button class="btnLogin-popups">询问AI</button>
        </RouterLink>
      </nav>
      <div class="w">
        <RouterLink to="/PublicVersion" custom v-slot="{ navigate }">
          <span @click="navigate" role="link" style="color: #b9eeea; font-size: 16px; margin-right: 5px; cursor: pointer;">公众版</span>
        </RouterLink>
        <RouterLink to="/FastVersion" custom v-slot="{ navigate }">
          <span @click="navigate" role="link" style="color: #107da8; font-size: 16px; margin-right: 5px; cursor: pointer;">|极速版</span>
        </RouterLink>
        <RouterLink to="/care" custom v-slot="{ navigate }">
          <span @click="navigate" role="link" style="font-size: 16px; cursor: pointer;color: #107da8;">|关怀模式</span>
        </RouterLink>
      </div>
    </header>

    <RouterLink to="/Picture">
      <span class="icon-tu">
        <ion-icon name="arrow-back-outline"></ion-icon>
      </span>
    </RouterLink>

    <div class="box">
  
  <div class="lishi-pel-container">
    

    <!-- 左侧：个人健康历史 -->
    <div class="lishi">
      <div class="history-image-container">
    <img src="../assets/yy.png" alt="健康历史图标" class="history-icon" />
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
    <!-- 底部导航 -->
    <footer class="footer">
      <nav class="navigation">
        <p><RouterLink to="/Picture">首页</RouterLink></p><br>
        <p><RouterLink to="/MedicalServices">医疗服务</RouterLink></p><br>
        <p><RouterLink to="/AboutUs">关于我们</RouterLink></p><br>
      </nav>
      <div class="slogan">
        “AI让健康预见未来，而我们的使命，是让每一寸肌肤都被精准守护”
      </div>
      <div class="qrcode">
        <img src="../assets/二维码/二维码.png" alt="二维码1" />
        <img src="../assets/二维码/二维码.png" alt="二维码2" />
      </div>
    </footer>
    <footer class="footer">
      <nav class="navigation">
        <ul>
          <li><RouterLink to="/sitemap">网站地图</RouterLink></li>
          <li><RouterLink to="/legal">法律声明</RouterLink></li>
          <li><RouterLink to="/contact">联系我们</RouterLink></li>
        </ul>
      </nav>
      <div class="copyright">
        <p>侵权申诉 | 隐私条约 | 网站建设</p>
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

.alert-box {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  background: #30b8d094;
  color: #fff;
  padding: 15px 20px;
  border-radius: 5px;
  box-shadow: 0 2px 5px rgba(0, 0, 0, 0.2);
  z-index: 1000;
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

