<template>
  <div class="sidebar-container">
    <!-- 新建对话：独立主按钮，避免和列表操作混在一起 -->
    <button type="button" class="create-chat-button" @click="createChat()">
      <ion-icon name="add-outline"></ion-icon>
      <span>新建对话</span>
    </button>

    <!-- 按对话标题搜索历史记录 -->
    <div class="history-search">
      <ion-icon name="search-outline" class="history-search-icon"></ion-icon>
      <input
        v-model="searchKeyword"
        type="search"
        class="history-search-input"
        placeholder="搜索历史对话"
        aria-label="搜索历史对话"
      />
      <button
        v-if="searchKeyword"
        type="button"
        class="history-search-clear"
        title="清空搜索"
        aria-label="清空搜索"
        @click="searchKeyword = ''"
      >
        <ion-icon name="close-circle-outline"></ion-icon>
      </button>
    </div>

    <div class="sidebar-title">
      <span>历史对话</span>
      <span class="history-count">{{ messages.length }}</span>
    </div>

    <div class="sidebar-content">
      <!-- 历史记录列表 -->
      <div class="history-list">
        <div
          v-for="message in filteredMessages"
          :key="message.chat_id"
          class="history-item"
          :class="{ 'is-active': activeChatId === message.chat_id }"
          @click="selectHistoryItem(message.chat_id, message.name)"
        >
          <template v-if="editingChatId === message.chat_id">
            <el-input
              v-model="editedChatName"
              @click.stop
              @blur="saveEditedChatName(message.chat_id)"
              @keyup.enter.stop.prevent="saveEditedChatName(message.chat_id)"
              @keyup.esc.stop="cancelEditingChatName"
              ref="editInput"
              autofocus
              size="small"
              maxlength="30"
              class="edit-input custom-background"
            />
          </template>
          <template v-else>
            <!-- 标题超长时省略显示，鼠标悬浮通过 title 展示完整名称 -->
            <span class="history-title" :title="message.name || '未命名对话'">
              {{ message.name || '未命名对话' }}
            </span>
            <span class="history-actions" @click.stop>
              <button
                type="button"
                class="history-action-btn"
                title="重命名"
                aria-label="重命名对话"
                @click.stop="startEditingChatName(message.chat_id)"
              >
                <ion-icon name="create-outline"></ion-icon>
              </button>
              <button
                type="button"
                class="history-action-btn history-action-btn--danger"
                title="删除对话"
                aria-label="删除对话"
                @click.stop="deleteChat(message.chat_id)"
              >
                <ion-icon name="trash-outline"></ion-icon>
              </button>
            </span>
          </template>
        </div>

        <p v-if="!messages.length" class="history-empty">暂无历史对话</p>
        <p v-else-if="!filteredMessages.length" class="history-empty">没有匹配的对话</p>
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
      page: 1, // 当前页码
      pageSize: 10, // 每页大小
      total: 0, // 总条数
      activeChatId: null, // 当前选中的历史对话
      editingChatId: null, // 当前正在编辑的聊天ID
      editedChatName: "", // 编辑中的聊天名称
      editingOriginalName: "", // 编辑前的名称，用于判断是否真的改过
      searchKeyword: "", // 历史对话搜索关键词
    };
  },
  computed: {
    filteredMessages() {
      const keyword = this.searchKeyword.trim().toLowerCase();
      if (!keyword) return this.messages;
      return this.messages.filter(item =>
        (item.name || '未命名对话').toLowerCase().includes(keyword)
      );
    },
  },
  created() {
    this.fetchHistory();
    // 进入页面时只准备一个可用 chat_id，不把它塞进历史列表；
    // 用户主动点“新建对话”时才在列表顶部创建可见的新条目。
    this.createChat(false);
  },
  methods: {
    // 切换选中的历史记录项，同时把标题传给右侧聊天头部
    selectHistoryItem(chatId, name) {
      if (this.editingChatId === chatId) return; // 编辑标题时不要重新加载对话
      this.activeChatId = chatId;
      this.$emit('chat-selected', chatId, name); // 右侧聊天区会拉取该对话内容
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
        // 后端把总数放在 pagination.total 里
        this.total = response.data.pagination ? response.data.pagination.total : 0;
      }
    } catch (error) {
      console.error('获取历史记录失败:', error);
    }
  },


 // 创建聊天框；showInList=false 时只生成 chat_id，不插入历史列表
 async createChat(showInList = true) {
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
          if (showInList) {
            // 用户主动新建时，立即在侧栏出现“新对话”，并设为当前选中项
            this.messages.unshift({
              chat_id: newChatId,
              type: 'assistant',
              name: '新对话',
              user_id: localStorage.getItem('user_id')
            });
            this.activeChatId = newChatId;
            this.editingChatId = null;
            this.searchKeyword = '';
          }
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
            if (this.activeChatId === chatId) {
              this.activeChatId = null;
              this.$emit('chat-deleted', chatId);
              // 当前对话被删掉后补一个可用 chat_id，避免下一条消息没有归属
              this.createChat(false);
            }
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

    // 开始编辑聊天记录名称
    startEditingChatName(chatId) {
      const target = this.messages.find(msg => msg.chat_id === chatId);
      if (!target) return;
      this.editingChatId = chatId;
      this.editedChatName = target.name || '';
      this.editingOriginalName = target.name || '';
      this.$nextTick(() => {
        // v-for 中的 ref 是数组，这里取当前渲染出来的输入框
        const input = Array.isArray(this.$refs.editInput)
          ? this.$refs.editInput[0]
          : this.$refs.editInput;
        input?.focus?.();
      });
    },

    // 取消编辑，恢复原标题
    cancelEditingChatName() {
      this.editingChatId = null;
      this.editedChatName = '';
      this.editingOriginalName = '';
    },

    // 保存编辑后的聊天记录名称
    async saveEditedChatName(chatId) {
      if (this.editingChatId !== chatId) return; // 防止 Enter 与 blur 重复提交

      const newName = this.editedChatName.trim();
      if (!newName || newName === this.editingOriginalName) {
        this.cancelEditingChatName();
        return;
      }

      try {
        const response = await axios.post(
          '/users/Update_chat',
          { id: chatId, name: newName },
          { headers: { Authorization: `Bearer ${localStorage.getItem('token')}` } }
        );
        if (response.data.code === 200) {
          ElMessage.success('对话名称已更新');
          // 更新本地数据
          this.messages = this.messages.map(msg =>
            msg.chat_id === chatId ? { ...msg, name: newName } : msg
          );
          // 当前选中的对话改名后，右侧头部标题同步更新
          if (String(this.activeChatId) === String(chatId)) {
            this.$emit('chat-title-updated', { chatId, name: newName });
          }
        } else {
          ElMessage.error('修改失败: ' + response.data.message);
        }
      } catch (error) {
        console.error('修改聊天记录名称失败:', error);
        ElMessage.error('修改失败');
      } finally {
        this.cancelEditingChatName();
      }
    }
  }
};
</script>

<style scoped>
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

<style scoped>
/* =========================================================
   问诊历史侧栏：从“深蓝大块 + 半透明项”改为白色医疗卡片
   ========================================================= */
.sidebar-container {
  position: sticky !important;
  top: 0 !important;
  left: auto !important;
  right: auto !important;
  flex: 0 0 300px;
  width: 300px !important;
  height: 100% !important; /* 跟随 .chat-layout，与右侧聊天卡片同高 */
  min-height: 0;
  padding: 16px !important;
  border: 1px solid var(--ai-line);
  border-radius: 24px !important;
  background: #fff !important;
  box-shadow: var(--ai-shadow-sm);
  z-index: 60 !important;
  overflow: hidden;
}

.sidebar-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin: 14px 2px 8px !important;
  padding: 0 2px 8px;
  border-bottom: 1px solid var(--ai-line);
  color: var(--ai-muted) !important;
  font-size: 12px !important;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.history-count {
  min-width: 20px;
  height: 20px;
  display: inline-grid;
  place-items: center;
  padding: 0 6px;
  border-radius: 99px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 11px;
  font-weight: 700;
}

.create-chat-button {
  position: static !important;
  width: 100%;
  height: 40px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  margin-top: 10px;
  padding: 0 12px !important;
  border: 1px solid var(--ai-primary) !important;
  border-radius: 11px;
  background: var(--ai-primary) !important;
  color: #fff !important;
  font-size: 13px !important;
  font-weight: 650;
  cursor: pointer;
  transition: background 0.18s ease, box-shadow 0.18s ease;
  box-shadow: 0 8px 18px rgba(15, 111, 122, 0.18);
}

.create-chat-button:hover {
  background: var(--ai-primary-deep) !important;
  box-shadow: 0 10px 22px rgba(15, 111, 122, 0.24);
}

.create-chat-button ion-icon {
  font-size: 17px;
}

/* 历史对话搜索框 */
.history-search {
  position: relative;
  display: flex;
  align-items: center;
  gap: 7px;
  height: 36px;
  margin-top: 9px;
  padding: 0 9px;
  border: 1px solid var(--ai-line);
  border-radius: 10px;
  background: #f7fbfa;
  transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

.history-search:focus-within {
  border-color: rgba(23, 184, 166, 0.55);
  background: #fff;
  box-shadow: 0 0 0 3px rgba(23, 184, 166, 0.1);
}

.history-search-icon {
  flex: 0 0 auto;
  color: var(--ai-muted);
  font-size: 15px;
}

.history-search-input {
  flex: 1;
  min-width: 0;
  height: 100%;
  padding: 0;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--ai-ink);
  font-size: 12px;
}

.history-search-input::placeholder {
  color: #9bb0b8;
}

.history-search-input::-webkit-search-cancel-button {
  display: none;
}

.history-search-clear {
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--ai-muted);
  font-size: 14px;
  cursor: pointer;
}

.history-search-clear:hover {
  color: var(--ai-primary);
}

.sidebar-content {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.history-list {
  max-height: none !important;
  display: flex;
  flex-direction: column;
  gap: 7px;
  padding-right: 2px;
}

.history-item {
  min-height: 48px;
  padding: 8px 8px 8px 12px !important;
  margin: 0 !important;
  display: flex;
  align-items: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: 13px !important;
  background: #f7fbfa !important;
  box-shadow: none !important;
  transition: border-color 0.2s ease, background 0.2s ease, transform 0.2s ease;
}

.history-item:hover {
  background: #fff !important;
  border-color: var(--ai-line-strong);
  transform: translateX(2px);
}

.history-item.is-active {
  background: linear-gradient(135deg, #e9f9f5 0%, #dff4ef 100%) !important;
  border-color: rgba(15, 111, 122, 0.42) !important;
  box-shadow: inset 3px 0 0 var(--ai-primary), 0 6px 16px rgba(15, 111, 122, 0.1) !important;
  transform: translateX(2px);
}

.history-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--ai-ink);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-item.is-active .history-title {
  color: var(--ai-primary-deep);
  font-weight: 700;
}

.history-item.is-active .history-action-btn {
  color: var(--ai-primary-deep);
}

.history-actions {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  opacity: 0.55;
  transition: opacity 0.18s ease;
}

.history-item:hover .history-actions,
.history-item:focus-within .history-actions,
.history-item.is-active .history-actions {
  opacity: 1;
}

.history-action-btn {
  width: 26px;
  height: 26px;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--ai-muted);
  font-size: 15px;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.history-action-btn:hover {
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
}

.history-action-btn--danger:hover {
  background: #fdecec;
  color: #c82333;
}

.history-empty {
  margin: 18px 4px;
  color: var(--ai-muted);
  font-size: 12px;
  text-align: center;
}

.history-header {
  color: var(--ai-muted) !important;
  font-size: 11px;
  font-weight: 500 !important;
}

.history-content {
  padding-top: 3px !important;
  color: var(--ai-ink) !important;
  font-size: 13px !important;
  font-weight: 600;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dropdown-icon {
  right: 9px !important;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 7px;
  color: var(--ai-muted) !important;
  font-size: 15px !important;
}

.dropdown-icon:hover {
  background: var(--ai-primary-soft);
  color: var(--ai-primary) !important;
}

.edit-input {
  width: 100% !important;
  height: 32px !important;
}

.custom-background {
  background: #fff !important;
}

.chat-messages-container {
  flex: 0 0 auto;
  max-height: 34%;
  overflow-y: auto;
  padding: 12px 0 0 !important;
  border-top: 1px solid var(--ai-line);
}

.chat-messages {
  width: 100% !important;
  max-width: none !important;
  margin: 0 !important;
  padding: 0 !important;
  background: transparent !important;
  box-shadow: none !important;
}

.chat-messages h3 {
  margin: 0 0 8px;
  color: var(--ai-ink);
  font-size: 12px;
  font-weight: 700;
}

.message-item {
  margin-bottom: 8px !important;
  padding: 9px 10px !important;
  border: 1px solid var(--ai-line);
  border-radius: 10px !important;
  background: #f7fbfa !important;
  box-shadow: none !important;
  color: var(--ai-ink-soft);
  font-size: 11px;
  line-height: 1.6;
}

.message-item p {
  margin: 2px 0;
}

@media (max-width: 1100px) {
  .sidebar-container {
    flex-basis: 250px;
    width: 250px !important;
  }
}

@media (max-width: 820px) {
  .sidebar-container {
    display: none !important;
  }
}
</style>