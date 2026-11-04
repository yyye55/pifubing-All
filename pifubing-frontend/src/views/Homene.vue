<template>
  <div class="landing-page">
    <AppNavbar />

    <main class="landing-main">
      <!-- Hero：一句话说明“AI 皮肤顾问能做什么”，并给出立即行动入口 -->
      <section class="landing-hero">
        <div class="hero-copy">
          <span class="ai-eyebrow"><span class="ai-pulse"></span> AI Dermatology Consultant</span>
          <h1>AI 让健康预见未来<br><em>让每一寸肌肤都被精准守护</em></h1>
          <p>
            上传患处照片，或用文字描述症状。AI 皮肤顾问会结合皮肤科知识库，
            给出初步识别、护理建议与就医提示，让不确定的皮肤问题先有一个清晰方向。
          </p>
          <div class="hero-actions">
            <RouterLink to="/HomeView" class="ai-primary-btn">
              开始免费咨询
              <ion-icon name="arrow-forward-outline"></ion-icon>
            </RouterLink>
            <RouterLink to="/Picture" class="ai-ghost-btn">了解服务</RouterLink>
          </div>
          <div class="hero-trust">
            <span><ion-icon name="shield-checkmark-outline"></ion-icon> 隐私保护</span>
            <span><ion-icon name="library-outline"></ion-icon> 知识库支撑</span>
            <span><ion-icon name="flash-outline"></ion-icon> 秒级初步分析</span>
          </div>
        </div>

        <div class="hero-visual" aria-label="AI 皮肤分析示意">
          <div class="visual-orbit orbit-one"></div>
          <div class="visual-orbit orbit-two"></div>
          <div class="ai-preview-card">
            <div class="preview-head">
              <span class="preview-status"><span class="ai-pulse"></span> AI 正在分析</span>
              <span class="preview-live">实时</span>
            </div>

            <div class="preview-image">
              <ion-icon name="scan-outline"></ion-icon>
              <span>患处图像</span>
              <small>请上传清晰、自然光下的照片</small>
            </div>

            <div class="preview-result">
              <div>
                <span class="result-label">初步识别</span>
                <strong>湿疹样皮炎</strong>
              </div>
              <span class="confidence-ring" aria-label="置信度 92%"></span>
            </div>

            <div class="preview-tags">
              <span>红斑</span><span>瘙痒</span><span>干燥</span>
            </div>

            <p class="preview-note">
              <ion-icon name="information-circle-outline"></ion-icon>
              结果仅作健康参考，建议结合线下医生诊断
            </p>
          </div>
        </div>
      </section>

      <section class="landing-features">
        <article class="feature-card">
          <span class="feature-icon"><ion-icon name="images-outline"></ion-icon></span>
          <h3>图像辅助识别</h3>
          <p>基于皮肤镜图像模型，对 15 类常见皮肤问题进行初步分类与置信度评估。</p>
        </article>
        <article class="feature-card">
          <span class="feature-icon"><ion-icon name="chatbubble-ellipses-outline"></ion-icon></span>
          <h3>连续健康对话</h3>
          <p>围绕症状、持续时间、用药与护理追问，保留上下文，回答更连贯。</p>
        </article>
        <article class="feature-card">
          <span class="feature-icon"><ion-icon name="document-text-outline"></ion-icon></span>
          <h3>个人健康档案</h3>
          <p>问诊历史、病史与过敏史统一归档，复诊时不再重复描述。</p>
        </article>
      </section>
    </main>

    <footer class="landing-footer">
      <span>Skin AI · Where Science Meets Humanity</span>
      <span>AI 建议不构成医疗诊断，如有不适请及时就医</span>
    </footer>
  </div>
</template>

<script setup>
import AppNavbar from '../components/AppNavbar.vue';
</script>

<style scoped>
.landing-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--ai-mint-2);
}

.landing-main {
  flex: 1;
  width: min(1180px, calc(100% - 40px));
  margin: 0 auto;
  padding: clamp(42px, 6vw, 82px) 0 44px;
}

.landing-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.02fr) minmax(380px, 0.85fr);
  gap: clamp(36px, 6vw, 80px);
  align-items: center;
}

.hero-copy h1 {
  margin: 20px 0 18px;
  color: var(--ai-ink);
  font-size: clamp(38px, 4.8vw, 62px);
  line-height: 1.12;
  letter-spacing: -0.045em;
  font-weight: 750;
}

.hero-copy h1 em {
  color: var(--ai-primary);
  font-style: normal;
}

.hero-copy > p {
  max-width: 600px;
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 16px;
  line-height: 1.85;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 28px;
}

.hero-trust {
  display: flex;
  flex-wrap: wrap;
  gap: 18px;
  margin-top: 30px;
  color: var(--ai-muted);
  font-size: 13px;
}

.hero-trust span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.hero-trust ion-icon {
  color: var(--ai-teal);
  font-size: 17px;
}

.hero-visual {
  position: relative;
  min-height: 430px;
  display: grid;
  place-items: center;
}

.visual-orbit {
  position: absolute;
  border-radius: 50%;
  border: 1px solid rgba(15, 111, 122, 0.12);
}

.orbit-one {
  width: 420px;
  height: 420px;
}

.orbit-two {
  width: 520px;
  height: 520px;
  border-style: dashed;
  opacity: 0.55;
}

.ai-preview-card {
  position: relative;
  z-index: 1;
  width: min(390px, 100%);
  padding: 22px;
  border: 1px solid rgba(255, 255, 255, 0.9);
  border-radius: 26px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 30px 80px rgba(16, 42, 55, 0.16);
  backdrop-filter: blur(18px);
}

.preview-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.preview-status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--ai-ink);
  font-size: 13px;
  font-weight: 650;
}

.preview-live {
  padding: 3px 9px;
  border-radius: 99px;
  background: #e8f8f4;
  color: var(--ai-primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.preview-image {
  height: 150px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px dashed var(--ai-line-strong);
  border-radius: 18px;
  background:
    linear-gradient(135deg, rgba(234, 247, 244, 0.8), rgba(255, 255, 255, 0.6)),
    repeating-linear-gradient(45deg, transparent, transparent 12px, rgba(15, 111, 122, 0.025) 12px, rgba(15, 111, 122, 0.025) 24px);
  color: var(--ai-primary);
}

.preview-image ion-icon {
  font-size: 34px;
}

.preview-image span {
  font-size: 14px;
  font-weight: 650;
}

.preview-image small {
  color: var(--ai-muted);
  font-size: 11px;
}

.preview-result {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin: 16px 0 12px;
  padding: 14px 16px;
  border-radius: 16px;
  background: #f5fbf9;
}

.result-label {
  display: block;
  color: var(--ai-muted);
  font-size: 11px;
  letter-spacing: 0.08em;
}

.preview-result strong {
  display: block;
  margin-top: 2px;
  color: var(--ai-ink);
  font-size: 18px;
}

.confidence-ring {
  width: 52px;
  height: 52px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: conic-gradient(var(--ai-teal) 92%, #dcebe7 0);
  color: var(--ai-primary-deep);
  font-size: 12px;
  font-weight: 750;
  position: relative;
}

.confidence-ring::before {
  content: "";
  position: absolute;
  inset: 6px;
  border-radius: 50%;
  background: #f5fbf9;
}

.confidence-ring {
  isolation: isolate;
}

.confidence-ring::after {
  content: "92%";
  position: relative;
  z-index: 1;
}

.preview-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.preview-tags span {
  padding: 4px 10px;
  border-radius: 99px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 11px;
  font-weight: 600;
}

.preview-note {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  margin: 14px 0 0;
  color: var(--ai-muted);
  font-size: 11px;
  line-height: 1.55;
}

.preview-note ion-icon {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--ai-amber);
}

.landing-features {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 18px;
  margin-top: clamp(52px, 7vw, 92px);
}

.feature-card {
  padding: 24px;
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background: rgba(255, 255, 255, 0.82);
  box-shadow: var(--ai-shadow-sm);
  transition: transform 0.22s ease, box-shadow 0.22s ease;
}

.feature-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--ai-shadow);
}

.feature-icon {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  border-radius: 13px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 22px;
}

.feature-card h3 {
  margin: 16px 0 8px;
  color: var(--ai-ink);
  font-size: 17px;
  font-weight: 680;
}

.feature-card p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 13px;
  line-height: 1.75;
}

.landing-footer {
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
  .landing-hero {
    grid-template-columns: 1fr;
  }

  .hero-visual {
    min-height: 380px;
  }

  .landing-features {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .landing-main {
    width: calc(100% - 28px);
    padding-top: 34px;
  }

  .hero-copy h1 {
    font-size: clamp(32px, 9vw, 44px);
  }

  .hero-copy > p {
    font-size: 14px;
  }

  .hero-actions .ai-primary-btn,
  .hero-actions .ai-ghost-btn {
    width: 100%;
  }

  .hero-visual {
    min-height: 360px;
  }

  .orbit-one {
    width: 320px;
    height: 320px;
  }

  .orbit-two {
    width: 390px;
    height: 390px;
  }

  .landing-footer {
    flex-direction: column;
    width: calc(100% - 28px);
  }
}
</style>