<template>
  <div class="cases-page">
    <AppNavbar />

    <main class="cases-main">
      <section class="cases-hero">
        <span class="ai-eyebrow"><span class="ai-pulse"></span> 皮肤健康知识库</span>
        <h1>常见皮肤问题，<br><em>先从了解开始</em></h1>
        <p>
          下面整理了 4 类常见皮肤表现。点击任一病例，可以直接带着你的症状进入 AI 问诊，
          让描述更具体、沟通更高效。
        </p>
      </section>

      <section class="cases-grid">
        <RouterLink to="/home" class="case-card">
          <div class="case-image"><img src="../assets/图片3.png" alt="玫瑰糠疹" /></div>
          <div class="case-body">
            <span class="case-tag">红斑、脱屑</span>
            <h3>玫瑰糠疹</h3>
            <p>常见于躯干和四肢近端，常表现为椭圆形红斑，表面可有细碎鳞屑。</p>
            <span class="case-link">咨询 AI <ion-icon name="arrow-forward-outline"></ion-icon></span>
          </div>
        </RouterLink>

        <RouterLink to="/home" class="case-card">
          <div class="case-image"><img src="../assets/图片4.png" alt="体癣" /></div>
          <div class="case-body">
            <span class="case-tag">环状、瘙痒</span>
            <h3>体癣</h3>
            <p>由真菌感染引起，常见环状红斑、边缘隆起，中心区域可逐渐消退。</p>
            <span class="case-link">咨询 AI <ion-icon name="arrow-forward-outline"></ion-icon></span>
          </div>
        </RouterLink>

        <RouterLink to="/home" class="case-card">
          <div class="case-image"><img src="../assets/图片2.png" alt="毛细血管扩张" /></div>
          <div class="case-body">
            <span class="case-tag">红血丝、敏感</span>
            <h3>毛细血管扩张</h3>
            <p>面部或鼻翼出现可见的红血丝，常与皮肤屏障受损、日晒或敏感相关。</p>
            <span class="case-link">咨询 AI <ion-icon name="arrow-forward-outline"></ion-icon></span>
          </div>
        </RouterLink>

        <RouterLink to="/home" class="case-card">
          <div class="case-image"><img src="../assets/图片1.png" alt="荨麻疹" /></div>
          <div class="case-body">
            <h3>荨麻疹</h3>
            <p>
              <span class="case-tag">风团、剧痒</span>
              突然出现的风团样皮疹，常伴明显瘙痒，可能与过敏、感染或环境因素有关。
            </p>
            <span class="case-link">咨询 AI <ion-icon name="arrow-forward-outline"></ion-icon></span>
          </div>
        </RouterLink>
      </section>

      <section class="cases-help">
        <div>
          <h2>不确定属于哪一种？</h2>
          <p>直接上传患处照片，AI 会根据图像特征给出初步分类，并提示是否需要尽快就医。</p>
        </div>
        <RouterLink to="/home" class="ai-ghost-btn">上传照片分析</RouterLink>
      </section>
    </main>
  </div>
</template>

<script setup>
import AppNavbar from '../components/AppNavbar.vue';
</script>

<style scoped>
.cases-page {
  min-height: 100vh;
  background: var(--ai-mint-2);
}

.cases-main {
  width: min(1180px, calc(100% - 40px));
  margin: 0 auto;
  padding: clamp(36px, 5vw, 64px) 0 56px;
}

.cases-hero {
  max-width: 720px;
}

.cases-hero h1 {
  margin: 18px 0 14px;
  color: var(--ai-ink);
  font-size: clamp(34px, 4.4vw, 54px);
  line-height: 1.15;
  letter-spacing: -0.045em;
  font-weight: 750;
}

.cases-hero h1 em {
  color: var(--ai-primary);
  font-style: normal;
}

.cases-hero p {
  margin: 0 0 24px;
  color: var(--ai-ink-soft);
  font-size: 15px;
  line-height: 1.85;
}

.cases-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 18px;
  margin-top: 38px;
}

.case-card {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--ai-line);
  border-radius: 20px;
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
  color: inherit;
  text-decoration: none;
  transition: transform 0.22s ease, box-shadow 0.22s ease;
}

.case-card:hover {
  transform: translateY(-5px);
  box-shadow: var(--ai-shadow);
}

.case-image {
  height: 190px;
  overflow: hidden;
  background: var(--ai-mint);
}

.case-image img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.35s ease;
}

.case-card:hover .case-image img {
  transform: scale(1.06);
}

.case-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 18px;
}

.case-tag {
  display: inline-block;
  margin-bottom: 8px;
  padding: 4px 9px;
  border-radius: 99px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 10px;
  font-weight: 700;
}

/* 独立标签不要被 flex 拉伸成整行 */
.case-body > .case-tag {
  align-self: flex-start;
}

/* 标签移入描述段落后，改成行内胶囊并与正文留出间距 */
.case-body p .case-tag {
  margin: 0 6px 0 0;
  vertical-align: middle;
}

.case-body h3 {
  margin: 0 0 7px;
  color: var(--ai-ink);
  font-size: 17px;
  font-weight: 680;
}

.case-body p {
  margin: 0 0 14px;
  color: var(--ai-muted);
  font-size: 12px;
  line-height: 1.7;
}

.case-link {
  display: inline-flex;
  align-self: flex-start;
  align-items: center;
  gap: 4px;
  margin-top: auto;
  color: var(--ai-primary);
  font-size: 12px;
  font-weight: 650;
}

.cases-help {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-top: 30px;
  padding: 26px 28px;
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background:
    linear-gradient(120deg, rgba(255, 255, 255, 0.98), rgba(237, 249, 246, 0.98)),
    #fff;
  box-shadow: var(--ai-shadow-sm);
}

.cases-help h2 {
  margin: 0 0 6px;
  color: var(--ai-ink);
  font-size: 19px;
  font-weight: 700;
}

.cases-help p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 13px;
}

@media (max-width: 1024px) {
  .cases-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .cases-main {
    width: calc(100% - 28px);
    padding-top: 28px;
  }

  .cases-hero h1 {
    font-size: clamp(32px, 9vw, 42px);
  }

  .cases-grid {
    grid-template-columns: 1fr;
    margin-top: 26px;
  }

  .cases-help {
    flex-direction: column;
    align-items: flex-start;
  }

  .cases-help .ai-ghost-btn {
    width: 100%;
  }
}
</style>
