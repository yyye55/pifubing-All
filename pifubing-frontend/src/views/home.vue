<script>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useRouter } from 'vue-router';
import { marked } from 'marked';
import MyComponent from '../components/history.vue';
import { API_BASE } from '../apiBase';

// 定义全局变量
window.globalState = {
  showResponse: false
};

const storedAvatarUrl = localStorage.getItem('avatarUrl');
const avatarUrl = ref(''); // 头像 URL
const token = localStorage.getItem('token');
const user_id = localStorage.getItem('user_id');
const chat_id = localStorage.getItem('newChatId') || localStorage.getItem('chat_id') || 0; // 优先读取 newChatId

onMounted(() => {
  const storedAvatarUrl = localStorage.getItem('avatarUrl');
  if (storedAvatarUrl) {
    avatarUrl.value = storedAvatarUrl;
  }

  console.log(storedAvatarUrl, '获取头像'); 
});

console.log('获取头像111', storedAvatarUrl);

// 手动更新 DOM 的函数
function updateDOM() {
  const responseDiv = document.querySelector('.title-content.response');
  if (window.globalState.showResponse) {
    responseDiv.style.display = 'block'; // 显示元素
  } else {
    responseDiv.style.display = 'none'; // 隐藏元素
  }
}

export default {
  data() {
    return {
      activities: [],
      contents: "",
      isSpeaking: false,
      isPaused: false,
      utterance: null,
      isAnalyzing: ref(false),
      imageAnalysisResult: ref(null),
      selectedImage: ref(null),
      imageAnalysisClass: ref(null),
      isChatting: false,
      userMessage: ref(''), // 用户输入的文本
      messages: ref([]), // 消息数组
      rawText: ref(''), // 存储 Markdown 格式的文本
      chatId: chat_id, // 初始化 chatId
      userId: user_id ,// 初始化 userId
      chatMessages: [], // 当前聊天框的内容
    };
  },

  components: {
    MyComponent
  },

  computed: {
    formattedText() {
      return marked(this.rawText); // 解析 Markdown
    },
  },

  methods: {
    handleChatCreated(newChatId) {
      this.chatId = newChatId; // 更新 chatId
      localStorage.setItem('newChatId', newChatId); // 更新 localStorage 中的 chat_id
    },

    // 触发文件上传选择
    triggerFileUpload() {
      this.$refs.fileInput.click();
    },

    // 切换历史记录的显示状态
    selectHistoryItem(index) {
      this.selectedIndex = this.selectedIndex === index ? null : index;
    },

    parseMarkdown(text) {
      // 替换 Markdown 中的 # 符号为其他字符或空字符串
      const modifiedText = text.replace(/#/g, '');
      return marked(modifiedText, {
        breaks: true, // 自动换行
        sanitize: true // 过滤危险HTML
      });
    },

    // 弹出消息框
    open() {
      ElMessageBox.confirm(
        `<div>
           <label for="email">出现部位:</label>
           <input type="email" id="email" placeholder="请输入" style="width: 100%; margin-bottom: 10px; padding: 5px;  ">
           <label for="phone">持续时间:</label>
           <input type="tel" id="phone" placeholder="请输入" style="width: 100%; margin-bottom: 10px; padding: 5px; border: 1px solid #ccc;">
           <label for="address">伴随症状:</label>
           <input type="text" id="address" placeholder="请输入" style="width: 100%; padding: 5px; border: 1px solid #ccc;">
        </div>`,
        {
          title: '填写信息',
          confirmButtonText: '提交',
          cancelButtonText: '取消',
          dangerouslyUseHTMLString: true,  // 允许插入 HTML 内容
        }
      )
      .then(() => {
        // 提交逻辑，获取输入的值
        const email = document.getElementById('email').value;
        const phone = document.getElementById('phone').value;
        const address = document.getElementById('address').value;
        ElMessage({
          type: 'success',
          message: `提交成功 ${email} ${phone}${address}`,
        });
      })
      .catch(() => {
        ElMessage({
          type: 'info',
          message: '操作已取消',
        });
      });
    },

    // 触发文件选择框
    triggerImageUpload(event) {
      console.log('触发文件选择框 ');
      event.preventDefault();  // 阻止表单提交事件
      this.$refs.fileInput.click();  // 触发文件选择框
    },

    // 朗读内容方法
    readContent(text) {
      if ("speechSynthesis" in window) {
        const utterance = new SpeechSynthesisUtterance(text);
        utterance.lang = "zh-CN";
        utterance.pitch = 1.6;
        utterance.rate = 1.1;
        utterance.volume = 1;

        if (this.isSpeaking && !this.isPaused) {
          window.speechSynthesis.pause();
          this.isPaused = true;
          localStorage.setItem('isPaused', JSON.stringify(true));  // 保存暂停状态
        } else if (this.isPaused) {
          window.speechSynthesis.resume();
          this.isPaused = false;
          localStorage.setItem('isPaused', JSON.stringify(false));  // 恢复朗读状态
        } else {
          this.isSpeaking = true;
          localStorage.setItem('isSpeaking', JSON.stringify(true));  // 保存朗读状态
          localStorage.setItem('isPaused', JSON.stringify(false));  // 确保没有暂停

          utterance.onend = () => {
            this.isSpeaking = false;
            this.isPaused = false;
            localStorage.setItem('isSpeaking', JSON.stringify(false));  // 朗读结束，更新状态
            localStorage.setItem('isPaused', JSON.stringify(false));   // 朗读结束，更新暂停状态
          };

          window.speechSynthesis.speak(utterance);
        }
      } else {
        alert("您的浏览器不支持语音朗读功能！");
      }
    },

    // 停止所有朗读任务
    stopAllReadings() {
      window.speechSynthesis.cancel();
      this.isSpeaking = false;
      this.isPaused = false;
      localStorage.setItem('isSpeaking', JSON.stringify(false));
      localStorage.setItem('isPaused', JSON.stringify(false));
    },

    startVoiceSearch() {
      if (!('webkitSpeechRecognition' in window)) {
        alert('您的浏览器不支持语音识别功能，请使用 Chrome 浏览器。');
        return;
      }

      const recognition = new webkitSpeechRecognition();
      recognition.lang = 'zh-CN'; // 设置语言为中文
      recognition.interimResults = false; // 是否获取实时结果
      recognition.continuous = false; // 是否连续识别
      recognition.start();
      recognition.onresult = (event) => {
        const speechResult = event.results[0][0].transcript;
        this.userMessage = speechResult; // 将识别结果存储到 userMessage 中
      };
      recognition.onerror = (event) => {
        console.error('语音识别错误:', event.error);
        alert('语音识别出错，请重试！');
      };
      recognition.onend = () => {
        console.log('语音识别已结束');
      };
    },

    // 处理图片选择
    onFileChange(event) {
      const file = event.target.files[0];
      if (file) {
        this.selectedImage = file;
      }
    },

    // 图片和文本提交
    async handleImageAndChat(event) {
      event.preventDefault();
      const editableContent = document.querySelector('[contenteditable="true"]');
      const content = editableContent.innerHTML.trim();
      const textOnly = content.replace(/<img[^>]*>/g, '').trim();

      if (!this.selectedImage && !textOnly) {
        alert('请添加文字或图片内容');
        return;
      }

      let apiUrl, requestData;
      const formData = new FormData();

      if (this.selectedImage) {
        // 仅图片 或 文本 + 图片 → 仍然 POST 到 users/predict/image
        apiUrl = "/users/predict/image";

        formData.append("conversation", textOnly || "如何治疗？");
        formData.append("chat_id", this.chatId || 0);
        formData.append("user_id", this.userId || 0);
        formData.append("model", "DeepSeek-R1");

        if (this.selectedImage) {
          formData.append("file", this.selectedImage);
          console.log('formData', formData);  // 打印 formData 中的数据
        }
      } else {
        // 仅文本 → GET 请求到 only/text
        apiUrl = "/predict/only/text";
        requestData = {
          conversation: textOnly,
          chat_id: this.chatId,
          user_id: this.userId,
          model: "DeepSeek-R1",
        };
        console.log("仅文本请求参数：", requestData);
      }

      this.messages.push({
        sender: "user",
        text: textOnly || this.userMessage,
        image: this.selectedImage ? URL.createObjectURL(this.selectedImage) : null,
        side: "right",
      });
      this.userMessage = "";

      if (!token) {
        useRouter().push("/");
        console.error("未登录，请先登录");
        return;
      }

      try {
        this.isChatting = true;
        console.log("开始上传数据...");

        let response;
        if (this.selectedImage) {
          // 处理 图片 或 图片+文本
          response = await axios.post(apiUrl, formData, {
            headers: { Authorization: `Bearer ${token}`, "Content-Type": "multipart/form-data" },
          });
        } else {
          // 处理仅文本（GET 请求）
          const queryParams = new URLSearchParams(requestData).toString();
          response = await axios.get(`${apiUrl}?${queryParams}`, {
            headers: { Authorization: `Bearer ${token}` },
          });
        }

        console.log("接口响应数据：", response.data);

        if (response.data.code === 200) {
          if (!this.selectedImage) {
            // 仅文本返回
            this.messages.push({
              sender: "system",
              text: response.data.data.response || "已收到您的消息",
              side: "left",
            });
          } else {
            // 处理图片分析的返回
            const { prediction, confidence,  result_search, desc_search, weight,  } = response.data.data;
            const { url, user_id, chat_id: newChatId } = response.data; // 从 response.data 获取新的 chat_id
            console.log("response.data.data:", response.data.data);
            console.log("新的 chat_id:", newChatId);
            this.messages.push({
              sender: "system",
              text: `预测分类结果: ${prediction}，置信度: ${confidence}%`,
              side: "left",
            });

            // 添加加载状态消息
            const loadingMessage = {
              sender: "system",
              side: "left",
              loading: true,
              thoughts: "",
              response: "",
              showResponse: false,
            };
            this.messages.push(loadingMessage);

// 触发流式 SSE 请求
const requestData = {
  conversation: textOnly || "皮肤瘙痒",
  prediction,
  confidence,
  result_search,
  desc_search,
  weight,
  url,
  chat_id: newChatId, // 使用新的 chat_id
  user_id: user_id,
  model: "DeepSeek-R1",
};
            console.log("发送到第三个接口的请求参数：", requestData);
            const jsonString = JSON.stringify(requestData);
            const base64Data = btoa(unescape(encodeURIComponent(jsonString)));
            const eventSource = new EventSource(
              `${API_BASE}/predict/text?data=${encodeURIComponent(base64Data)}&model=DeepSeek-R1`
            );

            eventSource.addEventListener("message", (event) => {
              window.globalState.showResponse = true;
              updateDOM();
              const lastLoadingMessage = this.messages.find((msg) => msg.loading);
              if (lastLoadingMessage) {
                lastLoadingMessage.response += event.data;
              }
            });

            eventSource.addEventListener("thoughts", (event) => {
              const lastLoadingMessage = this.messages.find((msg) => msg.loading);
              if (lastLoadingMessage) {
                lastLoadingMessage.thoughts += event.data;
                if (event.data.includes("end")) {
                  lastLoadingMessage.loading = false;
                  lastLoadingMessage.showResponse = true;
                }
              }
            });

            eventSource.addEventListener("error", function (event) {
              eventSource.close();
            });
          }
        } else if (response.data.code === 201) {
          await ElMessageBox.alert(
            `亲，由于图片的预测置信度较低，这边给您几个小建议哦。\n\n` +
              `1 重新拍摄患处的照片，确保清晰可见，以便更准确地分析情况。\n` +
              `2 最好去线下找专业医生看看，他们经验丰富，能给出更靠谱的诊断。\n` +
              `3 您可以补充一些症状的详细描述，这样我们可以更好地帮助您分析哦！`,
            "提示",
            {
              confirmButtonText: "确定",
              type: "warning",
            }
          );
        } else {
          console.error("上传失败:", response.data);
          alert("上传失败，请稍后再试");
        }
      } catch (error) {
        console.error("聊天失败:", error);
        alert("聊天失败，请稍后再试");
      } finally {
        this.isChatting = false;
        editableContent.innerHTML = "";
        this.userMessage = "";
        this.selectedImage = null;
        console.log("聊天处理完成");
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
          reader.onloadend = () => {
            const imageUrl = reader.result;
            const img = document.createElement('img');
            img.src = imageUrl;
            img.style.maxWidth = '30%';
            img.style.height = 'auto';
            img.style.borderRadius = '10px';
            img.style.margin = '10px 0';
            img.style.maxHeight = '400px';

            const editableContent = document.querySelector('[contenteditable="true"]');
            editableContent.appendChild(img);
          };
          console.log(file);
          reader.readAsDataURL(file);  // 将图像转换为 base64 编码，并插入
          break;
        }
      }
    },
  },
  async fetchChatMessages(chatId) {
  try {
    const response = await axios.get('/users/Check_image_ask_messages', {
      params: { chat_id: chatId, page: 1, size: 10 },
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
    });
    if (response.data.code === 200) {
      this.chatMessages = response.data.data; // 更新 chatMessages 数据
      console.log('成功获取 chatMessages:', this.chatMessages);
    } else {
      console.error('获取聊天记录失败:', response.data.message);
    }
  } catch (error) {
    console.error('获取聊天记录失败:', error);
  }
}
};
</script>

<template>
  <header1 style="position: fixed;">
    <nav class="n">
      <!-- <RouterLink to="/Picture">
        <span class="icon-tu">
          <ion-icon name="arrow-back-outline"></ion-icon>
        </span>
      </RouterLink> -->
    </nav>


  </header1>

  <main>
<div class="body">
    <div class="container1">
 <MyComponent @chat-created="handleChatCreated" @chat-selected="fetchChatMessages" />

<!-- 聊天部分 -->
<div id="qwen-chat" class="chat-container">
  <div class="header">
        <h2>AI诊疗顾问</h2>
        <RouterLink to="/Summarize">
          <el-avatar size="60" :src="avatarUrl" class="avatar">{{ avatarUrl }}</el-avatar>
        </RouterLink>
      </div>
  <div class="box5">
    <div class="chat-messages">
<!-- 显示聊天消息 -->
<div v-for="(message, index) in messages" :key="index" :class="['chat-message', message.side]">
  <div v-if="message.image">
    <img :src="message.image" alt="Image preview" class="chat-image" />
  </div>

  <div v-if="message.loading" class="loading-indicator">
  <div class="title-content thoughts">
    <h2>模型推理过程</h2>
    <div v-html="parseMarkdown(message.thoughts)" style="white-space: pre-wrap;"></div>
  </div>
  
  <div class="title-content response" style="display: none;">
  <h2>诊疗结果</h2>
  <div v-html="parseMarkdown(message.response)" style="white-space: pre-wrap;"></div>
  </div>
  
</div>
 
  <div v-if="message.text" class="markdown-content" v-html="parseMarkdown(message.text)">
    
  </div>
<!-- 当前聊天框内容展示 -->
<div v-if="chatMessages.length > 0" class="chat-messages-container">
  <div class="chat-messages">
    <h3>聊天内容</h3>11
    <div v-for="(msg, idx) in chatMessages" :key="idx" class="message-item">
      <p><strong>问题：</strong>{{ msg.ask || '无' }}</p>
      <p><strong>回答：</strong>{{ msg.message || '无' }}</p>
    </div>
  </div>
</div>
</div>


    </div>
  </div>
  <div v-if="selectedImage" class="image-preview" >
      <p class="image-filename">{{ selectedImage.name }}</p>
    </div>
  <div class="anniu">
    <form @submit="handleImageAndChat" class="chat-form">
      <div 
  contenteditable="true" 
  @input="updateUserMessage"
  placeholder="请输入您的问题" 
  class="textarea-wen" 
  @paste="handlePaste"
  ref="editableContent"
  v-html="userMessage"  
  @keydown.enter="handleImageAndChat"
>

 </div>

 <button type="submit" :disabled="isChatting" class="btn-inside" >
  <span class="icon-tu">
    <ion-icon :name="isChatting ? 'reload-circle' : 'arrow-redo'"></ion-icon> 
  </span>
  <span v-if="isChatting">{{ '发送中...' }}</span>
</button>

<!-- 语言 -->
      <div class="voice-btn" @click="startVoiceSearch">
        <span class="icon-tu">
   <ion-icon name="mic-outline"></ion-icon>
  </span>
      </div>


      
<!-- 选择文件（压缩包上传） -->
<div class="picture" @click="triggerFileUpload">
  <span class="icon-tu">
    <ion-icon name="folder-outline"></ion-icon>
  </span>
</div>
<input type="file" ref="fileInput" style="display: none" @change="handleFileUpload">



<!-- 上传图片按钮 -->
<button class="image-btn" @click="triggerImageUpload">
    <span class="icon-tu">
      <ion-icon name="image-outline"></ion-icon>
    </span>
  </button>

  <!-- 隐藏的文件输入框 -->
  <input type="file" ref="fileInput" style="display: none;" @change="onFileChange" accept="image/* ">
<div v-if="selectedImage" class="image-preview" >
    </div>
    </form>
  </div>
</div>
</div>
</div>
  </main>
</template>

<style scoped>
.chat-messages-container {
  /* 确保有足够空间显示内容 */
  margin-top: 20px;
  padding: 10px;
  background-color: #f9f9f9;
  border-radius: 10px;
  box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);
}

.message-item {
  margin-bottom: 15px;
  padding: 10px;
  background-color: #ffffff;
  border-radius: 5px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}
.body{
  /* background: url('../assets/home.jpg')no-repeat; */
  background-color: #12405d;
  min-height: 100vh;
  background-size: cover;
  background-position: center;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.loading-indicator h2 {
  font-size: 1.2em;
}

.loading-indicator div {
  background-color: #f9f9f9;
  padding: 10px;
  border-radius: 5px;
}

.loading-indicator {
  font-size: 14px;
  color: #888;
  display: flex;
  flex-direction: column; /* 设置为列布局 */
  align-items: flex-start; /* 内容左对齐 */
  gap: 10px; /* 设置元素之间的间距 */
}

.title-content {
  display: flex;
  flex-direction: column;
  align-items: flex-start; /* 确保标题和内容左对齐 */
}

.title-content h2 {
  margin: 0; /* 移除默认的上下外边距 */
  padding: 0; /* 移除默认的内边距 */
}

.title-content div {
  background-color: #f9f9f9;
  padding: 10px;
  border-radius: 5px;
}

/* 添加新的样式以突出显示回答部分 */
.title-content.response div {
  background-color: #f9f9f9; /* 浅绿色背景 */
  color: #333; /* 深色文字 */
  font-weight: bold; /* 加粗字体 */
}

bottom{
  position: fixed;
  bottom:0;
  width: 100%;
  padding: 20px 100px;
background: linear-gradient(90deg, #30b8d0f4 40% ,#55ceb2e4);
  display: flex;
  justify-content: space-between;
  align-items: center;
  z-index: 99;
  max-height: 4vw;
}

.picture {
color: rgb(13, 12, 12);
border: none;
border-radius: 5px;
padding: 10px;
font-size: 1.2em;  /* 提高按钮图标的大小 */
cursor: pointer;
margin-bottom: 10px;
position: absolute;
top: 60%;
left:30px;
}



.avatar {
  border-radius: 50%; /* 头像为圆形 */
  z-index:1003;
  background-image: url('https://13538813411wdw.oss-cn-chengdu.aliyuncs.com/33/avatar/bei1.png');
  background-size: cover;
  background-repeat: no-repeat;
  background-position: center;
  width: 2.5vw;
  height: 4.5vh;
}
.n .icon-tu{
 position: fixed;
   outline: none;
   cursor: pointer;
   font-size: 3em;
   color: #fff;
   left:350px;
   align-items: center;
   max-height: 4em;
 top: 22px;
}

header1{
  width: 100%;
  padding: 20px 100px;
 background:transparent;
  display: flex;
  justify-content: space-between;
  align-items: center;
  /* max-height: 10vh;
  height: 8vh; */
}

/* 添加Markdown内容样式 */
.markdown-content {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
  line-height: 1.6;
}

.markdown-content h2, .markdown-content h3 {
  color: #2c3e50;
  border-bottom: 1px solid #eee;
  padding-bottom: 0.3em;
}


.markdown-content ul {
  padding-left: 20px;
}


.markdown-content li {
  margin: 8px 0;
}

.markdown-content strong {
  color: #34495e;
}


.markdown-content em {
  color: #7f8c8d;
  font-style: italic;
}


.markdown-content p {
  margin: 10px 0;
}

/* 箭头符号样式 */
.markdown-content .arrow {
  font-family: "Segoe UI Symbol";
  margin: 0 5px;
}

/* 加载 */
.loading-indicator {
  font-size: 14px;
  color: #888;
  display: flex;
  align-items: center;

}


body{
min-height: 100vh;
background: url('../assets/home.jpg')no-repeat;
background-color: #7bc1cc78;
background-size: cover;
background-position: center;
min-height: 100vh;
}


/* 图片预览容器 */
.image-preview-container {
  display: flex; /* 使用 Flexbox 布局 */
  flex-direction: column; /* 垂直排列图片和文件名 */
  align-items: flex-start; /* 左对齐 */
  gap: 10px; /* 图片与文件名之间的间距 */
  width: auto; /* 限制容器宽度 */
  margin-top: 10px;
  position:absolute; /* 确保子元素相对定位 */
}


/* 图片预览样式 */
.preview-img {
  max-width: 40%; /* 保证图片宽度适配容器 */
  height: auto; /* 保持图片比例 */
  border-radius: 10px; /* 圆角效果 */
  display: block;
  max-height: 300px; /* 限制最大高度 */
}

/* 文件名样式 */
.image-filename {
  font-size: 1em; /* 文件名字体大小 */
  width:auto;
  height: auto;
  color: #333; /* 文件名颜色 */
  word-break: break-all; /*  长文件名换行 */
  white-space: normal; /* 文件名多行显示 */
  background-color: rgba(255, 255, 255, 0.); /* 文件名背景透明度 */
  padding: 2px; /* 增加内边距 */
  border-radius: 5px; /* 圆角背景 */
  z-index: 2; /* 确保文件名在图片上方 */
  text-align: left; /* 左对齐文本 */
}

.chat-form {
  display: flex;
  align-items: center;  /* 垂直居中对齐 */
  gap: 10px; /* 输入框与按钮之间的间距 */
  width:100%;
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
  width: 100%; /* 可以调整图片的宽度 */
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
  background-color: #f5f2f2;
  margin-top: 5px;
  word-wrap: break-word; /* 允许换行 */
}

.textarea-wen img {
  max-width: 40%;  /* 保证图片宽度不会超过输入框宽度 */
  height: auto;
  max-Height:20vh;
  border-radius: 10px;
  margin: 10px 0;  /* 上下间距 */
  display: block;  /* 让图片成为块级元素，避免与文本混排 */
}




.chat-container {
  width:75vw;
  /* max-width: 55vw; */
  height:auto;
  margin-left: 100px;
  border-radius: 10px;
  padding: 20px;
  margin-left: 350px;
  background-image:url('../assets/home.jpg') ;
}



.container1 {
  display: flex;
  justify-content: center;
  align-items:center;
  gap: 20px;
  max-width: 100%;
  padding-top: 1vh;

  
}



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
  min-height: 100vh;
  background: url('../assets/登.png')no-repeat;
  background-size: cover;
  background-position: center;
} 


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
  height: 100%;

}



.chat-message {
  max-width: 70%;
  padding: 10px;
  border-radius: 15px;
  margin: 5px 0;
  font-size: 1.2em;
  word-wrap: break-word;
}



.chat-message.right {
  align-self: flex-end;
  background-color: #f1f1f1;  /* 用户的消息背景色 */
  color: #333;
}



.chat-message.left {
  align-self: flex-start;
  background-color: #f1f1f1;  /* 系统或机器人的消息背景色 */
  color: #333;
}



.box5{
  padding: 10px;
  width:100%;
  height: 75vh;
  overflow: auto;
  background-color: rgba(255, 255, 255, 0.4);
  border-radius: 10px;
}

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
  font-size: 1.2em;
  width:auto;
}


.btn-inside:disabled {
  background-color: #aaa;
  cursor: not-allowed;
}



/* 图片上传按钮样式 */
.image-btn {
  position: absolute;
  top: 82%; /* 垂直居中 */
  left:67px; /* 靠右 */
  transform: translateY(-50%);
  background-color: transparent;
  color: #171515;
  border: none;
  border-radius: 5px;
  padding: 5px;
  cursor: pointer;
  font-size: 1.3em;
  width:auto;
}



.demo-type {
  display: flex;
  justify-content: space-between; /* 让两个部分（账号和头像）分开 */
  width: 100%; /* 填满整个父容器 */


}



.demo-type > div {
  flex: 1;
  text-align: center;

}

.demo-type > div:not(:last-child) {
  border-right: 1px solid var(--el-border-color);

}


</style>