<template>
  <div class="about-page">
    <AppNavbar />

    <main class="about-main">
      <!-- 首屏：说明产品定位，强调“辅助理解”而不是“替代医生” -->
      <section class="about-hero">
        <div class="about-hero-copy">
          <span class="ai-eyebrow"><span class="ai-pulse"></span>关于我们</span>
          <h1>让专业的皮肤知识，<br><em>更容易被理解。</em></h1>
          <p>
            构建多模态 RAG 皮肤病 AI 诊疗系统：结合 EfficientNet-B3 与 Transformer 架构处理患处图像，
            利用 LangChain 架构、FAISS 向量数据库与 DeepSeek 大模型生成诊疗方案，
            让专业判断有出处、可追溯。
          </p>
          <ul class="about-tags">
            <li>智辅</li>
            <li>诊疗</li>
            <li>高速</li>
            <li>高效</li>
            <li>轻量化</li>
          </ul>
        </div>

        <aside class="about-hero-card">
          <div class="about-card-head">
            <span>我们的原则</span>
          </div>
          <ul>
            <li>
              <ion-icon name="shield-checkmark-outline"></ion-icon>
              <div><strong>辅助而非替代</strong><span>AI 建议不构成医疗诊断，必要时请及时线下就医。</span></div>
            </li>
            <li>
              <ion-icon name="lock-closed-outline"></ion-icon>
              <div><strong>隐私优先</strong><span>账号数据相互隔离，问诊记录仅本人可见。</span></div>
            </li>
            <li>
              <ion-icon name="book-outline"></ion-icon>
              <div><strong>知识有出处</strong><span>回答结合皮肤科知识库，减少无依据的猜测。</span></div>
            </li>
          </ul>
        </aside>
      </section>

      <!-- 三项技术：说明系统靠什么完成诊疗 -->
      <section class="about-values">
        <article class="about-value-card">
          <span class="value-icon"><ion-icon name="images-outline"></ion-icon></span>
          <h2>多图联合推理引擎</h2>
          <p>同一患处的多张照片联合判断，让多个视角互相印证，结论比单张图片更稳定可靠。</p>
        </article>
        <article class="about-value-card">
          <span class="value-icon"><ion-icon name="library-outline"></ion-icon></span>
          <h2>医学知识增强 RAG</h2>
          <p>以皮肤病教材、诊疗指南与真实病例构建检索库，LangChain 编排流程，FAISS 建立向量索引，生成可回溯来源的方案。</p>
        </article>
        <article class="about-value-card">
          <span class="value-icon"><ion-icon name="layers-outline"></ion-icon></span>
          <h2>跨模态动态融合</h2>
          <p>EfficientNet-B3 提取局部病灶细节，Transformer 抽取全局语义，门控机制按需融合两种特征，兼顾精度与部署成本。</p>
        </article>
      </section>

      <!-- 使用流程：降低第一次使用的理解成本 -->
      <section class="about-flow">
        <div class="flow-head">
          <div>
            <h2 class="ai-section-title">一次问诊，三步完成</h2>
            <p class="ai-section-sub">从描述问题到获得建议，全程由你决定是否继续</p>
          </div>
        </div>
        <ol>
          <li>
            <span>01</span>
            <div><strong>拍照或描述</strong><p>上传患处照片，或直接输入瘙痒、红斑、脱屑等症状。</p></div>
          </li>
          <li>
            <span>02</span>
            <div><strong>AI 初步分析</strong><p>图像分类与知识检索同时进行，给出可能方向和护理提示。</p></div>
          </li>
          <li>
            <span>03</span>
            <div><strong>继续追问</strong><p>围绕症状、诱因和用药继续对话，整理出可带给医生的信息。</p></div>
          </li>
        </ol>
      </section>

      <!-- 免责声明：医疗类产品必须有清晰、可读的边界说明 -->
      <section class="about-note">
        <div class="note-icon"><ion-icon name="information-circle-outline"></ion-icon></div>
        <div class="note-copy">
          <h2>关于 AI 建议的边界</h2>
          <p>
            AI 皮肤医疗顾问是健康科普与问诊辅助工具，不能替代执业医师的诊断和治疗。
            若出现大面积皮疹、呼吸困难、发热、剧烈疼痛或症状快速加重，请立即前往医院就诊。
          </p>
        </div>
      </section>
    </main>

    <footer class="about-footer">
      <span>AI 建议仅供参考，如有不适请及时就医并遵医嘱</span>
    </footer>
  </div>
</template>

<script setup>
import AppNavbar from '../components/AppNavbar.vue';
</script>

<style scoped>
.about-page {
  min-height: 100vh;
  background: var(--ai-mint-2);
}

.about-main {
  width: min(1180px, calc(100% - 40px));
  margin: 0 auto;
  padding: clamp(36px, 5vw, 68px) 0 56px;
}

.about-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(320px, 0.85fr);
  gap: clamp(30px, 5vw, 64px);
  align-items: center;
}

.about-hero-copy h1 {
  margin: 18px 0 18px;
  color: var(--ai-ink);
  font-size: clamp(36px, 4.6vw, 58px);
  line-height: 1.12;
  letter-spacing: -0.05em;
  font-weight: 750;
}

.about-hero-copy h1 em {
  color: var(--ai-primary);
  font-style: normal;
}

.about-hero-copy > p {
  max-width: 620px;
  margin: 0;
  color: var(--ai-ink-soft);
  font-size: 15px;
  line-height: 1.9;
}

.about-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 22px 0 0;
  padding: 0;
  list-style: none;
}

.about-tags li {
  padding: 6px 14px;
  border: 1px solid var(--ai-line);
  border-radius: 999px;
  background: #fff;
  color: var(--ai-primary);
  font-size: 12px;
  font-weight: 650;
  letter-spacing: 0.04em;
}

.about-hero-card {
  padding: 24px;
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background: rgba(255, 255, 255, 0.92);
  box-shadow: var(--ai-shadow);
}

.about-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--ai-line);
  color: var(--ai-ink);
  font-size: 14px;
  font-weight: 700;
}

.about-hero-card ul {
  list-style: none;
  margin: 0;
  padding: 4px 0 0;
}

.about-hero-card li {
  display: flex;
  gap: 12px;
  padding: 15px 0;
  border-bottom: 1px solid var(--ai-line);
}

.about-hero-card li:last-child {
  border-bottom: 0;
  padding-bottom: 2px;
}

.about-hero-card li > ion-icon {
  flex: 0 0 34px;
  height: 34px;
  padding: 8px;
  border-radius: 10px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 18px;
}

.about-hero-card strong {
  display: block;
  margin-bottom: 3px;
  color: var(--ai-ink);
  font-size: 13px;
  font-weight: 650;
}

.about-hero-card span {
  display: block;
  color: var(--ai-muted);
  font-size: 12px;
  line-height: 1.6;
}

.about-values {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 18px;
  margin-top: 30px;
}

.about-value-card {
  padding: 24px;
  border: 1px solid var(--ai-line);
  border-radius: 20px;
  background: #fff;
  box-shadow: var(--ai-shadow-sm);
}

.value-icon {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  margin-bottom: 16px;
  border-radius: 12px;
  background: var(--ai-primary-soft);
  color: var(--ai-primary);
  font-size: 21px;
}

.about-value-card h2 {
  margin: 0 0 8px;
  color: var(--ai-ink);
  font-size: 17px;
  letter-spacing: -0.02em;
}

.about-value-card p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 13px;
  line-height: 1.8;
}

.about-flow {
  margin-top: 20px;
  padding: clamp(24px, 4vw, 38px);
  border: 1px solid var(--ai-line);
  border-radius: var(--ai-radius);
  background:
    linear-gradient(120deg, rgba(255, 255, 255, 0.98), rgba(240, 250, 247, 0.98)),
    #fff;
  box-shadow: var(--ai-shadow-sm);
}

.flow-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.about-flow ol {
  list-style: none;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin: 24px 0 0;
  padding: 0;
}

.about-flow li {
  display: flex;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--ai-line);
  border-radius: 16px;
  background: #fff;
}

.about-flow li > span {
  flex: 0 0 34px;
  height: 34px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: var(--ai-primary);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
}

.about-flow strong {
  display: block;
  margin-bottom: 4px;
  color: var(--ai-ink);
  font-size: 14px;
}

.about-flow p {
  margin: 0;
  color: var(--ai-muted);
  font-size: 12px;
  line-height: 1.7;
}

.about-note {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 16px;
  align-items: center;
  margin-top: 20px;
  padding: 22px 24px;
  border: 1px solid #f0dfb9;
  border-radius: 18px;
  background: #fffaf0;
}

.note-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: #fff1d6;
  color: #b57b12;
  font-size: 22px;
}

.note-copy h2 {
  margin: 0 0 5px;
  color: #5f4514;
  font-size: 15px;
}

.note-copy p {
  margin: 0;
  color: #8a6b32;
  font-size: 12px;
  line-height: 1.75;
}

.note-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 10px 15px;
  border: 1px solid #ead5a9;
  border-radius: 99px;
  color: #8a6b32;
  font-size: 12px;
  font-weight: 600;
  text-decoration: none;
  white-space: nowrap;
}

.note-link:hover {
  background: #fff1d6;
}

.about-footer {
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
  .about-hero {
    grid-template-columns: 1fr;
  }

  .about-values {
    grid-template-columns: 1fr;
  }

  .about-flow ol {
    grid-template-columns: 1fr;
  }

  .about-note {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .note-link {
    grid-column: 1 / -1;
    justify-self: start;
  }
}

@media (max-width: 640px) {
  .about-main {
    width: calc(100% - 28px);
    padding-top: 28px;
  }

  .about-hero-copy h1 {
    font-size: clamp(32px, 9vw, 42px);
  }

  .flow-head {
    flex-direction: column;
  }

  .about-note {
    grid-template-columns: 1fr;
  }

  .about-footer {
    width: calc(100% - 28px);
    flex-direction: column;
  }
}
</style>
