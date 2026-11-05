<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import axios from 'axios';
import { marked } from 'marked';
import {
  ElAvatar,
  ElButton,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage
} from 'element-plus';

import { useRouter } from 'vue-router';
import AppNavbar from '../components/AppNavbar.vue';

// token 在打开页面时读取一次；后续请求都带同一个 Bearer Token。
const token = localStorage.getItem('token');
const router = useRouter();

const avatarUrl = ref('');
const avatarInput = ref(null);

// 表单同时承载“个人信息”和“个人皮肤病情况”两组字段。
const formData = ref({
  account: '',
  email: '',
  name: '',
  age: '',
  gender: '',
  phone_number: '',
  medicalHistory: '',
  allergyHistory: ''
});

// 个人信息和皮肤病史共用同一个编辑状态。
const isEditable = ref(false);
// 提交中状态用于按钮 loading 和防止重复点击。
const isSubmitting = ref(false);
const isUploadingAvatar = ref(false);

// AI 健康档案：archiveError 与 healthArchive 二选一展示。
const healthArchive = ref('');
const archiveLoading = ref(false);
const archiveError = ref('');
// 每次请求分配一个序号；旧请求晚到时直接丢弃，避免覆盖新结果。
let archiveRequestId = 0;

// 档案缓存：同一登录会话内再次进入个人中心时直接使用缓存，
// 只有重新编辑并提交个人信息，或点击“重新生成”时才真正调用模型。
const ARCHIVE_CACHE_KEY = 'ai_health_archive_cache';

const readCachedArchive = () => {
  try {
    const raw = sessionStorage.getItem(ARCHIVE_CACHE_KEY);
    if (!raw) return '';
    const cache = JSON.parse(raw);
    // token 绑定登录态；重新登录后 token 变化，不会复用上一次登录的档案。
    if (cache?.token === token && typeof cache.archive === 'string') {
      return cache.archive;
    }
  } catch (error) {
    console.warn('读取 AI 健康档案缓存失败', error);
  }
  return '';
};

const writeCachedArchive = (archive) => {
  if (!archive) return;
  try {
    sessionStorage.setItem(ARCHIVE_CACHE_KEY, JSON.stringify({
      token,
      archive,
      savedAt: Date.now()
    }));
  } catch (error) {
    console.warn('写入 AI 健康档案缓存失败', error);
  }
};

const displayName = computed(() => {
  return formData.value.name?.trim() || formData.value.account?.trim() || '未命名用户';
});

// 把 AI 返回的 Markdown 渲染成 HTML。
// marked 默认允许 HTML 透传，而档案最终进入 v-html，
// 因此这里移除脚本类标签、事件属性和 javascript: 链接，降低 XSS 风险。
const sanitizeHtml = (html) => {
  const doc = new DOMParser().parseFromString(html, 'text/html');

  doc.querySelectorAll('script, iframe, object, embed, style, link, meta, form, input, button')
    .forEach((node) => node.remove());

  doc.querySelectorAll('*').forEach((node) => {
    [...node.attributes].forEach((attribute) => {
      const name = attribute.name.toLowerCase();
      const value = attribute.value.trim().toLowerCase();
      if (name.startsWith('on') || value.startsWith('javascript:')) {
        node.removeAttribute(attribute.name);
      }
    });
  });

  return doc.body.innerHTML;
};

const renderMarkdown = (text) => sanitizeHtml(
  marked.parse(text || '', { breaks: true, gfm: true })
);

const enableEdit = () => {
  isEditable.value = true;
};

// 查询个人基本信息。
const loadUserData = async () => {
  try {
    const response = await axios.get('/users/check_users_information', {
      headers: { Authorization: `Bearer ${token}` }
    });

    if (response.data.code === 200) {
      const data = response.data.data || {};
      formData.value = {
        account: data.account || '',
        email: data.email || '',
        name: data.name || '',
        age: data.age || '',
        gender: data.gender || '',
        phone_number: data.phone_number || '',
        medicalHistory: data.medical_history || '',
        allergyHistory: data.allergies || ''
      };

      avatarUrl.value = data.avatar_url
        || 'https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png';
    }
  } catch (error) {
    console.error('个人信息请求失败', error);
  }
};

// 生成 AI 健康档案。加载失败只影响档案区域，不影响页面其他内容。
const loadHealthArchive = async () => {
  if (!token) {
    return;
  }

  const requestId = ++archiveRequestId;
  archiveLoading.value = true;
  archiveError.value = '';

  try {
    const response = await axios.get('/users/health_archive', {
      headers: { Authorization: `Bearer ${token}` },
      // 模型需要汇总多条问诊并生成更长正文，给足时间；超时后提示用户重试。
      timeout: 95000
    });

    // 组件已卸载或有更新的请求时，丢弃这次结果。
    if (requestId !== archiveRequestId) {
      return;
    }

    if (response.data.code === 200) {
      healthArchive.value = response.data.data?.archive || '';
      // 生成成功后写入会话缓存，后续再次进入页面不再调用模型。
      writeCachedArchive(healthArchive.value);
    } else {
      healthArchive.value = '';
      archiveError.value = response.data.error || 'AI 健康档案生成失败，请稍后重试';
    }
  } catch (error) {
    if (requestId !== archiveRequestId) {
      return;
    }
    console.error('健康档案请求失败', error);
    healthArchive.value = '';
    archiveError.value = error.code === 'ECONNABORTED'
      ? 'AI 整理超时，请点击“重新生成”重试'
      : 'AI 健康档案生成失败，请稍后重试';
  } finally {
    // 只有最新请求才能结束加载状态。
    if (requestId === archiveRequestId) {
      archiveLoading.value = false;
    }
  }
};

// 统一提交个人信息和皮肤病史；两组字段都保存成功后只刷新一次 AI 档案。
const submitAll = async () => {
  if (!isEditable.value || isSubmitting.value) {
    return;
  }

  isSubmitting.value = true;
  try {
    const [profileResponse, diseaseResponse] = await Promise.all([
      axios.post(
        '/users/updates_users_information',
        {
          account: formData.value.account.trim(),
          email: formData.value.email.trim(),
          name: formData.value.name.trim(),
          age: Number(formData.value.age) || 0,
          gender: formData.value.gender.trim(),
          phone_number: formData.value.phone_number.trim()
        },
        { headers: { Authorization: `Bearer ${token}` } }
      ),
      axios.post(
        '/users/create_disease_information',
        {
          medical_history: formData.value.medicalHistory.trim(),
          allergies: formData.value.allergyHistory.trim()
        },
        { headers: { Authorization: `Bearer ${token}` } }
      )
    ]);

    const failedResponse = [profileResponse, diseaseResponse]
      .find((response) => response.data.code !== 200);

    if (failedResponse) {
      ElMessage.error(
        `更新失败：${failedResponse.data.error || failedResponse.data.message || '未知错误'}`
      );
      return;
    }

    ElMessage.success('个人信息和健康信息更新成功');
    // 提交成功后回到只读状态，编辑按钮恢复可用。
    isEditable.value = false;
    await loadUserData();
    loadHealthArchive();
  } catch (error) {
    console.error('个人信息提交失败', error);
    ElMessage.error('提交失败，请稍后再试');
  } finally {
    isSubmitting.value = false;
  }
};

const openAvatarPicker = () => {
  if (isUploadingAvatar.value) {
    return;
  }
  avatarInput.value?.click();
};

// 上传头像并同步导航栏使用的 localStorage.avatarUrl。
const handleAvatarChange = async (event) => {
  const file = event.target.files?.[0];
  if (!file) {
    return;
  }

  if (!file.type.startsWith('image/')) {
    ElMessage.warning('请选择图片文件');
    event.target.value = '';
    return;
  }
  if (file.size > 5 * 1024 * 1024) {
    ElMessage.warning('头像图片不能超过 5MB');
    event.target.value = '';
    return;
  }

  const uploadData = new FormData();
  uploadData.append('avatar', file);

  isUploadingAvatar.value = true;
  try {
    const response = await axios.post('/users/updates_avatar', uploadData, {
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'multipart/form-data'
      }
    });

    if (response.data.code === 200) {
      avatarUrl.value = response.data.data;
      localStorage.setItem('avatarUrl', response.data.data);
      ElMessage.success('头像上传成功！');
    } else {
      ElMessage.error(`头像更新失败：${response.data.error || response.data.message || '未知错误'}`);
    }
  } catch (error) {
    console.error('头像上传失败', error);
    ElMessage.error('上传失败，请稍后再试');
  } finally {
    isUploadingAvatar.value = false;
    // 清空 input，保证连续选择同一张图片也能触发 change。
    event.target.value = '';
  }
};

onMounted(() => {
  if (!token) {
    router.push('/');
    return;
  }

  // 两个请求互不依赖，并行发起，减少页面等待时间。
  loadUserData();
  // 同一登录会话内优先使用缓存；没有缓存时才在进入页面时生成一次。
  const cachedArchive = readCachedArchive();
  if (cachedArchive) {
    healthArchive.value = cachedArchive;
  } else {
    loadHealthArchive();
  }
});

onBeforeUnmount(() => {
  // 让仍在飞行中的档案请求结果失效，避免组件卸载后写状态。
  archiveRequestId += 1;
});
</script>

<template>
  <div class="profile-page">
    <AppNavbar />

    <main class="profile-main">
      <!-- 上方：个人信息 -->
      <section class="panel" aria-labelledby="profile-title">
        <div class="panel-head">
          <div>
            <h1 id="profile-title">个人信息</h1>
            <p>完善资料后，AI 给出的建议会更贴合你的情况。</p>
          </div>
          <span class="panel-badge">账号资料</span>
        </div>

        <div class="identity">
          <div
            class="avatar-wrap"
            title="点击修改头像"
            role="button"
            tabindex="0"
            @click="openAvatarPicker"
            @keydown.enter.prevent="openAvatarPicker"
            @keydown.space.prevent="openAvatarPicker"
          >
            <el-avatar :size="96" :src="avatarUrl" class="avatar" />
            <!-- 鼠标悬停时覆盖在头像上的提示，点击事件仍由外层容器接收。 -->
            <span class="avatar-hint">{{ isUploadingAvatar ? '上传中…' : '点击修改头像' }}</span>
          </div>
          <input
            ref="avatarInput"
            type="file"
            accept="image/*"
            class="avatar-input"
            @change="handleAvatarChange"
          />

          <div class="identity-text">
            <h2>{{ displayName }}</h2>
            <p>{{ formData.email || '暂未填写邮箱' }}</p>
            <p class="identity-tip">点击头像即可更换</p>
          </div>
        </div>

        <el-form :model="formData" label-position="top" class="profile-form">
          <div class="form-grid">
            <el-form-item label="账号">
              <el-input v-model="formData.account" :disabled="!isEditable" placeholder="请输入账号" />
            </el-form-item>
            <el-form-item label="邮箱">
              <el-input v-model="formData.email" :disabled="!isEditable" placeholder="请输入邮箱" />
            </el-form-item>
            <el-form-item label="姓名">
              <el-input v-model="formData.name" :disabled="!isEditable" placeholder="请输入姓名" />
            </el-form-item>
            <el-form-item label="年龄">
              <el-input v-model="formData.age" :disabled="!isEditable" placeholder="请输入年龄" />
            </el-form-item>
            <el-form-item label="性别">
              <el-input v-model="formData.gender" :disabled="!isEditable" placeholder="请输入性别" />
            </el-form-item>
            <el-form-item label="手机号">
              <el-input v-model="formData.phone_number" :disabled="!isEditable" placeholder="请输入手机号" />
            </el-form-item>
            <el-form-item label="皮肤病 / 既往病史">
              <el-input
                v-model="formData.medicalHistory"
                type="textarea"
                :rows="3"
                :disabled="!isEditable"
                placeholder="例如：湿疹反复 2 年，换季时加重"
              />
            </el-form-item>
            <el-form-item label="过敏史">
              <el-input
                v-model="formData.allergyHistory"
                type="textarea"
                :rows="3"
                :disabled="!isEditable"
                placeholder="没有请填“无”；有请写明过敏原"
              />
            </el-form-item>
          </div>

          <div class="form-actions">
            <el-button
              type="primary"
              :loading="isSubmitting"
              :disabled="!isEditable"
              @click="submitAll"
            >
              提交
            </el-button>
            <el-button :disabled="isEditable || isSubmitting" @click="enableEdit">
              编辑
            </el-button>
          </div>
        </el-form>
      </section>

      <!-- 下方：AI 健康档案 -->
      <section class="panel" aria-labelledby="archive-title">
        <div class="panel-head archive-panel-head">
          <div>
            <h1 id="archive-title">AI 健康档案</h1>
            <p>整合你填写的皮肤情况与 AI 问诊记录，形成一份持续更新的档案。</p>
          </div>
          <el-button text type="primary" :loading="archiveLoading" @click="loadHealthArchive">
            重新生成
          </el-button>
        </div>

        <div v-if="archiveLoading" class="archive-state">
          <el-skeleton :rows="6" animated />
          <p class="archive-state-text">AI 正在整理你的问诊记录，请稍候…</p>
        </div>
        <div v-else-if="archiveError" class="archive-state archive-error">
          <p>{{ archiveError }}</p>
          <el-button plain @click="loadHealthArchive">重试</el-button>
        </div>
        <article
          v-else-if="healthArchive"
          class="archive-body"
          v-html="renderMarkdown(healthArchive)"
        ></article>
        <el-empty v-else description="暂无可整理的问诊记录或皮肤情况" :image-size="90" />
      </section>
    </main>

    <footer class="profile-footer">
      <p>AI 建议仅供参考，如有不适请及时就医并遵医嘱</p>
    </footer>

  </div>
</template>

<style scoped>
.profile-page {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  background: var(--ai-mint-2);
}

.profile-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 22px;
  width: min(1080px, calc(100% - 40px));
  margin: 0 auto;
  padding: 28px 0 10px;
}

.panel {
  padding: 26px 28px;
  border: 1px solid var(--ai-line);
  border-radius: 24px;
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.panel-head h1 {
  margin: 0 0 6px;
  color: var(--ai-ink);
  font-size: 22px;
}

.panel-head p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 13px;
  line-height: 1.6;
}

.panel-badge {
  flex: 0 0 auto;
  padding: 5px 12px;
  border-radius: 99px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 12px;
  font-weight: 700;
}

.identity {
  display: flex;
  align-items: center;
  gap: 18px;
  padding-bottom: 20px;
  margin-bottom: 4px;
  border-bottom: 1px dashed var(--ai-line);
}

.avatar-wrap {
  position: relative;
  flex: 0 0 auto;
  width: 96px;
  height: 96px;
  border-radius: 50%;
  cursor: pointer;
  outline: none;
}

.avatar-wrap:focus-visible {
  box-shadow: 0 0 0 4px var(--ai-primary-soft);
}

.avatar {
  display: block;
  width: 96px;
  height: 96px;
  border: 3px solid #fff;
  background: var(--ai-primary-soft);
  box-shadow: var(--ai-shadow-sm);
}

.avatar-hint {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 10px;
  border-radius: 50%;
  background: rgba(11, 69, 87, 0.72);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  text-align: center;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease;
}

.avatar-wrap:hover .avatar-hint,
.avatar-wrap:focus-visible .avatar-hint {
  opacity: 1;
}

.avatar-input {
  display: none;
}

.identity-text h2 {
  margin: 0 0 4px;
  color: var(--ai-ink);
  font-size: 20px;
}

.identity-text p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 13px;
  word-break: break-all;
}

.identity-text .identity-tip {
  margin-top: 6px;
  color: var(--ai-primary);
  font-size: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 20px;
}

.profile-form :deep(.el-form-item__label) {
  color: var(--ai-ink-soft);
  font-weight: 600;
}

.form-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 10px;
  margin-top: 4px;
}

.form-actions :deep(.el-button) {
  margin-left: 0;
}

/* 禁用态按钮仍保持主题色，显式降低透明度，避免看起来可以点击。 */
.form-actions :deep(.el-button.is-disabled) {
  opacity: 0.45;
  cursor: not-allowed;
}

.archive-body {
  color: var(--ai-ink-soft);
  font-size: 14px;
  line-height: 1.8;
  word-break: break-word;
}

.archive-body :deep(h1),
.archive-body :deep(h2),
.archive-body :deep(h3) {
  margin: 18px 0 8px;
  color: var(--ai-ink);
  font-size: 16px;
  line-height: 1.5;
}

.archive-body :deep(h1:first-child),
.archive-body :deep(h2:first-child),
.archive-body :deep(h3:first-child) {
  margin-top: 0;
}

.archive-body :deep(p) {
  margin: 8px 0;
}

.archive-body :deep(ul),
.archive-body :deep(ol) {
  margin: 8px 0;
  padding-left: 20px;
}

.archive-body :deep(li) {
  margin: 4px 0;
}

.archive-body :deep(strong) {
  color: var(--ai-ink);
}

.archive-body :deep(blockquote) {
  margin: 10px 0;
  padding: 10px 14px;
  border-left: 3px solid var(--ai-primary);
  border-radius: 0 10px 10px 0;
  background: #fff;
}

.archive-state {
  padding: 8px 0;
}

.archive-state-text {
  margin: 12px 0 0;
  color: var(--ai-muted);
  font-size: 13px;
  text-align: center;
}

.archive-error p {
  margin: 0 0 12px;
  color: #b42318;
  font-size: 13px;
}

.profile-footer {
  width: min(1080px, calc(100% - 40px));
  margin: 16px auto 0;
  padding: 18px 0 24px;
  border-top: 1px solid var(--ai-line);
}

.profile-footer p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 12px;
  text-align: center;
}

@media (max-width: 760px) {
  .profile-main {
    width: calc(100% - 24px);
    gap: 16px;
    padding-top: 16px;
  }

  .panel {
    padding: 20px 16px;
    border-radius: 18px;
  }

  .panel-head {
    flex-direction: column;
  }

  .panel-badge {
    align-self: flex-start;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .profile-footer {
    width: calc(100% - 24px);
  }
}
</style>
