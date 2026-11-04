<template>
  <div class="care-page" :class="{ 'care-xl': extraLarge }">
    <AppNavbar large />

    <main class="care-main">
      <!-- 关怀模式首屏：大字、强对比、操作按钮足够大 -->
      <section class="care-hero">
        <span class="care-eyebrow"><ion-icon name="accessibility-outline"></ion-icon> 关怀模式</span>
        <h1>看得更清楚，<br><em>用得更安心。</em></h1>
        <p>
          这里放大了文字和按钮，减少了装饰。您可以用更大的字阅读皮肤健康知识，
          也可以直接上传照片或描述症状，让 AI 帮您整理问诊信息。
        </p>
        <div class="care-actions">
          <RouterLink to="/home" class="care-primary">
            <ion-icon name="chatbubbles-outline"></ion-icon>
            开始 AI 问诊
          </RouterLink>
          <button type="button" class="care-toggle" @click="extraLarge = !extraLarge">
            <ion-icon :name="extraLarge ? 'contract-outline' : 'expand-outline'"></ion-icon>
            {{ extraLarge ? '恢复标准字号' : '再放大字体' }}
          </button>
        </div>
        <p class="care-hint">如果不方便打字，也可以请家人陪同，或直接前往医院就诊。</p>
      </section>

      <!-- 三步说明：每条只保留一个动作，避免信息过载 -->
      <section class="care-steps" aria-label="使用步骤">
        <article>
          <span>1</span>
          <h2>拍一张清晰照片</h2>
          <p>在自然光下拍摄患处，不要开美颜或滤镜。</p>
        </article>
        <article>
          <span>2</span>
          <h2>说说哪里不舒服</h2>
          <p>告诉 AI 部位、持续多久、是否瘙痒或疼痛。</p>
        </article>
        <article>
          <span>3</span>
          <h2>阅读建议</h2>
          <p>AI 会给出初步方向和护理提示，请结合医生意见判断。</p>
        </article>
      </section>

      <!-- 紧急情况提示：使用高对比警示色，信息直接 -->
      <section class="care-help">
        <div class="care-help-icon"><ion-icon name="warning-outline"></ion-icon></div>
        <div>
          <h2>这些情况请尽快就医</h2>
          <p>大面积皮疹、呼吸困难、发热、剧烈疼痛、皮肤快速溃烂，或症状在短时间内明显加重。</p>
        </div>
        <RouterLink to="/doctor" class="care-help-link">查看常见病例</RouterLink>
      </section>

      <section class="care-note">
        <p>AI 建议仅供健康科普和问诊参考，不能替代执业医师的诊断与治疗。</p>
      </section>
    </main>

    <footer class="care-footer">
      <span>Skin AI · 关怀模式</span>
      <RouterLink to="/">返回普通版</RouterLink>
    </footer>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import AppNavbar from '../components/AppNavbar.vue';

// 在关怀模式基础上再放大一级字号，方便视力不佳的用户
const extraLarge = ref(false);
</script>

<style scoped>
.care-page {
  min-height: 100vh;
  background: #f7fbfa;
  font-size: 18px;
}

.care-main {
  width: min(980px, calc(100% - 40px));
  margin: 0 auto;
  padding: clamp(30px, 5vw, 56px) 0 48px;
}

.care-hero {
  padding: clamp(26px, 5vw, 48px);
  border: 2px solid var(--ai-line);
  border-radius: 26px;
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
}

.care-eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 8px 14px;
  border-radius: 99px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 14px;
  font-weight: 700;
}

.care-hero h1 {
  margin: 16px 0;
  color: var(--ai-ink);
  font-size: clamp(40px, 6.4vw, 64px);
  line-height: 1.15;
  letter-spacing: -0.04em;
}

.care-hero h1 em {
  color: var(--ai-primary);
  font-style: normal;
}

.care-hero > p {
  max-width: 760px;
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 18px;
  line-height: 1.9;
}

.care-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 28px;
}

.care-primary,
.care-toggle {
  min-height: 58px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  padding: 0 26px;
  border-radius: 999px;
  font-size: 18px;
  font-weight: 700;
  text-decoration: none;
  cursor: pointer;
}

.care-primary {
  background: var(--ai-primary);
  color: #fff;
  box-shadow: 0 10px 24px rgba(15, 111, 122, 0.22);
}

.care-primary:hover {
  background: var(--ai-primary-deep);
  color: #fff;
}

.care-toggle {
  border: 2px solid var(--ai-line-strong);
  background: #fff;
  color: var(--ai-primary);
}

.care-toggle:hover {
  background: var(--ai-primary-soft);
}

.care-hint {
  margin-top: 16px !important;
  color: var(--ai-muted) !important;
  font-size: 15px !important;
}

.care-steps {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-top: 20px;
}

.care-steps article {
  padding: 24px;
  border: 2px solid var(--ai-line);
  border-radius: 20px;
  background: #fff;
}

.care-steps article > span {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  margin-bottom: 14px;
  border-radius: 50%;
  background: var(--ai-primary);
  color: #fff;
  font-size: 18px;
  font-weight: 700;
}

.care-steps h2 {
  margin: 0 0 8px;
  color: var(--ai-ink);
  font-size: 20px;
  line-height: 1.4;
}

.care-steps p {
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 16px;
  line-height: 1.8;
}

.care-help {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 16px;
  align-items: center;
  margin-top: 20px;
  padding: 22px 24px;
  border: 2px solid #efd49d;
  border-radius: 20px;
  background: #fff8e8;
}

.care-help-icon {
  display: grid;
  place-items: center;
  width: 50px;
  height: 50px;
  border-radius: 14px;
  background: #ffe8b8;
  color: #9a6408;
  font-size: 26px;
}

.care-help h2 {
  margin: 0 0 5px;
  color: #5f4514;
  font-size: 20px;
}

.care-help p {
  margin: 0;
  color: #7c612d;
  font-size: 16px;
  line-height: 1.7;
}

.care-help-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 48px;
  padding: 0 18px;
  border: 2px solid #d9bb7c;
  border-radius: 999px;
  color: #7c612d;
  font-size: 15px;
  font-weight: 700;
  text-decoration: none;
  white-space: nowrap;
}

.care-help-link:hover {
  background: #ffe8b8;
}

.care-note {
  margin-top: 18px;
  padding: 16px 20px;
  border-radius: 16px;
  background: #eef7f5;
}

.care-note p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 14px;
  line-height: 1.7;
}

.care-footer {
  width: min(980px, calc(100% - 40px));
  margin: 0 auto;
  padding: 20px 0 30px;
  border-top: 1px solid var(--ai-line);
  display: flex;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  color: var(--ai-muted);
  font-size: 14px;
}

.care-footer a {
  color: var(--ai-primary);
  font-weight: 600;
  text-decoration: none;
}

/* 再放大一档：只放大正文与操作区，避免顶栏溢出 */
.care-xl {
  font-size: 21px;
}

.care-xl .care-hero h1 {
  font-size: clamp(46px, 7vw, 72px);
}

.care-xl .care-hero > p,
.care-xl .care-steps p,
.care-xl .care-help p {
  font-size: 20px;
}

.care-xl .care-steps h2,
.care-xl .care-help h2 {
  font-size: 23px;
}

.care-xl .care-primary,
.care-xl .care-toggle {
  min-height: 64px;
  font-size: 21px;
}

@media (max-width: 820px) {
  .care-steps {
    grid-template-columns: 1fr;
  }

  .care-help {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .care-help-link {
    grid-column: 1 / -1;
    justify-self: start;
  }
}

@media (max-width: 640px) {
  .care-page {
    font-size: 17px;
  }

  .care-main {
    width: calc(100% - 28px);
    padding-top: 24px;
  }

  .care-hero h1 {
    font-size: clamp(36px, 10vw, 46px);
  }

  .care-hero > p {
    font-size: 17px;
  }

  .care-actions {
    flex-direction: column;
  }

  .care-primary,
  .care-toggle {
    width: 100%;
    min-height: 60px;
    font-size: 17px;
  }

  .care-help {
    grid-template-columns: 1fr;
  }

  .care-footer {
    width: calc(100% - 28px);
    flex-direction: column;
  }
}
</style>
