<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { useRouter } from 'vue-router';
import { ElAvatar } from 'element-plus';

// 引入本地图片（如果需要）
import image4 from '../assets/images/image4.png';
// import image2 from '../assets/images/image2.jpg';
import image22 from '../assets/images/image22.png';
import image7 from '../assets/images/image7.png';

// 定义响应式数据
const imageList = ref([image4, image7,  image22]);

// 定义数据结构
const imageData = ref([]);
const avatarUrl = ref(''); // 头像 URL

const token = localStorage.getItem('token');
// 假设 avatarUrl 是上传成功后获取的头像 URL

onMounted(() => {
  const storedAvatarUrl = localStorage.getItem('avatarUrl');
  if (storedAvatarUrl) {
    avatarUrl.value = storedAvatarUrl;
  }
  console.log(storedAvatarUrl,'获取头像');

});
// 从后端获取数据的方法
const fetchData = async () => {
  try {
    if (!token) {
      useRouter().push('/');  // 如果没有 token，跳转到登录页面
      console.error('未登录，请先登录');
      return;
    }

    const headers = {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'multipart/form-data'
    };

    const response = await axios.get('/users/Check/Images_Description', { headers });
    console.log('请求数据成功', response.data);

    if (response.data.code === 200) {
      imageData.value = response.data.data;
      // avatarUrl.value = response.data.data[0].avatar_url;
       // 从接口返回的数据中提取头像 URL

    } else {
      console.error('数据加载失败:', response.data);
    }
  } catch (error) {
    console.error('请求失败:', error);
  }
};

// 在组件挂载时调用fetchData
onMounted(() => {
  fetchData();
});
</script>


<template>
  <div class="top" style="position: fixed;">
    <!-- <div><h1 class="q">AI皮肤病医疗顾问</h1></div> -->
    <div><img src="../assets/t.png" alt="logo" class="logo1" /></div>
    <select @change="toggleLanguage" class="language-select" >
        <option value="zh" :selected="language === 'zh'" >中文版</option>
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

      <span class="icon-tu"></span>
      <RouterLink to="/home">
        <button class="btnLogin-popups">询问AI</button>
      </RouterLink>
      
      <RouterLink to="/Summarize">
        <el-avatar size="120" :src="avatarUrl" class="avatar">{{ avatarUrl }}</el-avatar>
      </RouterLink>
    </nav>

    <div  class="w">
          <RouterLink to="/PublicVersion" custom v-slot="{ navigate }">
            <span @click="navigate" role="link"  style="color: #b9eeea; font-size: 16px; margin-right: 5px; cursor: pointer;">公众版</span>
          </RouterLink>
          <RouterLink to="/FastVersion" custom v-slot="{ navigate }">
            <span @click="navigate" role="link" style="color: #107da8; font-size: 16px; margin-right: 5px; cursor: pointer;">|极速版</span>
          </RouterLink>
          <RouterLink to="/care" custom v-slot="{ navigate }">
            <span @click="navigate" role="link" style=" font-size: 16px; cursor: pointer;color: #107da8;">|关怀模式</span>
          </RouterLink>
        </div>
  </div>

  <div class="box3">
    <div class="home1">
      <el-carousel indicator-position="outside" :height="'70vh'" style="width: 100%;">
        <el-carousel-item v-for="(imgSrc, index) in imageList" :key="index">
          <img :src="imgSrc" alt="轮播图" class="carousel-image" />
        </el-carousel-item>
      </el-carousel>

      <div class="home_content">
        <div class="center">
          <span>
            <h1 style="padding: 20px;">AI皮肤病医疗顾问(AI Dermatology Medical Consultant)</h1>
            AI皮肤问诊是一种利用人工智能技术来辅助诊断皮肤问题的服务。<br>
            近年来，随着AI技术的发展，这类服务逐渐变得更加普及和成熟。<br>
            这类服务通常通过手机应用程序提供，用户只需上传几张皮肤病变部位的照片，并回答一些关于症状的基本问题，AI系统就能基于大量的医学数据和算法模型，为用户提供可能的诊断建议及相关信息。
            <br>
            <RouterLink to="/home">
              <button class="btn5">点击进入AI皮肤问诊</button>
            </RouterLink>
          </span><br>
        </div>
      </div>

      <div class="header1">
        <p style="font-size: 1.5em; font-weight: bold;">下方典型皮肤病参考</p>
      </div>

      <div class="introduce">
        <!-- 动态渲染从后端获取的数据 -->
        <div v-for="item in imageData" :key="item.id" class="introduce_item" :class="{'left-item': item.id % 2 !== 0, 'right-item': item.id % 2 === 0}">
          <img class="introduce_item_img" :src="item.image_url" alt="点击查看图片" />
          <span class="introduce_item_title">{{ item.description }}</span>
        </div>
      </div>
      
    </div>
  </div>

  <div class="header1">
    <div></div>
    <p style="font-size: 1.1em; font-weight: bold;"></p>
  </div>
</template>



<style scoped>
.w {
  position: fixed;
  top: 8px; /* 根据需要调整 */
  right: 20px; /* 根据需要调整 */
  z-index: 1000;
  font-size: 1.1em;
  color: #fff;
}
.navigations{
  display: flex;
  align-items: flex-start; /* 左对齐 */
  align-items: center;
  padding: 10px;

}

.navigations .btnLogin-popups{
  width:130px;
  height:50px;
  background:transparent;
  border:2px solid #fff;
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
  z-index: 999;
  
}

.navigations .btnLogin-popups:hover{
  background: #fff;
  color: #162938;


}

.navigations .icon-tu{
  position: absolute;
  margin-right: 5vw;
  font-size: 1.7em;
  color: #fff;
  line-height: 57px;
 
  
}
.language-select option {
  color: #000;
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

.language-select::after{
  border:1px solid rgb(255, 255, 255);
  color:black;
}

.q{
  color: white;
}

.top{
  width: 100%;
  padding: 20px 100px;
z-index:998;
  /* background: linear-gradient(90deg, #4da8b7 40%, #55ceb2e4); */
  background-color: #12405d;
  display: flex;
  justify-content: space-between;
  max-height: 20vh;
  height: 12vh;
  box-shadow: 0 2px 0 0 rgb(255, 255, 255); /* 添加底部白色边框效果 */
}

.avatar {
  border-radius: 50%; /* 头像为圆形 */
  z-index: 1001;
  width: 3.5vw;
  height: 5.7vh;
  margin-left:60px ;
  margin-top: 20px;
}

.home1 {
    width: 100%;
    height: auto;
}

.header1{
  width: 100%;
  padding: 20px 100px;
/* background-color:rgba(92, 90, 90, 0.7); */
z-index:1000;
  /* background: linear-gradient(90deg, #4da8b7 40%, #55ceb2e4); */
  background-color: #12405d;
  display: flex;
  justify-content: space-between;
  align-items: center;
  max-height: 10vh;
  height: 10vh;
}


.carousel-image {
  width: 100%; /* 确保图片宽度占满 */
  height: 100%; /* 确保图片高度占满 */
  object-fit:cover; /* 裁剪图片以填充容器，同时保持图片比例 */
  border-radius: 0; /* 去掉图片的圆角 */
  box-shadow: none; /* 去掉阴影（可选） */
}

.el-carousel {
  width: 100%; /* 使轮播容器宽度占满 */
  height: 100%; /* 使轮播容器高度占满父级 */
  
}


.box4{
  width:100vw;
  top: 10px;
  position: relative;

}

.botton{
  border: 0;
  border-radius: 6px;
  color: rgb(16, 16, 16);
  padding: 15px 20px;
  margin: 20px;
  width:100px;
  height:50px;
  background:transparent;
  text-transform: uppercase;
  border:1px solid #fff;
  background: linear-gradient(to right, #6c7171 50%, rgba(255,255,255,0.9) 50%);
  background-size: 200% 100%;
  background-position: right bottom;
  transition: all 0.5s ease;
  cursor: pointer;
}

.el-carousel-item {
  width: 100%; /* 宽度占满横框 */
  height: 100%; /* 高度占满父级高度 */
  display: flex;
  align-items: center;
  justify-content: center;
}
button:hover{
  background-position: left bottom;
 
  color:#fff;
}


.home_content{
align-items: center;
justify-content: center;
display: flex;
flex-direction: column;
background-color: #dcf1e1;
display: flex;
justify-content: center;
align-items: center;
flex-direction: column;
font-size: 20px;
border: none;
color: rgb(39, 38, 26);
overflow: hidden;
box-sizing: border-box;
word-wrap: break-word; /* 防止文字溢出 */
height: 40vh;

}

.center{
  width: 55vw;
  height:auto;
}

.btn5{
  border: none;
  background-color: #829999;
  padding: 10px;
  color: #fff;
  border-radius: 10px;
}

.btn5:hover{
  background-color: #62a4a4;
  
}


.introduce {
  display: flex;
  flex-wrap: wrap; /* Allow items to wrap into a new line */
  gap: 20px;
  /* background-color:#dcf1e1; */
}

.introduce_item {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 20px;
  width: 100%; /* Adjust width to make them responsive */
  background-color: rgba(203, 198, 184, 0.301);
  border: 10px solid rgba(172, 190, 206, 0.2);
  border-radius: 10px;
}

.introduce_item.left-item {
  justify-content: flex-start; /* Align items with odd id to the left */
}

.introduce_item.right-item {
  justify-content: flex-end; /* Align items with even id to the right */
}

.introduce_item_img {
  width: 20vw;
  height: 30vh;
  object-fit: cover;
  /* background-color: #bdbdbd; */
}

.introduce_item_title {
  font-size: 1.3em;
  color: #333;
  background-color: rgba(255, 255, 255, 0.6);
  padding: 10px;
  border-radius: 10px;
  width: 60%;
}

</style>