<script>
import { ref } from 'vue';
import axios from 'axios';

export default {
  data() {
    return {
      activities: [], // 用来存储从后端获取的活动数据
      contents: "", // 动态从后端获取的内容
      isSpeaking: false,
      isPaused: false,
      utterance: null,
      // 响应式状态
      isAnalyzing: ref(false),
      imageAnalysisResult: ref(null),
      selectedImage: ref(null),
      imageAnalysisClass: ref(null),
      isChatting: ref(false),
      userMessage: ref(''),
      chatResult: ref(null),
      messages: ref([]), // 新增的消息数组
      askmessages: ref(''), // 新增的消息数组
      isChatting: false, // 是否正在聊天
    };
  },
  methods: {
    // 获取内容的接口方法
    async fetchContents() {
      try {
        const response = await fetch("http://localhost:9094/ceshi"); // 后端接口地址
        const data = await response.json(); // 解析 JSON 数据

        // 如果 data 是字符串，转为数组；如果是数组，直接赋值
        this.contents = typeof data.data === "string" ? [data.data] : data.data.contents;
        
        // 通过接口获取时间线数据并更新
        this.activities = data.data.activities || [];  // 假设接口返回的活动数据包含在 `activities` 字段中
      } catch (error) {
        console.error("获取内容失败：", error);
        alert("无法获取内容，请稍后再试！");
      }

      // 在 mounted 生命周期钩子中加载 localStorage 中的状态
      this.isSpeaking = JSON.parse(localStorage.getItem('isSpeaking')) || false;
      this.isPaused = JSON.parse(localStorage.getItem('isPaused')) || false;
    },

    // 朗读内容方法
    readContent(text) {
      if ("speechSynthesis" in window) {
        // 创建一个新的 SpeechSynthesisUtterance 对象，每次朗读都使用新的实例
        const utterance = new SpeechSynthesisUtterance(text);
        utterance.lang = "zh-CN";
        utterance.pitch = 1.6;
        utterance.rate = 1.1;
        utterance.volume = 1;

        // 如果当前朗读正在进行并且没有暂停，则暂停当前朗读
        if (this.isSpeaking && !this.isPaused) {
          window.speechSynthesis.pause();
          this.isPaused = true;
          localStorage.setItem('isPaused', JSON.stringify(true));  // 保存暂停状态
        }

        // 如果朗读已暂停，恢复朗读
        else if (this.isPaused) {
          window.speechSynthesis.resume();
          this.isPaused = false;
          localStorage.setItem('isPaused', JSON.stringify(false));  // 恢复朗读状态
        }

        // 否则，开始新的朗读
        else {
          this.isSpeaking = true;
          localStorage.setItem('isSpeaking', JSON.stringify(true));  // 保存朗读状态
          localStorage.setItem('isPaused', JSON.stringify(false));  // 确保没有暂停

          // 监听朗读结束事件，更新状态
          utterance.onend = () => {
            this.isSpeaking = false;
            this.isPaused = false;
            localStorage.setItem('isSpeaking', JSON.stringify(false));  // 朗读结束，更新状态
            localStorage.setItem('isPaused', JSON.stringify(false));   // 朗读结束，更新暂停状态
          };

          // 开始朗读
          window.speechSynthesis.speak(utterance);
        }
      } else {
        alert("您的浏览器不支持语音朗读功能！");
      }
    },

    // 停止所有朗读任务
    stopAllReadings() {
      // 停止所有正在进行的朗读
      window.speechSynthesis.cancel();
      this.isSpeaking = false;
      this.isPaused = false;
      localStorage.setItem('isSpeaking', JSON.stringify(false));
      localStorage.setItem('isPaused', JSON.stringify(false));
    },

    // 修改后的 startVoiceSearch 方法
    startVoiceSearch() {
      if (!('webkitSpeechRecognition' in window)) {
        alert('您的浏览器不支持语音识别功能，请使用 Chrome 浏览器。');
        return;
      }

      // 创建语音识别实例
      const recognition = new webkitSpeechRecognition();
      recognition.lang = 'zh-CN'; // 设置语言为中文
      recognition.interimResults = false; // 是否获取实时结果
      recognition.continuous = false; // 是否连续识别

      // 开始识别
      recognition.start();

      // 识别到结果
      recognition.onresult = (event) => {
        const speechResult = event.results[0][0].transcript;
        this.userMessage = speechResult; // 将识别结果存储到 userMessage 中
      };

      // 识别出错
      recognition.onerror = (event) => {
        console.error('语音识别错误:', event.error);
        alert('语音识别出错，请重试！');
      };

      // 识别结束
      recognition.onend = () => {
        console.log('语音识别已结束');
      };
    },

    // 图片选择事件
    onFileChange(event) {
      const file = event.target.files[0];
      if (file) {
        this.selectedImage = file;
      }
    },

    // 图片上传与分析
    async handleImageUpload(event) {
      event.preventDefault(); // 阻止表单默认提交行为

      if (!this.selectedImage) {
        alert('请先选择图片');
        return;
      }

      const token = localStorage.getItem('token');
      const formData = new FormData();
      formData.append('file', this.selectedImage);

      try {
        this.isAnalyzing = true;
        const response = await axios.post(`/users/image/ask`, formData, {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

        if (response.data.code === 200) {
          this.imageAnalysisResult = response.data.GeneratedText;
          this.imageAnalysisClass = response.data.PredictedClass;
        } else {
          alert('上传失败');
        }
      } catch (error) {
        console.error('图片分析失败:', error);
        alert('图片分析失败，请稍后再试');
      } finally {
        this.isAnalyzing = false;
      }
    },

    // 发送问题到 Qwen 模型
   // 发送问题到 Qwen 模型
async handleChat(event) {
  event.preventDefault();  // 阻止默认行为，以免页面跳转

  const editableContent = document.querySelector('[contenteditable="true"]');
  const content = editableContent.innerHTML.trim();

  // 检查内容是否为空或者仅包含图片
  const containsOnlyImage = !content || (content.includes('<img') && !content.replace(/<img[^>]*>/g, '').trim());

  // 如果包含图片或非空内容，才可以继续
  if (!containsOnlyImage && !content) {
    alert('请添加文字或图片内容');
    return;
  }

  // 用户提问，添加到消息数组
  this.messages.push({
    sender: 'user',
    text: content,
    side: 'right',
    image: content.includes('<img src=') ? content : null // 判断是否包含图片
  });

  const token = localStorage.getItem('token');
  try {
    this.isChatting = true; // 正在发送请求
    const response = await axios.post(
      `/users/ask`,
      { ask: content },
      { headers: { Authorization: `Bearer ${token}` } }
    );
    if (response.data.code === 200) {
      const answer = response.data.response;
      this.messages.push({
        sender: 'qwen',
        text: answer,
        side: 'left',
        image: null
      });
    } else {
      alert('上传失败');
    }
  } catch (error) {
    console.error('聊天失败:', error);
    alert('聊天失败，请稍后再试');
  } finally {
    this.isChatting = false; // 发送完毕，恢复按钮状态
    editableContent.innerHTML = ''; // 清空编辑框
  }
},



  

    // 处理图片粘贴
    handlePaste(event) {
  const items = event.clipboardData.items;
  for (let i = 0; i < items.length; i++) {
    const item = items[i];
    if (item.type.indexOf('image') !== -1) {
      const file = item.getAsFile();
      const reader = new FileReader();
      reader.onloadend = function () {
        const imageUrl = reader.result;
        console.log(imageUrl);  // 输出图片的 Base64 编码 URL
        // 在编辑框内插入图片
        const img = document.createElement('img');
        img.src = imageUrl;
        img.style.maxWidth = '100%'; // 确保图片最大宽度为100%
        img.style.height = 'auto';  // 高度自适应
        img.style.borderRadius = '10px';
        img.style.display = 'block';  // 确保图片是块级元素
        img.style.margin = '10px 0';  // 给图片上下添加间距
        img.style.maxHeight = '300px';
        const editableContent = document.querySelector('[contenteditable="true"]');
        editableContent.appendChild(img);
      };
      reader.readAsDataURL(file);  // 将图像转换为 base64 编码，并插入
      break;
    }
  }
    },
  },

};
</script>



<template>
  <header style="position: fixed;">
    <div><img src="../assets/logo5.png" alt="logo" class="logo1"></div>
    <nav class="navigation2">
      <RouterLink to="/Picture">
        <span class="icon-tu">
          <ion-icon name="chevron-back-circle-outline"></ion-icon>
        </span>
      </RouterLink>
    </nav>
  </header>

  <main>
    <div class="container1">
  <!-- 图片分类部分 -->
  <!-- <div id="image-classification" class="upload-container">
    <h2>皮肤疾病图像分类</h2>
    <form @submit="handleImageUpload">
      <div class="file-upload-box">
        <p v-if="!selectedImage">点击或拖拽图片到此处上传</p>
        <p v-else>已选择图片：{{ selectedImage.value?.name }}</p>
        <input type="file" name="image" accept="image/*" @change="onFileChange" class="hidden-input" />
      </div>
      <button type="submit" :disabled="isAnalyzing" class="btn2">
        {{ isAnalyzing ? '分析中...' : '上传图片并分析' }}
      </button>
    </form>

    <div v-if="imageAnalysisResult" class="result-box" @click="readContent(imageAnalysisResult)">
  <p style="font-size: 35px; font-weight: bold;">预测类型：{{ imageAnalysisClass }}</p>
  <div style="font-size: 20px; line-height: 1.5;">
    <p v-for="(paragraph, index) in imageAnalysisResult.split('\\n')" :key="index">
      {{ paragraph }}
    </p>
  </div>
</div>
  </div> -->

  <!-- 聊天部分 -->
<!-- 聊天部分 -->
<div id="qwen-chat" class="chat-container">
  <h2>与 Qwen 模型对话</h2>

  <div class="box5">
    <div class="chat-messages">
      <!-- 显示聊天消息 -->
<div v-for="(message, index) in messages" :key="index" :class="['chat-message', message.side]" @click="readContent(message.text)">
  <!-- 如果消息中包含图片，就显示图文混排 -->
  <div v-if="message.image">
    <div v-html="message.text"></div>  <!-- 显示带图片的 HTML 内容 -->
  </div>
  <p v-if="!message.image">{{ message.text }}</p> <!-- 如果没有图片，显示文本 -->
</div>


    </div>
  </div>

  <div class="anniu">
    <form @submit="handleChat" class="chat-form">
      <div 
  contenteditable="true" 
  @input="updateUserMessage"
  placeholder="请输入您的问题" 
  class="textarea-wen" 
  @paste="handlePaste"
  ref="editableContent"
  v-html="userMessage"  
  @keydown.enter="handleChat"
>
 </div>

 <button type="submit" :disabled="isChatting" class="btn-inside">
  <span class="icon-tu">
    <!-- 当正在聊天时，显示一个发送中的图标 -->
    <ion-icon :name="isChatting ? 'reload-circle' : 'arrow-redo'"></ion-icon>  <!-- 动态切换图标 -->
  </span>
  <!-- 当 isChatting 为 true，显示“发送中...” -->
  <span v-if="isChatting">{{ '发送中...' }}</span>
</button>

      <div class="voice-btn" @click="startVoiceSearch">
        🎤
      </div>
    </form>
  </div>
</div>








</div>
  </main>

  <bottom>
    <nav class="">
      <p class="zi"></p>
    </nav>
  </bottom>
</template>





<style scoped>
/* 调整语音按钮样式 */

.chat-form {
  display: flex;
  align-items: center;  /* 垂直居中对齐 */
  gap: 10px; /* 输入框与按钮之间的间距 */
  width: 100%;
}


.voice-btn {
 
  color: rgb(13, 12, 12);
  border: none;
  border-radius: 5px;
  padding: 10px;
  font-size: 1.2em;  /* 提高按钮图标的大小 */
  cursor: pointer;
  margin-bottom: 10px;
  position: absolute;
  top: 60%;
  left:0;
 
}

.chat-image {
  width: 50%; /* 可以调整图片的宽度 */
  height: auto;
  border-radius: 10px;
  display: block; /* 使图片成为块级元素 */
  margin: 10px auto; /* 设置上下间距，自动居中 */
}



.textarea-wen {
  width: 100%;
  height: auto;
  padding: 10px;
  font-size: 1.1em;
  border: 1px solid #ccc;
  border-radius: 5px;
  resize: vertical;
  background-color: #ffffff97;
  margin-top: 10px;
  word-wrap: break-word; /* 允许换行 */
}

.textarea-wen img {
  max-width: 100%;  /* 保证图片宽度不会超过输入框宽度 */
  height: auto;
  border-radius: 10px;
  margin: 10px 0;  /* 上下间距 */
  display: block;  /* 让图片成为块级元素，避免与文本混排 */
}



.chat-container {
  width: 50vw;
  max-width: 55vw;
  height: 80vh;
  margin: 0 auto;
  background-color: rgba(255, 255, 255, 0.3);
  border-radius: 10px;
  padding: 20px;
}


.container1 {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  gap: 20px;
  padding: 20px;
  max-width: 100%;
  padding-top: 11vh;
}

/* .upload-container,
.chat-container {
  width: 100%;
  padding: 20px;
  background-color: rgba(255, 255, 255, 0.349);
  border-radius: 10px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
  border: 1px solid #ccc;
  height: 80vh;
} */





/* 输入框 */
.input-wen {
  width: calc(100% - 90px);
  padding: 10px;
  margin-right: 10px;
  border: 1px solid #ccc;
  border-radius: 5px;
  width: 10vw;
  border:none;
  font-size: 1.1em;
  height: 10vh;
  width:100%;
  padding: 10px;
  border: 1px solid #ccc;
  border-radius: 5px;
}

.input-wen:focus{
  border-color: #fff;
  outline: none; /* 移除默认的轮廓线 */
  box-shadow: 0 0 5px #fff;

}



body{
  /* display: flex;
  justify-content: center;
  align-items: center; */
  min-height: 100vh;
  background: url('../assets/bei1.png')no-repeat;
  background-size: cover;
  background-position: center;
 
} 
/* 主容器样式 */




/* 标题样式 */
h1 {
  font-size: 4em;
  color: #162938;
  margin-bottom: 20px;
}

h2 {
  font-size: 1.5em;
  margin-bottom: 10px;
  color: #333;
}

/* 表单样式 */
input[type="text"],
input[type="file"],
input[type="submit"] {
  width: 100%;
  margin-top: 10px;
  padding: 10px;
  border-radius: 5px;
  border: 1px solid #ccc;
  background-color:rgba(255, 255, 255, 0.6);
}

/* 提交按钮 */
input[type="submit"] {
  background-color: #162938;
  color: white;
  cursor: pointer;
  border: none;
}

input[type="submit"]:hover {
  background-color: #333;
  color: white;
}





/* 内容列表样式 */
.content-item {
  cursor: pointer;
  padding: 10px;
  border: 1px solid #ddd;
  margin: 5px 0;
  background-color: #f9f9f9;
  transition: background-color 0.3s;
}
.content-item:hover {
  background-color: #e9e9e9;
}


.chat-messages {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 20px;
}

.chat-message {
  max-width: 70%;
  padding: 10px;
  border-radius: 15px;
  margin: 5px 0;
  font-size: 1.2em;
}

.chat-message.right {
  align-self: flex-end;
  background-color: #fff;
  color: rgb(15, 15, 15);

}

.chat-message.left {
  align-self: flex-start;
  background-color: #f1f1f1;
  color: #333;
}


.box5{
  padding: 10px;
  width:100%;
  height: 60vh;
  overflow: auto;
  background-color: rgba(255, 255, 255, 0.3);
  border-radius: 10px;
}

/* 输入容器样式 */
.input-container {
  display: flex;
  flex-direction: column; /* 纵向排列 */
  align-items: flex-end; /* 按钮靠右对齐 */
  gap: 10px; /* 添加间距 */

}

/* 文本框样式 */
.textarea-wen {
  width: 100%; /* 占满容器宽度 */
  height: 10vh;
  max-height: 15vh;
  overflow-y: auto;
  padding: 10px;
  font-size: 1.1em;
  border: 1px solid #ccc;
  border-radius: 5px;
  resize: vertical; /* 允许用户调整高度 */
}

.textarea-wen:focus {
  border-color: #007bff;
  outline: none;
  box-shadow: 0 0 5px rgba(0, 123, 255, 0.5); /* 添加聚焦效果 */
}


.anniu{
  position:relative;
  width: 100%;
  
}
.btn-inside {
  position: absolute;
  top: 50%; /* 垂直居中 */
  right: 15px; /* 靠右 */
  transform: translateY(-50%);
  background-color: #162938;
  color: #fff;
  border: none;
  border-radius: 5px;
  padding: 5px;
  cursor: pointer;
  font-size: 1.1em;
  width:auo;
  

}

.btn-inside:disabled {
  background-color: #aaa;
  cursor: not-allowed;
}
</style>


