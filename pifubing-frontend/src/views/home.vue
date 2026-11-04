<script>
import { ref } from 'vue';
import axios from 'axios';
import { ElMessage, ElMessageBox } from 'element-plus';
import { marked } from 'marked';
import MyComponent from '../components/history.vue';
import AppNavbar from '../components/AppNavbar.vue';
import { API_BASE } from '../apiBase';

const user_id = localStorage.getItem('user_id');

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
      isThinking: false, // 是否还在等待 AI 的第一个内容块，控制“思考中”按钮文案
      activeStreamController: null, // 文字流式回答使用的 AbortController
      userMessage: ref(''), // 用户输入的文本
      messages: ref([]), // 消息数组
      rawText: ref(''), // 存储 Markdown 格式的文本
      // 不再从 localStorage 恢复旧 chatId：那条对话可能已被删除，
      // 继续复用会把新消息写进一个不存在的对话。首次发送时按需创建。
      chatId: 0,
      userId: user_id ,// 初始化 userId
      chatTitle: '新对话', // 右侧聊天头部展示的当前对话标题
      // 历史记录请求序号：快速连续切换会话时，只采纳最后一次请求的结果
      historyRequestId: 0,
      // 发送会话序号：切换/新建/删除对话后，之前尚未返回的图片问诊响应会被丢弃
      streamSessionId: 0,
      isEditingTitle: false, // 是否正在编辑聊天头部标题
      titleDraft: '', // 标题编辑框中的临时内容
      activeEventSource: null, // 当前正在接收的 SSE 连接
      // 历史侧栏显隐：用户可折叠让聊天框铺满
      showHistory: localStorage.getItem('hideChatHistory') !== '1',
      // 空状态下的快捷提问：label 是界面上展示的短标题，text 是点击后实际发送的完整问题。
      // 这里先放内置兜底问题，进入页面后会异步替换成 AI 生成的热门问题；
      // 接口失败或返回为空时继续展示这一组，保证空状态永远有内容可点。
      quickQuestions: [
        { icon: 'flame-outline', label: '红疹瘙痒', text: '身上突然起了红疹，很痒，可能是什么原因？需要注意什么？' },
        { icon: 'water-outline', label: '面部泛红脱屑', text: '面部反复泛红脱屑，平时该怎么护理？' },
        { icon: 'bandage-outline', label: '手上长水疱', text: '手上长了小水疱，需要去医院吗？' },
      ],
      // 页面卸载后，迟到的热门问题响应不再写回组件，避免无意义的状态更新
      isUnmounted: false,
    };
  },

  components: {
    MyComponent,
    AppNavbar
  },

  mounted() {
    // 首页快捷提问优先使用 AI 生成的热门问题，失败时保留 data 里的兜底问题。
    this.loadHotQuestions();
  },

  computed: {
    formattedText() {
      return marked(this.rawText); // 解析 Markdown
    },
  },

  methods: {
    // 折叠/展开历史侧栏
    toggleHistorySidebar() {
      this.showHistory = !this.showHistory;
      localStorage.setItem('hideChatHistory', this.showHistory ? '0' : '1');
    },

    handleChatCreated(newChatId) {
      this.chatId = newChatId; // 更新 chatId
      localStorage.setItem('newChatId', newChatId); // 更新 localStorage 中的 chat_id
      // 新建对话后清空右侧聊天区，避免继续显示上一段对话
      this.stopActiveStreams();
      this.chatTitle = '新对话';
      this.cancelChatTitleEdit();
      this.messages = [];
      this.clearComposer();
    },

    // 删除当前对话后清空右侧聊天区，避免展示已删除的内容
    handleChatDeleted(chatId) {
      if (String(this.chatId) !== String(chatId)) return;
      this.stopActiveStreams();
      this.chatId = 0;
      this.chatTitle = '新对话';
      this.cancelChatTitleEdit();
      this.messages = [];
      this.clearComposer();
      localStorage.removeItem('newChatId');
    },

    // 侧栏重命名当前对话后，同步右侧头部标题
    handleChatTitleUpdated({ chatId, name }) {
      if (String(this.chatId) === String(chatId)) {
        this.chatTitle = name || '新对话';
        this.cancelChatTitleEdit();
      }
    },

    // 进入标题编辑：只有已经归属于某条对话时才允许改名
    startEditingTitle() {
      if (!this.chatId) {
        ElMessage.info('发送一条消息后即可修改对话标题');
        return;
      }
      // 默认标题“新对话”直接清空，避免用户还要手动删一遍
      this.titleDraft = this.chatTitle === '新对话' ? '' : this.chatTitle;
      this.isEditingTitle = true;
      this.$nextTick(() => {
        const input = this.$refs.titleInput;
        if (!input) return;
        input.focus();
        input.select();
      });
    },

    // 取消编辑，不改动标题
    cancelChatTitleEdit() {
      this.isEditingTitle = false;
      this.titleDraft = '';
    },

    // 保存头部标题：先关闭编辑态，再请求后端，成功后同步左侧历史列表
    async saveChatTitle() {
      // 防止 Enter 触发保存后，输入框卸载又触发一次 blur
      if (!this.isEditingTitle) return;
      this.isEditingTitle = false;

      const newName = (this.titleDraft || '').trim();
      const oldName = this.chatTitle || '新对话';
      this.titleDraft = '';

      // 空标题或没有实际修改时直接恢复原状，不发请求
      if (!newName || newName === oldName || !this.chatId) return;

      const token = localStorage.getItem('token');
      if (!token) {
        ElMessage.warning('登录状态已失效，请重新登录');
        this.$router.push('/HomeView');
        return;
      }

      try {
        const response = await axios.post(
          '/users/Update_chat',
          { id: this.chatId, name: newName },
          { headers: { Authorization: `Bearer ${token}` } }
        );
        if (response.data?.code === 200) {
          this.chatTitle = newName;
          // 子组件内部更新历史列表；如果当前对话还不在列表里，会补一条
          this.$refs.historyPanel?.updateChatTitle?.(this.chatId, newName);
          ElMessage.success('对话名称已更新');
        } else {
          ElMessage.error('修改失败: ' + (response.data?.message || '未知错误'));
        }
      } catch (error) {
        console.error('修改聊天记录名称失败:', error);
        ElMessage.error('修改失败');
      }
    },

    // 从用户第一条问题中提取对话标题：去掉客套话、问句和建议类内容，
    // 只保留“部位 + 主要症状”，再压缩成短语。标题不参与问诊内容。
    deriveChatTitle(text, hasImage) {
      const raw = String(text || '').replace(/\s+/g, ' ').trim();
      if (!raw) return hasImage ? '患处图片问诊' : '';

      let title = raw;
      const leadingPatterns = [
        /^(医生|大夫|专家)?(您好|你好)(医生|大夫|专家)?[，,。！!：:\s]*/,
        /^(请问|我想问一下|想咨询一下|麻烦问一下)[，,。！!：:\s]*/,
      ];
      // 连续去掉“医生您好，请问……”这类开头，避免标题只剩客套话。
      let changed = true;
      while (changed) {
        changed = false;
        for (const pattern of leadingPatterns) {
          const next = title.replace(pattern, '');
          if (next !== title) {
            title = next;
            changed = true;
          }
        }
      }

      // 问句、求医建议和情绪表达不进入标题，只保留症状描述。
      const questionTail = /[，,、]?(可能是什么|是什么.{0,4}病|什么病|什么原因|怎么回事|怎么办|怎么治疗|如何治疗|需要去医院|要不要去医院|需要就医|要不要就医|严重吗|要紧吗|能好吗|会好吗|用什么药|吃什么药|如何用药|需要注意什么|需要注意|怎么护理|如何护理|会不会|传染吗|挂什么科|看什么科|多久能好|正常吗|有问题吗|这是什么).*$/;
      const filler = /^(谢谢|好的|麻烦了|在线等|急|担心|害怕|焦虑|着急|烦死了)/;

      // 先按标点拆成短句，再逐句去掉问诊尾巴，剩下的才可能是“部位+症状”。
      const clauses = title
        .split(/[。！？!?；;\n]+|，|,/)
        .map((part) => part.replace(questionTail, '').replace(/[。！？!?，,；;：:、\s]+$/g, '').trim())
        .filter((part) => part && !filler.test(part));

      const bodyParts = ['头皮', '面部', '脸颊', '额头', '眼周', '嘴唇', '颈部', '脖子', '肩', '背', '胸', '腹', '腰', '手臂', '胳膊', '手背', '手心', '手指', '手', '大腿', '小腿', '膝盖', '脚踝', '脚背', '脚', '足', '私处', '外阴', '肛周', '腋下', '腹股沟', '全身', '身上', '皮肤'];
      const symptoms = ['红疹', '红斑', '红点', '红疙瘩', '丘疹', '疙瘩', '风团', '水疱', '脓疱', '脱屑', '鳞屑', '瘙痒', '痒', '痛', '疼', '红肿', '泛红', '脱皮', '干燥', '皲裂', '渗液', '糜烂', '结痂', '色素', '白斑', '黑痣', '痣', '痘', '痤疮', '掉发', '脱发', '掉头发', '头皮屑', '异味', '白带', '发黄', '边缘', '不规则', '发热', '发烧', '肿'];

      const tidy = (part) => part
        .replace(/^(我|我们|我的)/, '')
        .replace(/^(?:(?:平时|平常|一般|现在|目前|最近|这两天|这几天|今天|昨天|刚才|刚刚|该|应该))+/, '')
        .replace(/最近/g, '')
        .replace(/突然/g, '')
        .replace(/特别痒|非常痒|有点痒|有些痒|比较痒|很痒|好痒|瘙痒难忍|痒得(?:厉害|难受|睡不着)/g, '瘙痒')
        .replace(/^晚上|^夜里|^夜间/, '夜间')
        .replace(/特别多/g, '多')
        .replace(/很多|好多|许多|大量|一片|一些|几个/g, '')
        .replace(/^还|^而且|^并且/, '')
        .replace(/(起|长|出现)了/g, '$1')
        .replace(/(胳膊|手臂|腿|脚|背|手|身上)上有个?/g, (_match, part) => (part === '手' ? '手部' : part))
        .replace(/了$/, '')
        .replace(/\s+/g, '')
        .trim();

      const scored = clauses
        .map((part, index) => ({ text: tidy(part), index }))
        .filter((item) => item.text)
        .map((item) => ({
          ...item,
          score: (bodyParts.some((word) => item.text.includes(word)) ? 2 : 0)
            + (symptoms.some((word) => item.text.includes(word)) ? 1 : 0),
        }));

      // 优先保留有部位/症状的短句，最多两句，保证标题不是整段问题。
      let picked = scored.filter((item) => item.score > 0).slice(0, 2);
      if (picked.length === 0) {
        // 没有识别到部位/症状时，只接受至少两个字的短句，
        // 避免“这”“那”之类的问句残片被当成标题。
        const fallback = scored.find((item) => Array.from(item.text).length >= 2);
        if (fallback) picked = [fallback];
      }
      if (picked.length === 0) {
        return hasImage ? '患处图片问诊' : '皮肤问题咨询';
      }
      picked.sort((a, b) => a.index - b.index);

      let result = picked[0].text;
      if (picked.length === 2) {
        const second = picked[1].text;
        const separator = Array.from(second).length <= 6
          && !bodyParts.some((word) => second.includes(word)) ? '、' : '，';
        const joined = `${picked[0].text}${separator}${second}`;
        // 两句合起来太长时只保留第一句，避免又变成整段问题。
        if (Array.from(joined).length <= 16) {
          result = joined;
        }
      }

      const chars = Array.from(result);
      return chars.length > 16 ? chars.slice(0, 16).join('') + '…' : result;
    },

    // 首条消息发出后自动命名：先立即更新界面，再异步条件写入后端。
    // 只有“当前对话还没有消息、标题仍是新对话、不在编辑标题”时才生效。
    applyAutoChatTitle({ chatId, text, hasImage, isFirstMessage }) {
      if (!chatId || !isFirstMessage) return;
      if (this.isEditingTitle) return;
      if (this.chatTitle !== '新对话' && this.chatTitle !== '') return;

      const name = this.deriveChatTitle(text, hasImage);
      if (!name) return;

      // 用户可能已经切到别的对话，只同步对应会话的标题。
      if (String(this.chatId) === String(chatId)) {
        this.chatTitle = name;
      }
      this.$refs.historyPanel?.updateChatTitle?.(chatId, name);

      const token = localStorage.getItem('token');
      if (!token) return;
      // 标题是锦上添花：接口失败只记录日志，不打断发送和问诊。
      axios.post(
        '/users/Auto_title_chat',
        { id: chatId, name },
        { headers: { Authorization: `Bearer ${token}` } }
      ).then((response) => {
        if (response.data?.code !== 200) {
          console.warn('自动生成对话标题失败:', response.data);
        }
      }).catch((error) => {
        console.warn('自动生成对话标题失败:', error);
      });
    },

    // 清空输入框状态；输入框本身是不受 v-html 控制的内容，需要手动清 DOM
    clearComposer() {
      this.userMessage = '';
      this.selectedImage = null;
      if (this.$refs.editableContent) {
        this.$refs.editableContent.innerHTML = '';
      }
    },

    // 只同步输入状态，不回写 innerHTML，避免每输入一个字光标就跳到开头
    updateUserMessage(event) {
      this.userMessage = event?.target?.innerHTML || '';
    },

    // 触发文件上传选择
    triggerFileUpload() {
      this.$refs.archiveInput?.click();
    },

    // 切换历史记录的显示状态
    selectHistoryItem(index) {
      this.selectedIndex = this.selectedIndex === index ? null : index;
    },

    parseMarkdown(text) {
      // 保留标准 Markdown 标题/列表/引用，AI 返回的结构化内容才能正常展示
      return marked(text || '', {
        breaks: true, // 单个换行也保留
        gfm: true,
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
      event?.preventDefault();  // 阻止表单提交事件
      this.$refs.imageInput?.click();  // 触发文件选择框
    },

    // 普通文件入口目前没有对应后端接口，先明确提示，避免选了文件却毫无反应
    handleFileUpload(event) {
      const file = event.target.files?.[0];
      event.target.value = '';
      if (file) {
        ElMessage.info('目前仅支持上传图片，请使用右侧的图片按钮。');
      }
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
        const editableContent = this.$refs.editableContent;
        if (editableContent) {
          editableContent.textContent = speechResult;
          editableContent.focus();
        }
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
      const file = event.target.files?.[0];
      if (file) {
        this.selectedImage = file;
      }
      event.target.value = '';
    },

    // 回车发送；Shift+Enter 换行，中文输入法候选确认时不误发
    handleComposerEnter(event) {
      if (event.isComposing || event.keyCode === 229) return;
      if (event.shiftKey) return;
      event.preventDefault();
      this.handleImageAndChat(event);
    },

    scrollThreadToBottom() {
      this.$nextTick(() => {
        const box = this.$el?.querySelector('.box5');
        if (box) box.scrollTop = box.scrollHeight;
      });
    },

    // 追加一条“思考中”的 AI 消息，后续直接修改这个响应式对象来更新界面
    pushPendingMessage() {
      this.messages.push({
        sender: 'system',
        side: 'left',
        thinking: true,
        thoughts: '',
        response: '',
        suggestions: [],
        text: '',
        error: '',
        meta: '',
      });
      return this.messages[this.messages.length - 1];
    },

    // 只保留去重后的前3个非空追问，避免后端异常时渲染出无效按钮
    normalizeSuggestions(list) {
      if (!Array.isArray(list)) return [];
      const result = [];
      list.forEach((item) => {
        const text = typeof item === 'string' ? item.trim() : '';
        if (text && !result.includes(text) && result.length < 3) {
          result.push(text);
        }
      });
      return result;
    },

    failPendingMessage(aiMessage, error) {
      aiMessage.thinking = false;
      aiMessage.error = '抱歉，AI 服务暂时不可用，请稍后再试。';
      this.isThinking = false;
      this.isChatting = false;
      console.error('聊天失败:', error);
      ElMessage.error('发送失败，请稍后再试');
      this.scrollThreadToBottom();
    },

    // 图片问诊：先识别图片，再用 SSE 接收大模型的流式回答
    async sendImageQuestion({ imageFile, conversation, chatId, userId, token, aiMessage, sessionId }) {
      const formData = new FormData();
      formData.append('conversation', conversation);
      formData.append('chat_id', chatId);
      formData.append('user_id', userId);
      formData.append('model', 'DeepSeek-R1');
      formData.append('file', imageFile);

      let response;
      try {
        response = await axios.post('/users/predict/image', formData, {
          headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'multipart/form-data' },
        });
      } catch (error) {
        if (sessionId !== this.streamSessionId) return;
        this.failPendingMessage(aiMessage, error);
        return;
      }

      // 等待识别期间用户已切换/新建对话，丢弃这条过期响应：
      // 否则会把 chatId 改回旧会话，后续消息就发错地方了。
      if (sessionId !== this.streamSessionId) return;

      if (response.data?.code === 200) {
        const data = response.data.data || {};
        const newChatId = response.data.chat_id;
        if (newChatId) {
          this.chatId = newChatId;
          localStorage.setItem('newChatId', newChatId);
        }

        const {
          prediction,
          confidence,
          result_search,
          desc_search,
          weight,
          candidates,
          structure_summary,
        } = data;
        const confidenceNumber = Number(confidence);
        const lowConfidence = Number.isFinite(confidenceNumber) && confidenceNumber < 80;
        const candidateList = Array.isArray(candidates) ? candidates.filter((item) => item?.prediction) : [];
        // 展示 top-3 候选，让用户知道模型并非“只认一个病名”；
        // 置信度后端已经换算成百分比，这里只做两位小数格式化。
        const candidateText = candidateList
          .slice(0, 3)
          .map((item) => {
            const value = Number(item.confidence);
            return `${item.prediction} ${Number.isFinite(value) ? value.toFixed(2) : '-'}%`;
          })
          .join('、');
        aiMessage.meta =
          `预测结果：${prediction ?? '未知'}，置信度：${confidence ?? '-'}%` +
          (candidateText ? `；候选：${candidateText}` : '') +
          (lowConfidence ? '（置信度较低，结果仅供参考）' : '');
        // 视觉模型只描述形态，单独展示，避免和“预测结果”混在一起。
        aiMessage.structure = structure_summary || '';

        this.startImageStream(aiMessage, {
          conversation,
          prediction,
          confidence,
          candidates: candidateList,
          structure_summary: structure_summary || '',
          result_search,
          desc_search,
          weight,
          url: response.data.url,
          chat_id: newChatId || chatId,
          user_id: response.data.user_id || userId,
          model: 'DeepSeek-R1',
        });
        return;
      }

      // 低置信度或失败时撤回“思考中”气泡，只保留用户消息
      const pendingIndex = this.messages.indexOf(aiMessage);
      if (pendingIndex !== -1) this.messages.splice(pendingIndex, 1);
      this.isThinking = false;
      this.isChatting = false;

      if (response.data?.code === 201) {
        await ElMessageBox.alert(
          `亲，由于图片的预测置信度较低，这边给您几个小建议哦。\n\n` +
            `1 重新拍摄患处的照片，确保清晰可见，以便更准确地分析情况。\n` +
            `2 最好去线下找专业医生看看，他们经验丰富，能给出更靠谱的诊断。\n` +
            `3 您可以补充一些症状的详细描述，这样我们可以更好地帮助您分析哦！`,
          '提示',
          {
            confirmButtonText: '确定',
            type: 'warning',
          }
        );
      } else if (response.data?.code === 202) {
        // 图片质量门槛未通过：明确告诉用户哪里有问题，并给出可执行的拍摄建议。
        await ElMessageBox.alert(
          `${response.data?.error || '图片质量不足，请重新拍摄。'}\n\n` +
            `拍摄建议：\n` +
            `1 让患处占画面 40%~60%，不要离得太远；\n` +
            `2 在自然光下拍摄，关闭闪光灯和美颜；\n` +
            `3 保持对焦清晰，可换 2~3 个角度再拍。`,
          '提示',
          {
            confirmButtonText: '确定',
            type: 'warning',
          }
        );
      } else {
        console.error('上传失败:', response.data);
        ElMessage.error(response.data?.error || '上传失败，请稍后再试');
      }
    },

    // 打开 SSE，把推理过程和最终回答写回同一条 AI 消息
    startImageStream(aiMessage, requestData) {
      try {
        const jsonString = JSON.stringify(requestData);
        const base64Data = btoa(unescape(encodeURIComponent(jsonString)));
        const eventSource = new EventSource(
          `${API_BASE}/predict/text?data=${encodeURIComponent(base64Data)}&model=DeepSeek-R1`
        );
        this.activeEventSource = eventSource;

        eventSource.addEventListener('message', (event) => {
          if (!event.data) return;
          aiMessage.thinking = false;
          this.isThinking = false;
          aiMessage.response += event.data;
          this.scrollThreadToBottom();
        });

        eventSource.addEventListener('thoughts', (event) => {
          if (!event.data) return;
          aiMessage.thoughts += event.data;
          this.scrollThreadToBottom();
        });

        // 图片问答的追问由 Go 在回答结束后单独下发
        eventSource.addEventListener('suggestions', (event) => {
          if (!event.data) return;
          try {
            const payload = JSON.parse(event.data);
            aiMessage.suggestions = this.normalizeSuggestions(payload?.suggestions);
          } catch {
            aiMessage.suggestions = [];
          }
          this.scrollThreadToBottom();
        });

        eventSource.addEventListener('error', () => {
          // 后端流式响应正常结束时也会触发 error；主动 close 防止浏览器自动重连
          eventSource.close();
          if (this.activeEventSource === eventSource) this.activeEventSource = null;
          aiMessage.thinking = false;
          this.isThinking = false;
          if (!aiMessage.response) {
            aiMessage.error = aiMessage.thoughts
              ? '本次只返回了推理过程，请重新发送试试。'
              : 'AI 回复中断，请稍后重试。';
          }
          this.isChatting = false;
          this.scrollThreadToBottom();
        });
      } catch (error) {
        this.failPendingMessage(aiMessage, error);
      }
    },

    // 进入首页时请求 AI 生成的热门皮肤科问题，成功后替换默认快捷提问。
    // 这是锦上添花的功能：未登录、接口失败、返回为空或格式不对时都静默保留兜底问题。
    async loadHotQuestions() {
      const token = localStorage.getItem('token');
      if (!token) return;

      try {
        // 后端每小时才重新生成一次问题池，平时毫秒级返回；
        // 只有池子过期后的第一次请求要等大模型，超时留得比后端 45 秒稍长。
        // 页面上此时已经显示兜底问题，等待不会让用户面对空白区域。
        const response = await axios.get('/users/hot_questions', {
          headers: { Authorization: `Bearer ${token}` },
          timeout: 50000,
        });
        // 页面已经卸载，丢弃这次迟到的响应
        if (this.isUnmounted) return;
        if (response.data?.code !== 200) return;

        const list = response.data?.data;
        if (!Array.isArray(list) || list.length === 0) return;

        // 后端只返回 label/text，图标按顺序轮换，保证界面风格与内置问题一致
        const icons = ['flame-outline', 'water-outline', 'bandage-outline', 'sparkles-outline'];
        const questions = list
          .map((item, index) => {
            const label = typeof item?.label === 'string' ? item.label.trim() : '';
            const text = typeof item?.text === 'string' ? item.text.trim() : '';
            if (!text) return null;
            return {
              icon: icons[index % icons.length],
              label: label || text.slice(0, 8),
              text,
            };
          })
          .filter(Boolean)
          .slice(0, 4);

        if (questions.length > 0) {
          this.quickQuestions = questions;
        }
      } catch (error) {
        // 热门问题不是问诊主流程，失败时不弹提示打扰用户
        console.warn('获取 AI 热门问题失败，继续使用默认问题:', error);
      }
    },

    // 点击快捷提问：把问题写入输入框后直接发送，省去手动输入
    askSuggestion(text) {
      if (this.isChatting) return;
      const box =
        this.$refs.editableContent || document.querySelector('.textarea-wen');
      if (box) box.innerText = text;
      this.userMessage = text;
      this.handleImageAndChat();
    },

    // 图片和文本提交
    async handleImageAndChat(event) {
      event?.preventDefault();
      if (this.isChatting) return;

      const editableContent =
        this.$refs.editableContent || document.querySelector('.textarea-wen');
      const textOnly = (editableContent?.innerText || '')
        .replace(/\u200b|\u00a0/g, '')
        .trim();
      const imageFile = this.selectedImage;

      // 文字和图片都没有时用 Element Plus 提示，不再弹原生 alert
      if (!imageFile && !textOnly) {
        ElMessage.warning('请添加文字或图片内容');
        editableContent?.focus();
        return;
      }

      // 每次发送时重新读取登录态，避免登录前模块加载导致 token 一直为 null
      const token = localStorage.getItem('token');
      if (!token) {
        ElMessage.warning('登录状态已失效，请重新登录');
        this.$router.push('/HomeView');
        return;
      }

      const userId = localStorage.getItem('user_id') || this.userId || 0;
      let chatId = this.chatId || 0;
      const conversation = textOnly || '如何治疗？';

      // 进入页面时不再自动建对话；第一次真正发送消息时再创建，
      // 避免每次登录都留下一条空的“新对话”，也避免把消息写进已删除的对话。
      if (!chatId) {
        chatId = await this.$refs.historyPanel?.createChat?.(true);
        if (!chatId) {
          ElMessage.error('创建对话失败，请稍后重试');
          return;
        }
      }

      // 在插入本条消息前判断是否为该对话的第一问，后续标题只由首问决定。
      const isFirstMessage = this.messages.length === 0;

      // 发送后作废仍在途中的历史记录请求，避免它晚到后覆盖刚发出的消息；
      // 同时为这次发送分配会话序号，供图片问诊识别到旧会话已切走时丢弃响应。
      this.historyRequestId += 1;
      const sessionId = ++this.streamSessionId;

      this.messages.push({
        sender: 'user',
        text: textOnly || (imageFile ? '请帮我看看这张图片' : ''),
        image: imageFile ? URL.createObjectURL(imageFile) : null,
        side: 'right',
      });
      const aiMessage = this.pushPendingMessage();

      this.isChatting = true;
      this.isThinking = true;
      this.clearComposer();
      this.scrollThreadToBottom();

      // 首条消息立即生成标题；请求放到后台，不阻塞图片/文字问诊。
      this.applyAutoChatTitle({
        chatId,
        text: textOnly,
        hasImage: Boolean(imageFile),
        isFirstMessage,
      });

      if (imageFile) {
        await this.sendImageQuestion({ imageFile, conversation, chatId, userId, token, aiMessage, sessionId });
        return;
      }

      // 纯文字问诊走 SSE 流式接口，收到第一个内容块前按钮一直显示“思考中”
      await this.startTextStream(aiMessage, { conversation, chatId, userId, token });
    },

    // 读取 SSE 响应，把 AI 回答逐块追加到同一条消息
    async startTextStream(aiMessage, { conversation, chatId, userId, token }) {
      const controller = new AbortController();
      this.activeStreamController = controller;
      this.isThinking = true;

      const queryParams = new URLSearchParams({
        conversation,
        chat_id: chatId,
        user_id: userId,
      }).toString();

      const processBlock = (block) => {
        if (!block.trim()) return;

        let eventName = 'message';
        const dataLines = [];

        block.split(/\r?\n/).forEach((line) => {
          if (line.startsWith('event:')) {
            eventName = line.slice(6).trim();
          } else if (line.startsWith('data:')) {
            dataLines.push(line.slice(5).replace(/^ /, ''));
          }
        });

        const rawData = dataLines.join('\n');
        if (!rawData || rawData === '[DONE]') return;

        let payload = null;
        try {
          payload = JSON.parse(rawData);
        } catch {
          // 兼容没有 JSON 包裹的纯文本 data
          payload = null;
        }

        if (eventName === 'error') {
          const message = payload?.message || rawData || 'AI 服务暂时不可用，请稍后再试。';
          aiMessage.thinking = false;
          aiMessage.error = message;
          this.isThinking = false;
          ElMessage.error(message);
          return;
        }

        if (eventName === 'thoughts') {
          const thoughts = payload?.thoughts || payload?.content || '';
          if (thoughts) {
            aiMessage.thoughts += thoughts;
            this.scrollThreadToBottom();
          }
          return;
        }

        if (eventName === 'done') return;

        if (eventName === 'suggestions') {
          aiMessage.suggestions = this.normalizeSuggestions(payload?.suggestions);
          this.scrollThreadToBottom();
          return;
        }

        const content = typeof payload?.content === 'string' ? payload.content : rawData;
        if (!content) return;

        // 第一个内容块到达后才把按钮从“思考中”切回“发送”
        aiMessage.thinking = false;
        this.isThinking = false;
        aiMessage.response += content;
        this.scrollThreadToBottom();
      };

      try {
        const response = await fetch(
          `${API_BASE}/predict/only/text/stream?${queryParams}`,
          {
            method: 'GET',
            headers: { Authorization: `Bearer ${token}` },
            signal: controller.signal,
          }
        );

        if (!response.ok || !response.body) {
          throw new Error(`流式接口返回 ${response.status}`);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder('utf-8');
        let buffer = '';

        while (true) {
          const { value, done } = await reader.read();
          if (done) break;

          buffer += decoder.decode(value, { stream: true });
          const blocks = buffer.split(/\r?\n\r?\n/);
          buffer = blocks.pop() || '';
          blocks.forEach(processBlock);
        }

        buffer += decoder.decode();
        if (buffer.trim()) processBlock(buffer);

        if (!aiMessage.response && !aiMessage.error) {
          aiMessage.thinking = false;
          aiMessage.error = 'AI 未返回内容，请稍后再试。';
          ElMessage.error(aiMessage.error);
        }
      } catch (error) {
        if (error.name === 'AbortError') return;
        this.failPendingMessage(aiMessage, error);
      } finally {
        if (this.activeStreamController === controller) {
          this.activeStreamController = null;
        }
        aiMessage.thinking = false;
        this.isThinking = false;
        this.isChatting = false;
        this.scrollThreadToBottom();
      }
    },

    // 处理图片粘贴
    handlePaste(event) {
      const items = event.clipboardData.items;
      for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item.type.indexOf('image') !== -1) {
          event.preventDefault();
          const file = item.getAsFile();
          // 粘贴图片也走图片问诊接口，否则点发送时会被当成“没有内容”
          this.selectedImage = file;
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

    // 停止当前仍在进行的回答：切换/新建/删除对话时调用，
    // 避免上一条对话的流式内容继续写进新打开的会话。
    stopActiveStreams() {
      // 让所有尚未返回的旧发送请求失效
      this.streamSessionId += 1;
      if (this.activeStreamController) {
        this.activeStreamController.abort();
        this.activeStreamController = null;
      }
      if (this.activeEventSource) {
        this.activeEventSource.close();
        this.activeEventSource = null;
      }
      this.isChatting = false;
      this.isThinking = false;
    },

    // 把后端 image_ask_messages 的“一条问答记录”拆成用户气泡 + AI 气泡，
    // 这样历史会话与刚发送时的渲染结构完全一致。
    historyRowsToMessages(rows) {
      const list = [];
      (Array.isArray(rows) ? rows : []).forEach((row) => {
        const ask = typeof row?.ask === 'string' ? row.ask.trim() : '';
        const answer = typeof row?.message === 'string' ? row.message.trim() : '';
        // 后端字段名是 url，兼容部分环境返回 materials 的情况
        const image = row?.url || row?.materials || '';

        if (ask || image) {
          list.push({
            sender: 'user',
            side: 'right',
            text: ask,
            image: image || null,
          });
        }

        if (answer) {
          list.push({
            sender: 'system',
            side: 'left',
            thinking: false,
            thoughts: '',
            response: answer,
            suggestions: [],
            text: '',
            error: '',
            meta: '',
          });
        }
      });
      return list;
    },

    // 历史侧栏选中某个会话时，拉取该会话的消息，并同步右侧标题
    async fetchChatMessages(chatId, name) {
      // 记录本次请求序号，连续点击多个会话时只渲染最后一次的结果
      const requestId = ++this.historyRequestId;

      // 切换会话前先停止上一条对话的回答，并立即清空右侧，
      // 避免加载新记录期间仍显示上一个会话的气泡。
      this.stopActiveStreams();
      this.messages = [];

      // 选中历史对话后同步当前 chatId，后续发送消息才会写回这条对话
      this.chatId = chatId;
      this.chatTitle = name || '新对话';
      this.cancelChatTitleEdit();
      localStorage.setItem('newChatId', chatId);

      try {
        const response = await axios.get('/users/Check_image_ask_messages', {
          params: { chat_id: chatId, page: 1, size: 50 },
          headers: { Authorization: `Bearer ${localStorage.getItem('token')}` }
        });
        // 期间又切换了别的会话，丢弃这次过期的响应
        if (requestId !== this.historyRequestId) return;
        if (response.data.code === 200) {
          this.messages = this.historyRowsToMessages(response.data.data);
          this.scrollThreadToBottom();
        } else {
          console.error('获取聊天记录失败:', response.data.message);
          ElMessage.error(response.data.message || '获取聊天记录失败');
        }
      } catch (error) {
        if (requestId !== this.historyRequestId) return;
        console.error('获取聊天记录失败:', error);
        ElMessage.error('获取聊天记录失败，请稍后重试');
      }
    }
  },

  beforeUnmount() {
    // 标记已卸载，阻止仍在途中的热门问题请求回写组件状态
    this.isUnmounted = true;
    // 离开页面时断开流式连接，避免后台继续推流
    if (this.activeEventSource) {
      this.activeEventSource.close();
      this.activeEventSource = null;
    }
    if (this.activeStreamController) {
      this.activeStreamController.abort();
      this.activeStreamController = null;
    }
  }
};
</script>

<template>
  <div class="chat-page">
    <AppNavbar />

    <main class="chat-main">
      <div class="chat-layout" :class="{ 'chat-layout--full': !showHistory }">
        <MyComponent
          v-show="showHistory"
          ref="historyPanel"
          @chat-created="handleChatCreated"
          @chat-selected="fetchChatMessages"
          @chat-title-updated="handleChatTitleUpdated"
          @chat-deleted="handleChatDeleted"
          @toggle-sidebar="toggleHistorySidebar"
        />

        <section id="qwen-chat" class="chat-container">
          <div class="chat-header">
            <!-- 折叠/展开历史侧栏按钮：始终可见 -->
            <button
              type="button"
              class="sidebar-toggle"
              :title="showHistory ? '隐藏历史对话' : '展开历史对话'"
              :aria-label="showHistory ? '隐藏历史对话' : '展开历史对话'"
              @click="toggleHistorySidebar"
            >
              <ion-icon :name="showHistory ? 'menu-outline' : 'menu-open-outline'"></ion-icon>
            </button>

            <div class="chat-title" :class="{ 'is-editing': isEditingTitle }">
              <!-- 标题区域点击进入编辑；保存后同时更新头部和左侧历史列表 -->
              <input
                v-if="isEditingTitle"
                ref="titleInput"
                v-model="titleDraft"
                class="chat-title-input"
                maxlength="30"
                aria-label="修改对话标题"
                @blur="saveChatTitle"
                @keydown.enter.prevent="saveChatTitle"
                @keydown.esc.prevent="cancelChatTitleEdit"
              />
              <template v-else>
                <h2
                  :title="chatId ? '点击修改对话标题' : chatTitle"
                  :class="{ 'is-editable': chatId }"
                  @click="startEditingTitle"
                >
                  {{ chatTitle || '新对话' }}
                </h2>
                <button
                  v-if="chatId"
                  type="button"
                  class="chat-title-edit"
                  title="修改对话标题"
                  aria-label="修改对话标题"
                  @click="startEditingTitle"
                >
                  <ion-icon name="create-outline"></ion-icon>
                </button>
              </template>
            </div>

            <!-- 右侧状态：与左侧对话标题分开，标题过长时自动省略 -->
            <div class="chat-header-status">
              <span class="chat-status-dot"></span>
              <span>AI 在线回复</span>
            </div>
          </div>

          <div class="box5">
            <div class="chat-thread">
              <div v-if="messages.length === 0" class="chat-empty">
                <span class="chat-empty-icon"><ion-icon name="sparkles-outline"></ion-icon></span>
                <h3>先说说你的皮肤困扰</h3>
                <p>描述部位、外观和持续时间，或上传一张清晰的患处照片。信息越具体，分析越贴合。</p>
                <div class="chat-suggestions">
                  <button
                    v-for="(item, index) in quickQuestions"
                    :key="`${item.label}-${index}`"
                    type="button"
                    class="chat-suggestion"
                    :disabled="isChatting"
                    @click="askSuggestion(item.text)"
                  >
                    <ion-icon :name="item.icon"></ion-icon>
                    <span>{{ item.label }}</span>
                  </button>
                </div>
              </div>

              <!-- 显示聊天消息；原有点击/上传/SSE 数据字段全部保留 -->
              <div
                v-for="(message, index) in messages"
                :key="index"
                :class="['chat-message', message.side]"
              >
                <div v-if="message.side === 'left'" class="message-avatar">AI</div>
                <div class="message-body">
                  <div v-if="message.image">
                    <img :src="message.image" alt="患处图片预览" class="chat-image" />
                  </div>

                  <div v-if="message.meta" class="message-meta">{{ message.meta }}</div>

                  <div v-if="message.structure" class="message-structure">
                    <span class="message-structure-label">图像结构</span>{{ message.structure }}
                  </div>

                  <div v-if="message.thinking" class="thinking-indicator">
                    <span class="thinking-spark" aria-hidden="true">✦</span>
                    <span>思考中<span class="thinking-ellipsis">...</span></span>
                  </div>

                  <div v-if="message.thoughts" class="thoughts-block">
                    <h4>推理过程</h4>
                    <div
                      class="markdown-content"
                      style="white-space: pre-wrap;"
                      v-html="parseMarkdown(message.thoughts)"
                    ></div>
                  </div>

                  <div
                    v-if="message.response"
                    class="markdown-content"
                    v-html="parseMarkdown(message.response)"
                  ></div>

                  <div v-else-if="message.text" class="markdown-content user-text">
                    {{ message.text }}
                  </div>

                  <div v-if="message.error" class="message-error">{{ message.error }}</div>

                  <!-- 回答下方的相关追问，点击后直接作为新问题发送 -->
                  <div v-if="message.suggestions && message.suggestions.length" class="follow-up-questions">
                    <p class="follow-up-title">你可能还想问</p>
                    <div class="follow-up-list">
                      <button
                        v-for="(question, questionIndex) in message.suggestions"
                        :key="questionIndex"
                        type="button"
                        class="follow-up-chip"
                        :disabled="isChatting"
                        @click="askSuggestion(question)"
                      >
                        {{ question }}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="selectedImage" class="image-preview">
            <span class="image-preview-icon"><ion-icon name="image-outline"></ion-icon></span>
            <p class="image-filename">{{ selectedImage.name }}</p>
          </div>

          <div class="anniu">
            <form @submit.prevent="handleImageAndChat" class="chat-form">
              <div class="composer">
                <div
                  contenteditable="true"
                  @input="updateUserMessage"
                  placeholder="描述你的症状，或上传患处照片..."
                  class="textarea-wen"
                  @paste="handlePaste"
                  ref="editableContent"
                  @keydown.enter="handleComposerEnter"
                ></div>

                <div class="composer-toolbar">
                  <div class="composer-tools">
                    <button type="button" class="tool-btn" title="语音输入" @click="startVoiceSearch">
                      <ion-icon name="mic-outline"></ion-icon>
                    </button>

                    <button type="button" class="tool-btn" title="上传文件" @click="triggerFileUpload">
                      <ion-icon name="folder-outline"></ion-icon>
                    </button>
                    <input type="file" ref="archiveInput" style="display: none" @change="handleFileUpload">

                    <button type="button" class="tool-btn tool-btn--accent" title="上传患处照片" @click="triggerImageUpload">
                      <ion-icon name="image-outline"></ion-icon>
                    </button>
                    <input type="file" ref="imageInput" style="display: none;" @change="onFileChange" accept="image/*">

                    <span class="composer-hint">支持图片与文字</span>
                  </div>

                  <button type="submit" :disabled="isChatting" class="btn-inside">
                    <ion-icon :name="isChatting ? 'reload-circle' : 'arrow-redo'"></ion-icon>
                    <span>{{ isThinking ? '思考中' : isChatting ? '回答中' : '发送' }}</span>
                  </button>
                </div>
              </div>
            </form>
          </div>
        </section>
      </div>

      <p class="disclaimer">AI 建议仅供参考，如有不适请及时就医并遵医嘱</p>
    </main>
  </div>
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

.disclaimer {
  text-align: center;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin: 12px 0 16px;
}

</style>

<style scoped>
/* =========================================================
   问诊页最终布局：顶栏 + 历史侧栏 + 聊天卡片 + 底部工具栏
   这一层放在原样式之后，用来覆盖旧版 absolute 定位和背景图。
   ========================================================= */
.chat-page {
  min-height: 100vh;
  height: 100vh;
  height: 100dvh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--ai-mint-2);
}

.chat-main {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  width: min(1440px, calc(100% - 32px));
  margin: 0 auto;
  padding: 18px 0 12px;
}

.chat-layout {
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: stretch;
  gap: 18px;
}

.chat-container {
  flex: 1;
  min-width: 0;
  height: 100% !important; /* 由 .chat-layout 的可视高度决定，不再用 100vh 扣导航 */
  min-height: 0;
  margin: 0 !important;
  padding: 0 !important;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #fff !important;
  background-image: none !important;
  border: 1px solid var(--ai-line);
  border-radius: 24px;
  box-shadow: var(--ai-shadow);
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 52px;
  padding: 10px 20px;
  border-bottom: 1px solid var(--ai-line);
  background: #fff;
}

/* 折叠/展开历史侧栏按钮：chat-header 最左侧的小图标 */
.sidebar-toggle {
  flex: 0 0 auto;
  width: 32px;
  height: 32px;
  display: inline-grid;
  place-items: center;
  padding: 0;
  border: 1px solid var(--ai-line);
  border-radius: 8px;
  background: #fff;
  color: var(--ai-muted);
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease, transform 0.18s ease;
}

.sidebar-toggle:hover {
  border-color: var(--ai-primary);
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  transform: scale(1.05);
}

.sidebar-toggle:active {
  transform: scale(0.95);
}

.sidebar-toggle ion-icon {
  font-size: 18px;
}

.chat-layout {
  transition: gap 0.22s ease;
}

.chat-layout--full {
  gap: 0;
}

.chat-title {
  flex: 0 1 auto;
  min-width: 0;
  max-width: 58%;
  display: flex;
  align-items: center;
}

/* 编辑标题时，让输入框在 chat-title 内部居中；同时放开宽度限制，让输入框更宽 */
.chat-title.is-editing {
  justify-content: center;
  width: min(460px, 68%);
  max-width: 68%;
}

.chat-title h2 {
  flex: 1;
  min-width: 0;
  max-width: 100%;
  margin: 0 !important;
  overflow: hidden;
  color: var(--ai-ink);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
  /* 标题文字居中，与编辑状态的输入框对齐 */
  text-align: center;
}

/* 标题可点击改名，悬浮时给出颜色反馈 */
.chat-title h2.is-editable {
  cursor: text;
  transition: color 0.18s ease;
}

.chat-title h2.is-editable:hover {
  color: var(--ai-primary);
}

.chat-title-edit {
  flex: 0 0 auto;
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  margin-left: 6px;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: #8aa4ac;
  font-size: 17px;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

/* 头部右侧状态标签 */
.chat-header-status {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border: 1px solid var(--ai-line);
  border-radius: 99px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.chat-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ai-primary);
}

.chat-title-edit:hover {
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
}

.chat-title-input {
  width: 80%;
  height: 34px;
  padding: 0 10px;
  border: 1px solid rgba(23, 184, 166, 0.55);
  border-radius: 9px;
  background: #f7fdfc;
  color: var(--ai-ink);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.4;
  outline: none;
  box-shadow: 0 0 0 3px rgba(23, 184, 166, 0.1);
  /* 输入框内文字居中，与标题展示时的居中效果对齐 */
  text-align: center;
}

.box5 {
  flex: 1;
  height: auto !important;
  min-height: 0;
  overflow-y: auto;
  /* 始终预留滚动条宽度：回答变长出现滚动条时，气泡不会突然变窄重排 */
  scrollbar-gutter: stable;
  padding: 22px !important;
  border-radius: 0 !important;
  background: #f8fcfb !important;
}

.chat-thread {
  width: min(860px, 100%);
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.chat-empty {
  margin: 7vh auto 0;
  max-width: 520px;
  text-align: center;
}

.chat-empty-icon {
  width: 58px;
  height: 58px;
  display: grid;
  place-items: center;
  margin: 0 auto 14px;
  border-radius: 18px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 27px;
}

.chat-empty h3 {
  margin: 0 0 7px;
  color: var(--ai-ink);
  font-size: 20px;
}

.chat-empty p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 13px;
  line-height: 1.75;
}

.chat-suggestions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 9px;
  margin-top: 20px;
}

.chat-suggestion {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border: 1px solid var(--ai-line);
  border-radius: 99px;
  background: #fff;
  color: var(--ai-ink-soft);
  font-family: inherit;
  font-size: 13px;
  line-height: 1.4;
  white-space: nowrap;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.chat-suggestion:hover:not(:disabled),
.chat-suggestion:focus-visible:not(:disabled) {
  border-color: var(--ai-primary);
  background: var(--ai-primary-soft);
  color: var(--ai-ink);
}

.chat-suggestion:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.chat-suggestion ion-icon {
  flex: 0 0 auto;
  color: var(--ai-primary);
  font-size: 15px;
}

/* AI 回答下方的相关追问 */
.follow-up-questions {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px dashed var(--ai-line);
}

.follow-up-title {
  margin: 0 0 8px;
  color: var(--ai-muted);
  font-size: 12px;
  letter-spacing: 0.02em;
}

.follow-up-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.follow-up-chip {
  max-width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--ai-line);
  border-radius: 10px;
  background: #fff;
  color: var(--ai-ink-soft);
  font-family: inherit;
  font-size: 13px;
  line-height: 1.5;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.follow-up-chip:hover:not(:disabled),
.follow-up-chip:focus-visible:not(:disabled) {
  border-color: var(--ai-primary);
  background: var(--ai-primary-soft);
  color: var(--ai-ink);
}

.follow-up-chip:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.chat-message {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  max-width: 100% !important;
  padding: 0 !important;
  margin: 0 !important;
  border-radius: 0 !important;
  background: transparent !important;
  color: var(--ai-ink) !important;
  font-size: 14px !important;
}

.chat-message.right {
  flex-direction: row-reverse;
  align-self: flex-end;
}

.chat-message.left {
  align-self: flex-start;
  /* 让 AI 行占满聊天区宽度，气泡的固定宽度才能按容器计算，
     否则行宽会随内容收缩，流式时气泡仍会被撑大。 */
  width: 100%;
}

.message-avatar {
  flex: 0 0 30px;
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  margin-top: 2px;
  border-radius: 10px;
  background: linear-gradient(135deg, var(--ai-primary), var(--ai-teal));
  color: #fff;
  font-size: 10px;
  font-weight: 750;
}

.message-body {
  max-width: min(720px, 82%);
}

/* 用户发送的问题气泡自适应宽度，最宽 400px，超出后自动换行 */
.chat-message.right .message-body {
  max-width: min(500px, calc(100% - 40px));
  min-width: 200px;
  width: fit-content;
}

.message-meta {
  margin-bottom: 8px;
  color: var(--ai-muted);
  font-size: 12px;
}

.message-structure {
  margin: 0 0 8px;
  padding: 7px 10px;
  border-left: 3px solid var(--ai-accent, #7c9cbf);
  background: rgba(124, 156, 191, 0.08);
  border-radius: 6px;
  color: var(--ai-muted);
  font-size: 12px;
  line-height: 1.6;
}

.message-structure-label {
  display: inline-block;
  margin-right: 6px;
  color: var(--ai-ink-soft);
  font-weight: 600;
}

.thinking-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ai-ink-soft);
  font-size: 13px;
  white-space: nowrap;
}

.thinking-spark {
  color: var(--ai-primary);
  font-size: 14px;
  animation: thinking-pulse 1.4s ease-in-out infinite;
}

.thinking-ellipsis {
  display: inline-block;
  width: 14px;
  text-align: left;
  animation: thinking-blink 1.3s ease-in-out infinite;
}

.thoughts-block {
  margin-bottom: 10px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #f4faf8;
}

.thoughts-block h4 {
  margin: 0 0 6px;
  color: var(--ai-ink);
  font-size: 12px;
  font-weight: 700;
}

.thoughts-block .markdown-content {
  color: var(--ai-ink-soft);
  font-size: 12px;
}

.message-error {
  color: #c0504d;
  font-size: 13px;
  line-height: 1.7;
}

@keyframes thinking-pulse {
  0%, 100% { opacity: 0.45; transform: scale(0.92); }
  50% { opacity: 1; transform: scale(1.06); }
}

@keyframes thinking-blink {
  0%, 100% { opacity: 0.25; }
  50% { opacity: 1; }
}

.chat-message.left .message-body {
  /* AI 气泡使用固定宽度，流式输出时不会随文字多少忽宽忽窄；
     40px 预留给左侧 AI 头像和间距。 */
  width: min(720px, calc(100% - 40px));
  max-width: min(720px, calc(100% - 40px));
  min-height: 62px;
  padding: 16px 18px;
  border: 1px solid rgba(15, 111, 122, 0.14);
  border-radius: 5px 20px 20px 20px;
  background: linear-gradient(180deg, #fff 0%, #fbfefd 100%);
  box-shadow: 0 10px 28px rgba(15, 111, 122, 0.08), 0 2px 6px rgba(15, 111, 122, 0.04);
}

.chat-message.right .message-body {
  padding: 13px 16px;
  border-radius: 18px 4px 18px 18px;
  background: linear-gradient(135deg, var(--ai-primary), #138a8f);
  color: #fff;
  box-shadow: 0 10px 24px rgba(15, 111, 122, 0.18);
}

.chat-message.right .markdown-content,
.chat-message.right .markdown-content p,
.chat-message.right .markdown-content strong,
.chat-message.right .markdown-content li {
  color: #fff !important;
}

/* 用户输入按纯文本展示，保留换行但不解析 HTML */
.chat-message.right .user-text {
  white-space: pre-wrap;
}

.chat-image {
  max-width: 260px;
  margin: 0 0 8px !important;
  border-radius: 12px;
}

.chat-message.right .chat-image {
  margin-left: auto !important;
}

.loading-indicator {
  display: block !important;
  color: var(--ai-ink-soft) !important;
  font-size: 13px !important;
}

.loading-indicator .title-content {
  margin-bottom: 10px;
}

.loading-indicator .title-content h2 {
  margin: 0 0 6px;
  color: var(--ai-ink);
  font-size: 13px;
  font-weight: 700;
}

.loading-indicator .title-content > div {
  padding: 10px 12px !important;
  border-radius: 10px !important;
  background: #f4faf8 !important;
  color: var(--ai-ink-soft) !important;
  font-size: 12px;
  line-height: 1.7;
}

.typing-dots {
  display: inline-flex;
  gap: 4px;
  padding: 6px 2px;
}

.typing-dots span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--ai-primary);
  opacity: 0.45;
  animation: typing 1.2s infinite ease-in-out;
}

.typing-dots span:nth-child(2) {
  animation-delay: 0.15s;
}

.typing-dots span:nth-child(3) {
  animation-delay: 0.3s;
}

@keyframes typing {
  0%, 80%, 100% { transform: translateY(0); opacity: 0.35; }
  40% { transform: translateY(-4px); opacity: 1; }
}

.markdown-content {
  color: var(--ai-ink-soft);
  font-size: 14px;
  line-height: 1.85;
  overflow-wrap: anywhere;
}

.markdown-content :deep(p) {
  margin: 8px 0;
}

.markdown-content :deep(p:first-child) {
  margin-top: 0;
}

.markdown-content :deep(p:last-child) {
  margin-bottom: 0;
}

.markdown-content :deep(h2),
.markdown-content :deep(h3) {
  margin: 16px 0 8px;
  padding: 0 0 0 9px;
  border-bottom: 0;
  border-left: 3px solid var(--ai-teal);
  color: var(--ai-ink);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.5;
}

.markdown-content :deep(h2:first-child),
.markdown-content :deep(h3:first-child) {
  margin-top: 0;
}

.markdown-content :deep(ul),
.markdown-content :deep(ol) {
  margin: 8px 0;
  padding-left: 20px;
}

.markdown-content :deep(li) {
  margin: 5px 0;
  padding-left: 2px;
}

.markdown-content :deep(li::marker) {
  color: var(--ai-teal);
}

.markdown-content :deep(strong) {
  color: var(--ai-ink);
  font-weight: 700;
}

.markdown-content :deep(code) {
  padding: 2px 5px;
  border-radius: 5px;
  background: #eef7f5;
  color: var(--ai-primary-deep);
  font-size: 12px;
}

.markdown-content :deep(pre) {
  overflow-x: auto;
  margin: 10px 0;
  padding: 11px 13px;
  border-radius: 10px;
  background: #f4faf8;
}

.markdown-content :deep(pre code) {
  padding: 0;
  background: transparent;
}

.markdown-content :deep(blockquote) {
  margin: 10px 0;
  padding: 7px 12px;
  border-left: 3px solid rgba(15, 111, 122, 0.28);
  border-radius: 0 8px 8px 0;
  background: #f6fbfa;
  color: var(--ai-muted);
}

.image-preview {
  display: flex;
  align-items: center;
  gap: 8px;
  width: min(860px, calc(100% - 44px));
  margin: 0 auto 8px;
  padding: 8px 12px;
  border: 1px solid var(--ai-line);
  border-radius: 12px;
  background: #f7fbfa;
}

.image-preview-icon {
  display: inline-flex;
  color: var(--ai-primary);
  font-size: 17px;
}

.image-filename {
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 12px;
}

.anniu {
  position: relative;
  width: 100%;
  padding: 8px 22px 14px;
  background: #fff;
}

/* 输入区与 .box5 使用相同左右内边距，composer 占满聊天容器 */
.chat-form {
  display: block; /* 覆盖旧版横向 flex，避免 .composer 被压缩 */
  width: 100%;
  max-width: none;
  margin: 0;
}

.composer {
  padding: 6px 10px;
  border: 1px solid var(--ai-line);
  border-radius: 16px;
  background: #f9fdfc;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.composer:focus-within {
  border-color: rgba(23, 184, 166, 0.6);
  background: #fff;
  box-shadow: 0 0 0 4px rgba(23, 184, 166, 0.1);
}

.textarea-wen {
  width: 100% !important;
  height: 48px !important;
  min-height: 48px;
  max-height: 48px;
  overflow-y: auto;
  padding: 2px 4px 4px !important;
  border: 0 !important;
  border-radius: 0 !important;
  background: transparent !important;
  color: var(--ai-ink) !important;
  font-size: 14px !important;
  line-height: 1.6;
  resize: none;
  outline: none !important;
  box-shadow: none !important;
}

.textarea-wen:empty::before {
  content: attr(placeholder);
  color: #9bb0b8;
  pointer-events: none;
}

.textarea-wen img {
  max-width: 160px;
  max-height: 120px;
  border-radius: 10px;
}

.composer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 4px;
  border-top: 1px solid var(--ai-line);
}

.composer-tools {
  display: flex;
  align-items: center;
  gap: 5px;
}

.tool-btn {
  width: 30px;
  height: 30px;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--ai-ink-soft);
  font-size: 17px;
  cursor: pointer;
  transition: background 0.18s ease, color 0.18s ease;
}

.tool-btn:hover {
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
}

.tool-btn--accent {
  color: var(--ai-primary);
}

.composer-hint {
  margin-left: 5px;
  color: #9bb0b8;
  font-size: 11px;
}

.btn-inside {
  position: static !important;
  transform: none !important;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 76px;
  height: 32px;
  padding: 0 14px !important;
  border: 0;
  border-radius: 11px;
  background: linear-gradient(135deg, var(--ai-primary), var(--ai-teal)) !important;
  color: #fff !important;
  font-size: 13px !important;
  font-weight: 650;
  cursor: pointer;
  box-shadow: 0 8px 18px rgba(15, 111, 122, 0.2);
}

.btn-inside ion-icon {
  font-size: 17px;
}

.btn-inside:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.btn-inside:disabled ion-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.disclaimer {
  margin: 10px 0 0 !important;
  color: var(--ai-muted) !important;
  font-size: 11px !important;
}

/* 覆盖旧模板里可能残留的绝对定位工具栏 */
.picture,
.voice-btn,
.image-btn {
  position: static !important;
  display: none !important;
}

@media (max-width: 1100px) {
  .chat-layout {
    gap: 12px;
  }
}

@media (max-width: 820px) {
  .chat-main {
    width: calc(100% - 18px);
    padding-top: 10px;
  }

  /* 共享导航在移动端会换成两行（约 112px），这里同步扣除，避免输入区被顶出视口 */
  .chat-container {
    border-radius: 20px;
  }

  /* 移动端去掉侧栏后，输入区与问答区都按 16px 内边距对齐 */
  .anniu {
    padding-left: 16px;
    padding-right: 16px;
  }

  .textarea-wen {
    height: 44px !important;
    min-height: 44px;
    max-height: 44px;
  }

  .image-preview {
    width: calc(100% - 32px);
  }

  .chat-header {
    min-height: 48px;
    padding: 8px 14px;
  }

  /* 窄屏下隐藏右侧状态，把空间留给对话标题 */
  .chat-header-status {
    display: none;
  }

  .box5 {
    padding: 16px !important;
  }

  .message-body {
    max-width: 88%;
  }

  .composer-hint {
    display: none;
  }
}
</style>