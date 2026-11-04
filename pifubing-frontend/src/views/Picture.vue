<script setup>
import { ref, onMounted } from 'vue';
import axios from 'axios';
import { useRouter } from 'vue-router';
import AppNavbar from '../components/AppNavbar.vue';

// 图鉴使用统一风格的自绘医学示意图，后续可直接替换为真实临床照片
import eczemaIllustration from '../assets/gallery/eczema.svg';
import urticariaIllustration from '../assets/gallery/urticaria.svg';
import tineaIllustration from '../assets/gallery/tinea.svg';

const galleryItems = [
  {
    id: 'eczema',
    image: eczemaIllustration,
    title: '湿疹样皮炎',
    tag: '红斑、干燥、脱屑',
    description: '常见于面颊、肘窝或腘窝，边界多不清晰，常伴明显瘙痒。'
  },
  {
    id: 'urticaria',
    image: urticariaIllustration,
    title: '荨麻疹',
    tag: '风团、瘙痒、时起时消',
    description: '典型表现为隆起的风团，中央偏白、周围红晕，单个皮损多在 24 小时内消退。'
  },
  {
    id: 'tinea',
    image: tineaIllustration,
    title: '体癣',
    tag: '环形红斑、边缘脱屑',
    description: '真菌感染常见环形红斑，边缘活跃脱屑，中央可相对消退，需规范抗真菌治疗。'
  }
];

// 定义数据结构
const imageData = ref([]);

const router = useRouter();
const token = localStorage.getItem('token');

// 从后端获取数据的方法（未登录时跳过，不影响首页展示）
const fetchData = async () => {
  const token = localStorage.getItem('token');
  if (!token) return;

  try {
    const headers = {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'multipart/form-data'
    };

    const response = await axios.get('/users/Check/Images_Description', { headers });
    console.log('请求数据成功', response.data);

    if (response.data.code === 200) {
      imageData.value = response.data.data;
    } else {
      console.error('数据加载失败:', response.data);
    }
  } catch (error) {
    console.error('请求失败:', error);
  }
};

// 在组件挂载时调用 fetchData
onMounted(() => {
  fetchData();
});
</script>

<template>
  <div class="dashboard-page">
    <AppNavbar />

    <main class="dashboard-main">
      <!-- 首屏：一句话告诉用户“现在可以做什么”，避免旧版大图轮播喧宾夺主 -->
      <section class="dashboard-hero">
        <div class="dashboard-hero-copy">
          <span class="ai-eyebrow"><span class="ai-pulse"></span> 今日皮肤健康助手</span>
          <h1>你好，今天想了解<br><em>哪方面的皮肤问题？</em></h1>
          <p>
            上传一张清晰的患处照片，或直接描述瘙痒、红斑、脱屑等症状。
            AI 会先给出初步判断，再陪你一步步梳理护理与就医建议。
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

      <!-- 主内容：图鉴 + 问诊准备，形成清晰的两栏信息层级 -->
      <section class="dashboard-grid">
        <div class="dashboard-card carousel-card">
          <div class="section-head">
            <div>
              <h2 class="ai-section-title">皮肤健康图鉴</h2>
              <p class="ai-section-sub">常见皮肤表现速览，帮助你更准确地描述症状</p>
            </div>
          </div>

          <el-carousel
            indicator-position="outside"
            :height="'clamp(240px, 30vw, 360px)'"
            class="gallery-carousel"
          >
            <el-carousel-item v-for="item in galleryItems" :key="item.id">
              <figure class="gallery-slide">
                <img
                  :src="item.image"
                  :alt="`${item.title}医学示意图`"
                  class="carousel-image"
                />
                <figcaption class="gallery-caption">
                  <div class="gallery-caption-row">
                    <h3>{{ item.title }}</h3>
                  </div>
                  <p>{{ item.tag }}，{{ item.description }}</p>
                </figcaption>
              </figure>
            </el-carousel-item>
          </el-carousel>
          <p class="gallery-note">图示为医学示意图，仅用于健康科普，不作为诊断依据。</p>
        </div>

        <aside class="dashboard-card preparation-card">
          <h2 class="ai-section-title">问诊前准备</h2>
          <p class="ai-section-sub">准备得越充分，建议越贴合</p>
          <ul class="check-list">
            <li><span>01</span><div><strong>自然光下拍摄</strong><small>避免美颜、滤镜和强反光</small></div></li>
            <li><span>02</span><div><strong>补充症状描述</strong><small>出现部位、持续时间、是否瘙痒</small></div></li>
            <li><span>03</span><div><strong>说明用药情况</strong><small>近期是否使用药膏或抗过敏药</small></div></li>
          </ul>
          <RouterLink to="/home" class="side-cta">
            上传照片开始分析
            <ion-icon name="arrow-forward-outline"></ion-icon>
          </RouterLink>
        </aside>
      </section>

      <!-- 服务说明：保留原有业务介绍，改为可读性更高的卡片 -->
      <section class="dashboard-card about-card">
        <div class="about-copy">
          <h2 class="ai-section-title">AI 皮肤问诊，如何帮助你？</h2>
          <p>
            AI 皮肤问诊利用图像识别与皮肤科知识库，辅助你理解皮肤问题的可能方向。
            它不会替代医生，但可以帮你在就医前整理症状、减少焦虑，并在需要时提醒你及时线下就诊。
          </p>
        </div>
        <div class="about-metrics">
          <div><strong>图像</strong><span>患处照片初步分类</span></div>
          <div><strong>知识库</strong><span>名称 / 概述 / 症状 / 治疗</span></div>
          <div><strong>对话</strong><span>连续追问与护理建议</span></div>
        </div>
      </section>

      <!-- 典型皮肤病参考：数据仍来自 /users/Check/Images_Description -->
      <section class="disease-library">
        <div class="section-head">
          <div>
            <h2 class="ai-section-title">典型皮肤病参考</h2>
            <p class="ai-section-sub">以下内容由后台知识库提供，仅作健康科普</p>
          </div>
        </div>

        <div v-if="imageData.length" class="disease-grid">
          <article v-for="item in imageData" :key="item.id" class="disease-card">
            <img :src="item.image_url" :alt="item.description" />
            <div class="disease-card-body">
              <p>{{ item.description }}</p>
              <RouterLink to="/home">咨询 AI <ion-icon name="arrow-forward-outline"></ion-icon></RouterLink>
            </div>
          </article>
        </div>
        <div v-else class="disease-empty">
          知识库内容加载中，或当前暂无可展示的参考条目。
        </div>
      </section>
    </main>

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

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(280px, 0.65fr);
  gap: 20px;
  margin-top: 28px;
}

.dashboard-card {
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
}

.carousel-card {
  padding: 24px;
}

.section-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 18px;
}

.gallery-slide {
  position: relative;
  width: 100%;
  height: 100%;
  margin: 0;
  overflow: hidden;
  border-radius: 16px;
  background: var(--ai-mint);
}

.carousel-image {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  border-radius: 16px;
}

.gallery-caption {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  padding: 46px 20px 18px;
  background: linear-gradient(180deg, transparent, rgba(8, 36, 43, 0.78));
  color: #fff;
  pointer-events: none;
}

.gallery-tag {
  display: inline-flex;
  align-items: center;
  padding: 4px 9px;
  border: 1px solid rgba(255, 255, 255, 0.34);
  border-radius: 99px;
  background: rgba(255, 255, 255, 0.14);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.gallery-caption-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 8px 0 5px;
}

.gallery-caption h3 {
  margin: 0;
  color: #fff;
  font-size: 22px;
  line-height: 1.2;
  letter-spacing: -0.02em;
}

.gallery-disclaimer {
  padding: 2px 7px;
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 99px;
  color: rgba(255, 255, 255, 0.86);
  font-size: 10px;
  font-weight: 500;
}

.gallery-caption p {
  max-width: 580px;
  margin: 0;
  color: rgba(255, 255, 255, 0.82);
  font-size: 12px;
  line-height: 1.6;
}

.gallery-note {
  margin: 8px 0 0;
  color: var(--ai-muted);
  font-size: 11px;
  text-align: right;
}

.carousel-card :deep(.el-carousel__indicators--outside) {
  margin-top: 14px;
}

.carousel-card :deep(.el-carousel__button) {
  width: 22px;
  height: 4px;
  border-radius: 99px;
  background: var(--ai-line-strong);
}

.carousel-card :deep(.el-carousel__indicator.is-active .el-carousel__button) {
  background: var(--ai-primary);
}

.preparation-card {
  padding: 24px;
  display: flex;
  flex-direction: column;
}

.check-list {
  list-style: none;
  margin: 16px 0 20px;
  padding: 0;
  display: grid;
  gap: 12px;
}

.check-list li {
  display: flex;
  gap: 12px;
  padding: 12px;
  border-radius: 14px;
  background: #f7fbfa;
}

.check-list li > span {
  flex: 0 0 26px;
  height: 26px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 11px;
  font-weight: 700;
}

.check-list strong {
  display: block;
  color: var(--ai-ink);
  font-size: 13px;
  font-weight: 650;
}

.check-list small {
  color: var(--ai-muted);
  font-size: 12px;
  line-height: 1.5;
}

.side-cta {
  margin-top: auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 13px 15px;
  border-radius: 13px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  text-decoration: none;
  font-size: 13px;
  font-weight: 650;
  transition: background 0.2s ease;
}

.side-cta:hover {
  background: #d4eeea;
}

.about-card {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(280px, 0.7fr);
  gap: 30px;
  align-items: center;
  margin-top: 20px;
  padding: clamp(24px, 4vw, 40px);
  background:
    linear-gradient(120deg, rgba(255, 255, 255, 0.98), rgba(240, 250, 247, 0.98)),
    #fff;
}

.about-copy h2 {
  margin: 14px 0 10px;
}

.about-copy p {
  margin: 0 0 20px;
  color: var(--ai-ink-soft);
  font-size: 14px;
  line-height: 1.85;
}

.about-metrics {
  display: grid;
  gap: 10px;
}

.about-metrics > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 16px;
  border: 1px solid var(--ai-line);
  border-radius: 14px;
  background: #fff;
}

.about-metrics strong {
  color: var(--ai-primary);
  font-size: 16px;
}

.about-metrics span {
  color: var(--ai-muted);
  font-size: 12px;
  text-align: right;
}

.disease-library {
  margin-top: 34px;
}

.disease-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 18px;
}

.disease-card {
  overflow: hidden;
  border: 1px solid var(--ai-line);
  border-radius: 18px;
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
  transition: transform 0.22s ease, box-shadow 0.22s ease;
}

.disease-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--ai-shadow);
}

.disease-card img {
  width: 100%;
  height: 190px;
  object-fit: cover;
  display: block;
}

.disease-card-body {
  padding: 16px;
}

.disease-card-body p {
  display: -webkit-box;
  margin: 0 0 12px;
  overflow: hidden;
  color: var(--ai-ink-soft);
  font-size: 13px;
  line-height: 1.65;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
}

.disease-card-body a {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--ai-primary);
  font-size: 12px;
  font-weight: 650;
  text-decoration: none;
}

.disease-empty {
  padding: 34px;
  border: 1px dashed var(--ai-line-strong);
  border-radius: var(--ai-radius);
  color: var(--ai-muted);
  text-align: center;
  font-size: 13px;
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
  .dashboard-hero,
  .dashboard-grid,
  .about-card {
    grid-template-columns: 1fr;
  }

  .disease-grid {
    grid-template-columns: repeat(2, 1fr);
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

  .dashboard-grid {
    margin-top: 20px;
  }

  .carousel-card,
  .preparation-card {
    padding: 18px;
  }

  /* 移动端轮播高度仅 240px，收紧说明层，避免遮住大半张示意图 */
  .gallery-caption {
    padding: 26px 14px 14px;
  }

  .gallery-tag {
    padding: 3px 8px;
    font-size: 10px;
  }

  .gallery-caption h3 {
    font-size: 18px;
  }

  .gallery-caption p {
    display: -webkit-box;
    overflow: hidden;
    font-size: 11px;
    line-height: 1.5;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }

  .gallery-note {
    text-align: left;
  }

  .disease-grid {
    grid-template-columns: 1fr;
  }

  .dashboard-footer {
    width: calc(100% - 28px);
    flex-direction: column;
  }
}
</style>
