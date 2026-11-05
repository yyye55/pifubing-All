<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import AppNavbar from '../components/AppNavbar.vue';

import eczemaImage from '../assets/cases/eczema.png';
import psoriasisImage from '../assets/cases/psoriasis.png';
import atopicDermatitisImage from '../assets/cases/atopic-dermatitis.png';
import exanthemImage from '../assets/cases/exanthem.png';
import roseaImage from '../assets/cases/rosea.png';
import tineaImage from '../assets/cases/tinea.png';
import telangiectasiaImage from '../assets/图片2.png';
import urticariaImage from '../assets/图片1.png';

const caseItems = [
  {
    id: 'eczema',
    image: eczemaImage,
    title: '湿疹',
    description: '皮肤屏障受损引起的炎症反应，容易反复发作，抓挠后会加重。',
    symptoms: '红斑、密集小丘疹、抓破后渗液结痂，夜间瘙痒明显。',
    relief: '以保湿修复为主，避免搔抓和热水烫洗，急性期按医嘱短期使用弱效激素药膏。',
    question: '我想了解湿疹：皮肤出现红斑、密集丘疹，很痒还反复发作，日常该怎么护理和缓解？'
  },
  {
    id: 'psoriasis',
    image: psoriasisImage,
    title: '银屑病',
    description: '边界清楚的红色斑块，表面覆盖银白色鳞屑，常见于肘膝伸侧和头皮。',
    symptoms: '红色斑块、厚层银白色鳞屑，刮除鳞屑后可见点状出血，冬季易加重。',
    relief: '先软化鳞屑并坚持保湿，避免抠抓、熬夜和饮酒；外用激素或卡泊三醇需遵医嘱，范围大时及时就医。',
    question: '我想了解银屑病：身上有边界清楚的红斑和银白色鳞屑，反复发作，该怎么缓解和护理？'
  },
  {
    id: 'atopic-dermatitis',
    image: atopicDermatitisImage,
    title: '特应性皮炎',
    description: '慢性反复的过敏性皮肤病，皮肤干燥、瘙痒明显，常累及肘窝、腘窝等屈侧部位。',
    symptoms: '皮肤干燥粗糙、剧烈瘙痒，屈侧分布，长期搔抓后皮肤增厚呈苔藓样变。',
    relief: '每天足量涂抹润肤剂，温水短时沐浴并避免搓澡，远离羊毛衣物和已知过敏原，发作期按医嘱用药。',
    question: '我想了解特应性皮炎：皮肤很干、特别痒，肘窝腘窝反复发作，日常该怎么护理和缓解？'
  },
  {
    id: 'exanthem',
    image: exanthemImage,
    title: '发疹',
    description: '局部出现的成簇小水疱，常在免疫力下降、劳累或受凉后出现。',
    symptoms: '红斑基础上成簇透明小水疱，伴灼热、刺痛或瘙痒，可反复出现在同一部位。',
    relief: '保持局部清洁干燥、不要挤压水疱，尽早就医评估是否需要抗病毒治疗，越早处理效果越好。',
    question: '我想了解发疹：局部长了成簇的小水疱，有灼热刺痛感，需要怎么处理，要不要尽快就医？'
  },
  {
    id: 'rosea',
    image: roseaImage,
    title: '玫瑰糠疹',
    description: '常见于躯干和四肢近端，多表现为椭圆形红斑，表面有细碎鳞屑。',
    symptoms: '先出现较大的母斑，随后成批出现椭圆形红斑，边缘有领圈状鳞屑，轻度瘙痒。',
    relief: '多数可自行消退，期间注意保湿止痒、避免热水烫洗和剧烈运动出汗，持续不缓解再就医。',
    question: '我想了解玫瑰糠疹：躯干出现椭圆形红斑，边缘有细碎鳞屑，有点痒，该怎么处理？'
  },
  {
    id: 'tinea',
    image: tineaImage,
    title: '体癣',
    description: '真菌感染引起，常见环状红斑、边缘隆起，中心区域可逐渐消退。',
    symptoms: '环形红斑、边缘活跃脱屑并隆起，中央相对自愈，伴明显瘙痒。',
    relief: '保持患处干燥透气、毛巾衣物单独清洗，需要规范抗真菌治疗，不要自行涂激素药膏。',
    question: '我想了解体癣：皮肤出现环形红斑、边缘脱屑，很痒，该怎么治疗和预防复发？'
  },
  {
    id: 'telangiectasia',
    image: telangiectasiaImage,
    title: '毛细血管扩张',
    description: '面部或鼻翼出现可见红血丝，常与皮肤屏障受损、日晒或敏感相关。',
    symptoms: '面颊、鼻翼可见细小红血丝，遇热、情绪激动或日晒后潮红加重，伴紧绷感。',
    relief: '坚持防晒、温和清洁，避免频繁去角质和冷热刺激，明显影响外观时可咨询激光治疗。',
    question: '我想了解毛细血管扩张：面部出现红血丝，遇热更明显，平时该怎么护理和改善？'
  },
  {
    id: 'urticaria',
    image: urticariaImage,
    title: '荨麻疹',
    description: '突然出现的风团样皮疹，常伴明显瘙痒，可能与过敏、感染或环境因素有关。',
    symptoms: '隆起风团、中央偏白周围红晕，单个皮损多在 24 小时内消退，可在身体各处反复出现。',
    relief: '记录并避开可疑诱因，按医嘱口服抗组胺药；若出现呼吸困难、眼睑肿胀需立即就医。',
    question: '我想了解荨麻疹：身上反复起风团，很痒，一会消一会起，该怎么缓解和排查诱因？'
  }
];

// 点击图片放大查看，与问诊跳转互不影响
const previewItem = ref(null);

function openPreview(item) {
  previewItem.value = item;
  document.body.style.overflow = 'hidden';
}

function closePreview() {
  previewItem.value = null;
  document.body.style.overflow = '';
}

function handleKeydown(event) {
  if (event.key === 'Escape') closePreview();
}

onMounted(() => window.addEventListener('keydown', handleKeydown));

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown);
  document.body.style.overflow = '';
});
</script>

<template>
  <div class="dashboard-page">
    <AppNavbar />

    <main class="dashboard-main">
      <!-- 首屏：保持问候式开头，右侧给出服务概览 -->
      <section class="dashboard-hero">
        <div class="dashboard-hero-copy">
          <span class="ai-eyebrow"><span class="ai-pulse"></span> 今日皮肤健康助手</span>
          <h1>你好，今天想了解<br><em>哪方面的皮肤问题？</em></h1>
          <p>
            先从下方常见病例里找一找相似的皮肤表现，也可以直接上传患处照片或描述症状，
            AI 会给出初步判断与护理方向。
          </p>
          <div class="dashboard-actions">
            <RouterLink to="/home" class="ai-primary-btn">
              体验 AI 问诊
              <ion-icon name="arrow-forward-outline"></ion-icon>
            </RouterLink>
            <RouterLink to="/Summarize" class="ai-ghost-btn">查看健康档案</RouterLink>
          </div>
        </div>

        <aside class="dashboard-summary-card">
          <div class="summary-header">
            <span>健康服务概览</span>
            <span class="ai-pill">AI 在线</span>
          </div>
          <div class="summary-row">
            <strong>15</strong>
            <span>类常见皮肤病知识覆盖</span>
          </div>
          <div class="summary-row">
            <strong>2 种</strong>
            <span>图像识别 + 文字问诊</span>
          </div>
          <div class="summary-row">
            <strong>隐私</strong>
            <span>账号隔离，记录仅自己可见</span>
          </div>
          <div class="summary-foot">
            <ion-icon name="shield-checkmark-outline"></ion-icon>
            AI 建议不替代线下医生诊断
          </div>
        </aside>
      </section>

      <!-- 主内容：常见病例，点击卡片把该病的提问带入 AI 问诊输入框 -->
      <section class="cases-section">
        <div class="section-head">
          <div>
            <h2 class="ai-section-title">常见病例</h2>
            <p class="ai-section-sub">点击图片可放大查看，点击「咨询 AI」会把该病的提问带入问诊输入框</p>
          </div>
        </div>

        <div class="cases-grid">
          <article v-for="item in caseItems" :key="item.id" class="case-card">
            <!-- 图片单独作为按钮：只负责放大预览，不触发问诊跳转 -->
            <button
              type="button"
              class="case-image"
              :aria-label="'放大查看' + item.title"
              @click="openPreview(item)"
            >
              <img :src="item.image" :alt="item.title" />
              <span class="case-zoom-hint">
                <ion-icon name="expand-outline"></ion-icon>
              </span>
            </button>

            <div class="case-body">
              <div class="case-head">
                <h3>{{ item.title }}</h3>
                <RouterLink
                  :to="{ path: '/home', query: { ask: item.question } }"
                  class="case-link"
                >
                  咨询 AI
                  <ion-icon name="arrow-forward-outline"></ion-icon>
                </RouterLink>
              </div>
              <p class="case-desc">{{ item.description }}</p>

              <div class="case-detail">
                <div class="case-row">
                  <span class="case-label">常见表现</span>
                  <p>{{ item.symptoms }}</p>
                </div>
                <div class="case-row">
                  <span class="case-label">如何缓解</span>
                  <p>{{ item.relief }}</p>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>
    </main>

    <!-- 图片放大预览层：点击遮罩或关闭按钮退出，Esc 也可关闭 -->
    <Teleport to="body">
      <div v-if="previewItem" class="preview-mask" @click.self="closePreview">
        <div class="preview-panel">
          <button type="button" class="preview-close" aria-label="关闭预览" @click="closePreview">
            <ion-icon name="close-outline"></ion-icon>
          </button>
          <img :src="previewItem.image" :alt="previewItem.title" />
          <div class="preview-caption">
            <h3>{{ previewItem.title }}</h3>
            <p>{{ previewItem.description }}</p>
          </div>
        </div>
      </div>
    </Teleport>

    <footer class="dashboard-footer">
      <span>AI 建议仅供参考，如有不适请及时就医并遵医嘱</span>
    </footer>
  </div>
</template>

<style scoped>
.dashboard-page {
  min-height: 100vh;
  background: var(--ai-mint-2);
}

.dashboard-main {
  width: min(1180px, calc(100% - 40px));
  margin: 0 auto;
  padding: clamp(34px, 5vw, 64px) 0 56px;
}

.dashboard-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(300px, 0.65fr);
  gap: clamp(28px, 5vw, 64px);
  align-items: center;
}

.dashboard-hero-copy h1 {
  margin: 18px 0 16px;
  color: var(--ai-ink);
  font-size: clamp(34px, 4.4vw, 54px);
  line-height: 1.15;
  letter-spacing: -0.045em;
  font-weight: 750;
}

.dashboard-hero-copy h1 em {
  color: var(--ai-primary);
  font-style: normal;
}

.dashboard-hero-copy > p {
  max-width: 620px;
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 15px;
  line-height: 1.85;
}

.dashboard-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 26px;
}

.dashboard-summary-card {
  padding: 24px;
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background: rgba(255, 255, 255, 0.9);
  box-shadow: var(--ai-shadow);
}

.summary-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  color: var(--ai-ink);
  font-size: 14px;
  font-weight: 700;
}

.summary-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 14px 0;
  border-bottom: 1px solid var(--ai-line);
}

.summary-row strong {
  flex: 0 0 62px;
  color: var(--ai-primary);
  font-size: 22px;
  line-height: 1;
}

.summary-row span {
  color: var(--ai-muted);
  font-size: 13px;
}

.summary-foot {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 16px;
  color: var(--ai-muted);
  font-size: 11px;
}

.summary-foot ion-icon {
  color: var(--ai-amber);
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 18px;
}

.cases-section {
  margin-top: clamp(32px, 4.5vw, 54px);
}

/* 两列横向小卡片：左侧缩略图、右侧病名 + 表现/缓解，一屏能看更多病例 */
.cases-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.case-card {
  display: grid;
  grid-template-columns: 148px minmax(0, 1fr);
  overflow: hidden;
  border: 1px solid var(--ai-line);
  border-radius: 14px;
  background: #fff;
  box-shadow: 0 2px 6px rgba(16, 42, 55, 0.04);
  color: inherit;
  text-decoration: none;
  transition: transform 0.28s ease, box-shadow 0.28s ease, border-color 0.28s ease;
}

.case-card:hover {
  transform: translateY(-4px);
  border-color: rgba(15, 111, 122, 0.32);
  box-shadow: 0 14px 28px rgba(16, 42, 55, 0.12);
}

/* 图片层：整体是按钮，悬停时轻微放大并显示放大图标 */
.case-image {
  position: relative;
  display: block;
  overflow: hidden;
  width: 100%;
  height: 100%;
  padding: 0;
  border: 0;
  background: var(--ai-mint);
  cursor: zoom-in;
}

.case-image img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.5s ease, filter 0.4s ease;
}

.case-card:hover .case-image img {
  transform: scale(1.09);
  filter: saturate(1.08);
}

.case-zoom-hint {
  position: absolute;
  right: 8px;
  bottom: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: rgba(16, 42, 55, 0.6);
  color: #fff;
  font-size: 13px;
  opacity: 0;
  transition: opacity 0.24s ease;
}

.case-card:hover .case-zoom-hint {
  opacity: 1;
}

.case-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px 16px;
}

/* 病名与「咨询 AI」同一行，按钮固定在卡片右上角 */
.case-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.case-head h3 {
  margin: 0;
  color: var(--ai-ink);
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.case-desc {
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 12px;
  line-height: 1.7;
}

/* 表现与缓解放进浅色块，扫读时能快速区分“这是什么”和“该怎么办” */
.case-detail {
  display: grid;
  gap: 7px;
  margin-top: auto;
  padding: 10px 12px;
  border-radius: 10px;
  background: #f6fbfa;
}

.case-row {
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
}

.case-label {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 20px;
  border-radius: 6px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 10.5px;
  font-weight: 700;
  white-space: nowrap;
}

.case-row p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 11.5px;
  line-height: 1.65;
}

/* 右上角的咨询入口：小胶囊按钮，只有它会把病例提问带进问诊 */
.case-link {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 3px;
  padding: 4px 9px;
  border: 1px solid var(--ai-line);
  border-radius: 999px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 11.5px;
  font-weight: 650;
  text-decoration: none;
  white-space: nowrap;
  transition: background 0.22s ease, border-color 0.22s ease, color 0.22s ease;
}

.case-link:hover {
  border-color: var(--ai-primary);
  background: var(--ai-primary);
  color: #fff;
}

.case-link ion-icon {
  font-size: 12px;
  transition: transform 0.22s ease;
}

.case-link:hover ion-icon {
  transform: translateX(3px);
}

/* 图片放大预览：深色遮罩 + 居中大图，层级高于导航栏 */
.preview-mask {
  position: fixed;
  inset: 0;
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 28px;
  background: rgba(9, 26, 32, 0.8);
  backdrop-filter: blur(4px);
}

.preview-panel {
  position: relative;
  max-width: min(820px, 100%);
  max-height: 90vh;
  overflow: hidden;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.35);
}

.preview-panel img {
  display: block;
  width: 100%;
  max-height: 74vh;
  object-fit: contain;
  background: #10242b;
}

.preview-caption {
  padding: 13px 18px 15px;
}

.preview-caption h3 {
  margin: 0 0 4px;
  color: var(--ai-ink);
  font-size: 16px;
  font-weight: 700;
}

.preview-caption p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 12.5px;
  line-height: 1.7;
}

.preview-close {
  position: absolute;
  top: 10px;
  right: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: rgba(16, 42, 55, 0.68);
  color: #fff;
  font-size: 17px;
  cursor: pointer;
}

.preview-close:hover {
  background: rgba(16, 42, 55, 0.9);
}

.dashboard-footer {
  width: min(1180px, calc(100% - 40px));
  margin: 0 auto;
  padding: 22px 0 30px;
  border-top: 1px solid var(--ai-line);
  display: flex;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  color: var(--ai-muted);
  font-size: 12px;
}

@media (max-width: 980px) {
  .dashboard-hero {
    grid-template-columns: 1fr;
  }

  .cases-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .dashboard-main {
    width: calc(100% - 28px);
    padding-top: 28px;
  }

  .dashboard-hero-copy h1 {
    font-size: clamp(32px, 9vw, 42px);
  }

  .dashboard-actions .ai-primary-btn,
  .dashboard-actions .ai-ghost-btn {
    width: 100%;
  }

  /* 手机端改为上下结构，图片放在最上方，避免被压成窄条 */
  .case-card {
    grid-template-columns: 1fr;
  }

  .case-image {
    height: 150px;
  }

  .case-body {
    padding: 13px 15px;
  }

  .preview-mask {
    padding: 14px;
  }

  .dashboard-footer {
    width: calc(100% - 28px);
    flex-direction: column;
  }
}
</style>
