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
              <span class="preview-status">三步完成一次问诊</span>
              <span class="preview-live">免费使用</span>
            </div>

            <ol class="preview-steps">
              <li>
                <span class="step-no">01</span>
                <div>
                  <strong>上传照片或描述症状</strong>
                  <small>自然光下拍摄患处照片，或直接描述瘙痒、红斑、脱屑等情况</small>
                </div>
              </li>
              <li>
                <span class="step-no">02</span>
                <div>
                  <strong>AI 给出初步分析</strong>
                  <small>图像模型识别 15 类常见皮肤病，结合皮肤科知识库给出护理与就医建议</small>
                </div>
              </li>
              <li>
                <span class="step-no">03</span>
                <div>
                  <strong>记录进健康档案</strong>
                  <small>问诊记录自动保存到个人中心，可随时回看历史对话与建议</small>
                </div>
              </li>
            </ol>

            <p class="preview-note">
              <ion-icon name="information-circle-outline"></ion-icon>
              AI 建议仅作健康参考，不替代线下医生诊断
            </p>
          </div>
        </div>
      </section>

      <section class="landing-features" aria-label="典型使用场景">
        <article class="feature-card">
          <span class="feature-icon"><ion-icon name="images-outline"></ion-icon></span>
          <h3>皮肤出现说不清的变化</h3>
          <p>红点、脱屑、瘙痒或色斑，先拍一张清晰患处照片，了解可能方向，减少盲目搜索。</p>
        </article>
        <article class="feature-card">
          <span class="feature-icon"><ion-icon name="refresh-outline"></ion-icon></span>
          <h3>皮肤问题反复发作</h3>
          <p>结合皮肤科知识库和你的历史问诊，给出日常护理重点，并提示什么时候该去线下就诊。</p>
        </article>
        <article class="feature-card">
          <span class="feature-icon"><ion-icon name="clipboard-outline"></ion-icon></span>
          <h3>就医前想先整理症状</h3>
          <p>把部位、持续时间、诱因和用药情况先理清楚，面诊时表达更完整，沟通更高效。</p>
        </article>
      </section>

      <!-- 转化入口：不重复功能介绍，只给一次明确的行动引导 -->
      <section class="landing-cta">
        <div>
          <h2>皮肤问题，不必先猜</h2>
          <p>不必带着不确定反复搜索，也不必等到症状加重。现在开始，把皮肤问题先理清楚。</p>
        </div>
        <RouterLink to="/HomeView" class="ai-primary-btn">
          立即开始问诊
          <ion-icon name="arrow-forward-outline"></ion-icon>
        </RouterLink>
      </section>
    </main>

    <footer class="landing-footer">
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

.preview-steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 10px;
}

.preview-steps li {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--ai-line);
  border-radius: 14px;
  background: #f7fbfa;
}

.step-no {
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

.preview-steps strong {
  display: block;
  color: var(--ai-ink);
  font-size: 13px;
  font-weight: 650;
}

.preview-steps small {
  display: block;
  margin-top: 2px;
  color: var(--ai-muted);
  font-size: 12px;
  line-height: 1.5;
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

.landing-cta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  flex-wrap: wrap;
  margin-top: clamp(28px, 4vw, 48px);
  padding: clamp(24px, 4vw, 40px);
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
}

.landing-cta h2 {
  margin: 0 0 8px;
  color: var(--ai-ink);
  font-size: clamp(22px, 2.4vw, 30px);
  letter-spacing: -0.02em;
}

.landing-cta p {
  margin: 0;
  max-width: 620px;
  color: var(--ai-muted);
  font-size: 14px;
  line-height: 1.75;
}

.landing-cta .ai-primary-btn {
  flex: 0 0 auto;
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
  .hero-actions .ai-ghost-btn,
  .landing-cta .ai-primary-btn {
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