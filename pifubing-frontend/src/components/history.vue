<template>
  <div class="sidebar-container">
    <RouterLink to="/Picture" class="back-button">
      <span class="icon-tu">
        <ion-icon name="arrow-back-outline"></ion-icon>返回上一页
      </span>
    </RouterLink>
    <div class="sidebar-title">聊天历史
      <!-- 创建聊天框按钮 -->
      <button @click="createChat" class="create-chat-button">
        <ion-icon name="create-outline"></ion-icon>
      </button>
    </div>

    <div class="sidebar-content">
      <!-- 历史记录列表 -->
      <div class="history-list">
        <div
          v-for="(message, index) in messages"
          :key="index"
          class="history-item"
          @click="selectHistoryItem(message.chat_id)"
        >
          <div class="history-header">
            <div class="history-sender">{{ message.type === 'user' ? '你' : '' }}</div>
            <div class="history-preview">{{ }}</div>
          </div>
          <div class="history-content">
            <template v-if="editingChatId === message.chat_id">
              <el-input
                v-model="editedChatName"
                @blur="saveEditedChatName(message.chat_id)"
                @keyup.enter="saveEditedChatName(message.chat_id)"
                ref="editInput"
                autofocus
                size="small"
                class="edit-input custom-background" 
              />
            </template>
            <template v-else>
              {{ message.name }}
            </template>
          </div>
          <!-- 下拉菜单 -->
          <el-dropdown trigger="click" @command="handleCommand">
            <span class="dropdown-icon">
              <ion-icon name="ellipsis-vertical-outline"></ion-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item :command="{ action: 'delete', chatId: message.chat_id }">
                  <span class="icon"><ion-icon name="trash-outline"></ion-icon></span><p style="color:#c82333;"> 删除历史</p>
                </el-dropdown-item>
                <el-dropdown-item :command="{ action: 'edit', chatId: message.chat_id }">
                  <span class="icons"><ion-icon name="brush-outline"></ion-icon></span>重命名
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </div>
    </div>

    <!-- 当前聊天框内容展示 -->
    <div v-if="chatMessages.length > 0" class="chat-messages-container">
      <div class="chat-messages">
        <h3>聊天内容</h3>
        <div v-for="(msg, idx) in chatMessages" :key="idx" class="message-item">
          <p><strong>问题：</strong>{{ msg.ask }}</p>
          <p><strong>回答：</strong>{{ msg.message }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import axios from 'axios';
// 假设在登录成功后存储 token
import { ElMessageBox, ElMessage } from 'element-plus';
export default {
  data() {
    return {
      messages: [], // 聊天历史记录
      chatMessages: [], // 当前聊天框的内容
      page: 1, // 当前页码
      pageSize: 10, // 每页大小
      total: 0, // 总条数
      editingChatId: null, // 当前正在编辑的聊天ID
      editedChatName: "", // 编辑中的聊天名称
    };
  },
  created() {
    this.fetchHistory();
    this.createChat();
  },
  methods: {
    // 切换选中的历史记录项
    selectHistoryItem(chatId) {
    console.log('触发 chat-selected 事件，chatId:', chatId); // 调试信息
    this.$emit('chat-selected', chatId); // 触发 chat-selected 事件
    this.fetchChatMessages(chatId); // 获取当前聊天框的内容
  },

 // 获取聊天历史记录
 async fetchHistory() {
    try {
      const token = localStorage.getItem('token');
      if (!token) return;
      const response = await axios.get('/users/Check_chat', {
        headers: { Authorization: `Bearer ${token}` },
        params: { page: this.page, size: this.pageSize }
      });
      if (response.data.code === 200) {
        this.messages = response.data.data.map(item => ({
          chat_id: item.id,
          type: 'assistant',
          name: item.name,
          user_id: item.user_id,
        }));
        console.log('获取历史记录成功:', response.data);
        this.total = response.data.total;
      }
    } catch (error) {
      console.error('获取历史记录失败:', error);
    }
  },


    // 获取单个聊天框的所有内容
    async fetchChatMessages(chatId) {
      try {
        const response = await axios.get('/users/Check_image_ask_messages', {
          params: { chat_id: chatId, page: 1, size: 10 },
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
        });
        console.log('获取聊天框内容请求成功:', response.data);
        if (response.data.code === 200) {
          this.chatMessages = response.data.data; // 存储聊天框内容
          console.log('获取聊天框内容成功:', this.chatMessages);
        } else {
          console.error('获取聊天框内容失败:', response.data.message);
        }
      } catch (error) {
        console.error('获取聊天框内容失败:', error);
      }
    },


 // 创建聊天框
 async createChat() {
    try {
      const url = '/users/Create_chat';
      const method = 'POST';
      const headers = { Authorization: `Bearer ${localStorage.getItem('token')}` };
      const data = { name: '新对话' };

      console.log('请求方法:', method);

      const response = await axios.post(url, data, { headers });

      console.log('创建个人聊天框请求成功:', response.data);
      if (response.data.code === 200) {
        console.log('创建个人聊天框成功:', response.data.message);
        const newChatId = response.data.chat_id; // 获取新创建聊天框的ID
        console.log('新创建聊天框的ID:', newChatId);
        localStorage.setItem('newChatId', newChatId);
        if (newChatId) {
          this.$emit('chat-created', newChatId); // 触发自定义事件并传递 chat_id
          return newChatId; // 返回新创建聊天框的ID
        } else {
          console.error('新创建聊天框的ID未找到:', response.data);
        }
      } else {
        console.error('创建个人聊天框失败:', response.data.message);
      }
    } catch (error) {
      console.error('创建个人聊天框失败:', error);
    }
  },
    // 删除聊天框
    async deleteChat(chatId) {
      try {
        const confirm = await ElMessageBox.confirm(
          '确定要删除这个聊天记录吗？此操作无法撤销。',
          '删除确认',
          {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning',
          }
        );

        if (confirm === 'confirm') {
          const response = await axios.delete('/users/Delete_chat', {
            params: { chat_id: chatId },
            headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
          });

          if (response.data.code === 200) {
            ElMessage.success('聊天记录删除成功');
            this.messages = this.messages.filter(msg => msg.chat_id !== chatId);
            this.chatMessages = [];
          } else {
            ElMessage.error('删除失败: ' + response.data.message);
          }
        }
      } catch (error) {
        if (error !== 'cancel') {
          console.error('删除聊天框失败:', error);
          ElMessage.error('删除失败');
        }
      }
    },

    // 处理下拉菜单命令
    handleCommand(command) {
      if (command.action === 'delete') {
        this.deleteChat(command.chatId);
      } else if (command.action === 'edit') {
        this.startEditingChatName(command.chatId);
      }
    },

    // 开始编辑聊天记录名称
    startEditingChatName(chatId) {
      this.editingChatId = chatId;
      this.editedChatName = this.messages.find(msg => msg.chat_id === chatId).name;
      this.$nextTick(() => {
        this.$refs.editInput.focus();
      });
    },

    // 保存编辑后的聊天记录名称
    async saveEditedChatName(chatId) {
      if (this.editedChatName) {
        try {
          const response = await axios.post(
            '/users/Update_chat',
            { id: chatId, name: this.editedChatName },
            { headers: { Authorization: `Bearer ${localStorage.getItem('token')}` } }
          );
          if (response.data.code === 200) {
            console.log('修改聊天记录名称成功:', response.data.message);
            // 更新本地数据
            this.messages = this.messages.map(msg =>
              msg.chat_id === chatId ? { ...msg, name: this.editedChatName } : msg
            );
            this.editingChatId = null;
            this.editedChatName = "";
          } else {
            console.error('修改聊天记录名称失败:', response.data.message);
          }
        } catch (error) {
          console.error('修改聊天记录名称失败:', error);
        }
      }
    }
  }
};
</script>

<style scoped>
/* 返回按钮样式 */
.back-button {
  display: flex;
  align-items: center;
  margin-bottom: 10px; /* 与标题之间的间距 */
  color: #fff; /* 按需调整颜色 */
  font-size: 1.2em;
  cursor: pointer;
  outline: none;
}

.icon-tu {
  position: relative; /* 不再使用 fixed 定位 */
  outline: none;
  cursor: pointer;
  font-size: 0.7em;
  color: #fff; /* 按需调整颜色 */
  align-items: center;

}
.icon {
  color: #c82333;
  font-size: 1.2em;
  padding: 10px;
}

.icons {
  color: black;
  font-size: 1.2em;
  padding: 10px;
}

/* 侧边栏容器样式 */
.sidebar-container {
  width: 350px;
  background-color: #2076ac9f;
  box-shadow: 2px 0 10px rgba(0, 0, 0, 0.1);
  padding: 10px;
  height: 100vh;
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 999;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
}

/* 标题样式 */
.sidebar-title {
  font-size: 1.5em;
  margin-bottom: 20px;
  color: #343a40;
  position: relative;
}

/* 侧边栏内容样式 */
.sidebar-content {
  flex: 1;
  overflow-y: auto;
}

/* 历史记录列表样式 */
.history-list {
  max-height: calc(100% - 50px);
  overflow-y: auto;
  flex: 1;
}

/* 单条历史记录的样式 */
.history-item {
  background-color: #f2f4f6c5;
  border-radius: 10px;
  margin-bottom: 5px;
  padding: 10px;
  box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: all 0.3s;
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

/* 下拉菜单图标样式 */
.dropdown-icon {
  padding: 0;
  cursor: pointer;
  transition: color 0.3s;
  font-size: 1.2em;
  color: #333;
  position: absolute;
  right: 10px;
}

.dropdown-icon:hover {
  color: #007bff;
}

/* 鼠标悬停时的样式 */
.history-item:hover {
  background-color: #2076acd4;
  border-radius: 10px;
}

/* 记录头部样式 */
.history-header {
  display: flex;
  justify-content: space-between;
  font-weight: bold;
  color: #007bff;
}

/* 记录内容样式 */
.history-content {
  padding-top: 10px;
  color: #333;
  font-size: 1em;
  line-height: 1.4;
}

/* 消息内容中的图片样式 */
.history-content img {
  max-width: 100%;
  border-radius: 5px;
  margin-top: 10px;
}

/* 创建聊天框按钮样式 */
.create-chat-button {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  transition: color 0.3s;
  font-size: 1.2em;
  color: black;
  right: 20px;
  position: absolute;
  padding: 5px;
}

/* 按钮悬停时的样式 */
.create-chat-button:hover {
  background-color: #f1f1f1ce;
  border-radius: 5px;
}

/* 删除聊天框按钮样式 */
.delete-chat-button {
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  transition: color 0.3s;
  font-size: 1.2em;
  color: #dc3545;
  position: absolute;
  top: 10px;
  right: 10px;
}

/* 删除按钮悬停时的样式 */
.delete-chat-button:hover {
  color: #c82333;
}

/* 新增样式：聊天内容展示容器 */
.chat-messages-container {
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 20px;
}

/* 新增样式：聊天内容展示 */
.chat-messages {
  width: 80%;
  max-width: 600px;
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

/* 三个点的图标样式 */
.more-options-button {
  padding: 0;
  cursor: pointer;
  transition: color 0.3s;
  font-size: 1.2em;
  color: #333;
  position: absolute;
  top: 10px;
  right: 10px;
}

/* 三个点的图标悬停时的样式 */
.more-options-button:hover {
  color: #007bff;
}

/* 下拉菜单样式 */
.dropdown-menu {
  position: absolute;
  top: 40px;
  right: 10px;
  background: white;
  border: 1px solid #ccc;
  border-radius: 5px;
  box-shadow: 0 2px 5px rgba(0, 0, 0, 0.1);
  z-index: 2000;
}

.dropdown-menu ul {
  list-style: none;
  padding: 0;
  margin: 0;
}

.dropdown-menu li {
  padding: 5px 10px;
  cursor: pointer;
  transition: background 0.3s;
}

.dropdown-menu li:hover {
  background: #007bff;
  color: white;
}

/* 编辑输入框样式 */
.edit-input {
  width: 17vw;
  height: 3vh;
  margin: 0 auto;
}

/* 自定义背景颜色 */
.custom-background {
  background-color: #f0f8ff; /* 设置背景颜色 */
}
</style>