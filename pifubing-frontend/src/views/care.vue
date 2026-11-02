<template>
  <div class="container5">
    <header style="position: fixed;">
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
        <RouterLink to="/HomeView">
          <button class="btnLogin-popups">登录</button>
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
      <!-- <h1 class="i" style="padding-top: 20vh;">
        <i>HELLO</i>
      </h1>
      <h1 class="h">
        <p>AI让健康预见未来，而我们的使命，是让每一寸肌肤都被精准守护<br>
         Skin AI. Where Science Meets Humanity.</p>
      </h1> -->
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
  background-color: #12405d;
  display: flex;
  justify-content: space-between;
  max-height: 20vh;
  height: 12vh;
  box-shadow: 0 2px 0 0 rgb(255, 255, 255); /* 添加底部白色边框效果 */
  position: fixed; /* 固定在顶部 */
  top: 0;
  left: 0;
}

.language-select {
  background-color: transparent;
  border: 1px solid rgb(255, 255, 255);
  border-radius: 8px;
  color: #e8ecf0;
  font-size: 1.1em;
  cursor: pointer;
  position: absolute;
  left: 420px; /* 根据需要调整 */
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
  position: relative; /* 使得视频和文字可以重叠 */
  height: calc(100vh - 12vh); /* 减去 header 的高度 */
  overflow: hidden;
  background-image: url('../assets/关怀.png');
  background-repeat: no-repeat;
  background-size: cover;
  background-position: center;
  margin-top: 12vh; /* 与 header 的高度相同 */
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
  position: relative; /* 确保子元素可以相对于容器定位 */
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
@media screen and (max-width: 1200px) {
  .language-select {
    margin-left: -100px; /* 调整适合小屏幕的值 */
  }
}
</style>