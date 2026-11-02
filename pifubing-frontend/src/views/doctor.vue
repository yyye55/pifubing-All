<template>
    <div class="container5">
      <header style="position: fixed;">
        <!-- 导航栏内容 -->
        <div><img src="../assets/t.png" alt="logo" class="logo1" /></div>
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
  
      <div class="box4">
        <div class="skin-diseases">
          <RouterLink to="/Picture" class="disease-items">
            <div class="ring-container">
              <img src="../assets/图片3.png" alt="玫瑰糠疹" />
            </div>
            <h3>玫瑰糠疹</h3>
          </RouterLink>
          <RouterLink to="/Picture" class="disease-item2">
            <div class="ring-container">
              <img src="../assets/图片4.png" alt="体癣" />
            </div>
            <h3>体癣</h3>
          </RouterLink>
          <RouterLink to="/Picture" class="disease-item3">
            <div class="ring-container">
              <img src="../assets/图片2.png" alt="毛细血管扩张" />
            </div>
            <h3>毛细血管扩张</h3>
          </RouterLink>
          <RouterLink to="/Picture" class="disease-item4">
            <div class="ring-container">
              <img src="../assets/图片1.png" alt="荨麻疹" />
            </div>
            <h3>荨麻疹</h3>
          </RouterLink>
        </div>
        <div class="additional-image">
    <img src="../assets/球s.png" alt="新增图片" style="display: block; margin: 0 auto;" />
  </div>
      </div>
      
    </div>
  </template>
  <script setup>
  import { ref } from 'vue';
  import { useRouter } from 'vue-router';
  import axios from 'axios';
  
  const router = useRouter();
  const language = ref('zh');
  
  const toggleLanguage = (event) => {
    language.value = event.target.value;
    // 你可以在这里添加其他语言切换逻辑
  };
  </script>
  
  <style scoped>
 .additional-image {
  display: flex;
  justify-content: center;
  align-items: center;
  margin-top: 20px; /* 根据需要调整 */
}

.additional-image img {
  width: 400px; /* 根据需要调整 */
  height: 400px; /* 根据需要调整 */
  object-fit: cover;
  border-radius: 50%;
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
    z-index: 998;
   /* background: linear-gradient(90deg, #4da8b7 40% ,#55ceb2e4); */
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
  
  .h {
    text-align: center;
    color: #f6eded;
    font-size: 2em;
  }
  
  .i {
    color: #efe7e7;
    font-size: 15em;
    text-align: center;
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
  
  #color {
    color: #162938;
  }
  
.box4 {
  position: relative;
  height: 100vh;
  overflow: hidden;
  background-image: url('../assets/首页.png');
  background-repeat: no-repeat;
  background-size: cover;
  background-position: center;
}

.skin-diseases {
  display: flex;
  justify-content: space-around;
  align-items: center;
  padding-top: 150px;
  z-index: 1;
}


.disease-items {
  text-align: center;
  color: inherit; /* 确保颜色继承 */
  margin-top: 30vh;
  z-index: 2;
}
.disease-items img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease; /* 添加过渡效果 */
}

.disease-items:hover img {
  transform: scale(1.1); /* 放大到原来的1.1倍 */
}

.disease-items h3 {
  color: #fff;
  font-size: 1.5em;
  margin-top: 10px;
}

.disease-items:hover {
  text-decoration: none; /* 移除默认链接下划线 */
}




.disease-item2 {
  text-align: center;
  color: inherit; /* 确保颜色继承 */
  margin-top: -25vh;
  z-index: 3;
}
.disease-item2 img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease; /* 添加过渡效果 */
}

.disease-item2:hover img {
  transform: scale(1.1); /* 放大到原来的1.1倍 */
}

.disease-item2 h3 {
  color: #fff;
  font-size: 1.5em;
  margin-top: 10px;
}

.disease-item2:hover {
  text-decoration: none; /* 移除默认链接下划线 */
}



.disease-item3 {
  text-align: center;
  color: inherit; /* 确保颜色继承 */
  margin-top: -25vh;
  z-index: 4;
}
.disease-item3 img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease; /* 添加过渡效果 */
}

.disease-item3:hover img {
  transform: scale(1.1); /* 放大到原来的1.1倍 */
}

.disease-item3 h3 {
  color: #fff;
  font-size: 1.5em;
  margin-top: 10px;
}

.disease-item3:hover {
  text-decoration: none; /* 移除默认链接下划线 */
}



.disease-item4 {
  text-align: center;
  color: inherit; /* 确保颜色继承 */
  margin-top: 40vh;
  z-index: 5;
}
.disease-item4 img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease; /* 添加过渡效果 */
}

.disease-item4:hover img {
  transform: scale(1.1); /* 放大到原来的1.1倍 */
}

.disease-item4 h3 {
  color: #fff;
  font-size: 1.5em;
  margin-top: 10px;
}

.disease-item4:hover {
  text-decoration: none; /* 移除默认链接下划线 */
}

.ring-container {
  position: relative;
  width: 200px;
  height: 200px;
  border-radius: 50%;
  overflow: hidden;
}

.ring-container::before {
  content: '';
  position: absolute;
  top: 5px;
  left: 5px;
  right: 5px;
  bottom: 5px;
  border: 2px solid rgba(255, 255, 255, 0.8);
  border-radius: 50%;
  z-index: -1;
}

.disease-item img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.3s ease; /* 添加过渡效果 */
}

.disease-item:hover img {
  transform: scale(1.1); /* 放大到原来的1.1倍 */
}

.disease-item h3 {
  color: #fff;
  font-size: 1.5em;
  margin-top: 10px;
}

.disease-item:hover {
  text-decoration: none; /* 移除默认链接下划线 */
}

@media screen and (max-width: 768px) {
  .skin-diseases {
    flex-direction: column;
    align-items: center;
  }

  .disease-item {
    margin-bottom: 20px;
  }
}

  
  .box4 video {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
    z-index: -1;
  }
  
  .container5 {
    height: 100vh;
  }
  
  .b1 {
    border: 0;
    border-radius: 6px;
    color: rgb(16, 16, 16);
    padding: 15px 20px;
    margin: 20px;
    width: 100px;
    height: 50px;
    background: transparent;
    text-transform: uppercase;
    border: 1px solid #fff;
    background: linear-gradient(to right, #6c7171 50%, rgba(255, 255, 255, 0.5) 50%);
    background-size: 200% 100%;
    background-position: right bottom;
    transition: all 0.5s ease;
    cursor: pointer;
  }
  
  .b1:hover {
    background-position: left bottom;
    color: #fff;
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
  </style>