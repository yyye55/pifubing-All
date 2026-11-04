<template>
  <header class="app-navbar" :class="{ 'app-navbar--large': props.large }">
    <RouterLink to="/" class="app-brand" aria-label="SkinCare AI 首页">
      <img src="../assets/t.png" alt="AI 皮肤医疗顾问" class="logo1" />
    </RouterLink>

    <nav class="navigations app-nav-links" aria-label="主导航">
      <RouterLink
        v-for="item in navItems"
        :key="item.id"
        :to="resolveTo(item)"
        class="nav-link"
        :class="{
          'is-active': isActive(item),
          'nav-cta': item.cta
        }"
        :aria-current="isActive(item) ? 'page' : undefined"
      >
        <span v-if="item.id === 'profile'" class="nav-avatar">
          <img
            v-if="avatarUrl && !avatarFailed"
            :src="avatarUrl"
            alt=""
            @error="avatarFailed = true"
          />
          <template v-else>我</template>
        </span>
        <span>{{ item.label }}</span>
      </RouterLink>
    </nav>
  </header>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute } from 'vue-router';

const props = defineProps({
  // 关怀模式使用更大的字号和点击区域，但导航项内容保持一致
  large: {
    type: Boolean,
    default: false
  }
});

const route = useRoute();
const token = ref(localStorage.getItem('token') || '');
const avatarUrl = ref(localStorage.getItem('avatarUrl') || '');
const avatarFailed = ref(false);

const NAV_ITEMS = [
  { id: 'home', label: '首页', path: '/Picture', requiresAuth: true },
  { id: 'cases', label: '常见病例', path: '/doctor' },
  { id: 'about', label: '关于我们', path: '/AboutUs' },
  { id: 'ai', label: '询问 AI', path: '/home', requiresAuth: true, cta: true },
  { id: 'profile', label: '个人中心', path: '/Summarize', requiresAuth: true }
];

const navItems = computed(() => NAV_ITEMS);

function refreshAuthState() {
  token.value = localStorage.getItem('token') || '';
  avatarUrl.value = localStorage.getItem('avatarUrl') || '';
  avatarFailed.value = false;
}

function resolveTo(item) {
  // 未登录时，受保护入口统一先进入登录页，导航文字和顺序保持不变
  if (item.requiresAuth && !token.value) {
    return '/HomeView';
  }
  return item.path;
}

function isActive(item) {
  return route.path === item.path;
}

onMounted(refreshAuthState);
watch(() => route.fullPath, refreshAuthState);
</script>

<style scoped>
.app-navbar {
  position: sticky;
  top: 0;
  z-index: 1000;
}

.app-brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  color: #f5fdfb;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: -0.02em;
  text-decoration: none;
  white-space: nowrap;
}

.app-brand img {
  border-radius: 8px;
}

.app-nav-links {
  min-width: 0;
}

.nav-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  height: 40px;
  padding: 0 16px;
  border: 1px solid rgba(255, 255, 255, 0.22);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.08);
  color: #f5fdfb;
  font-size: 14px;
  font-weight: 500;
  line-height: 1;
  text-decoration: none;
  white-space: nowrap;
  flex: 0 0 auto;
  transition: background 0.2s ease, border-color 0.2s ease, transform 0.2s ease;
}

.nav-link:hover {
  background: rgba(255, 255, 255, 0.16);
  border-color: rgba(255, 255, 255, 0.36);
  color: #fff;
  transform: translateY(-1px);
}

.nav-link:focus-visible {
  outline: 2px solid #fff;
  outline-offset: 2px;
}

.nav-link.is-active {
  background: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.58);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
  color: #fff;
}

.nav-link.nav-cta {
  border-color: #fff;
  background: #fff;
  color: var(--ai-primary-deep);
  font-weight: 700;
}

.nav-link.nav-cta:hover {
  background: var(--ai-mint);
  border-color: var(--ai-mint);
  color: var(--ai-primary-deep);
}

.nav-link.nav-cta.is-active {
  background: #fff;
  border-color: #fff;
  color: var(--ai-primary-deep);
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.12);
}

.nav-avatar {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  overflow: hidden;
  border-radius: 50%;
  background: #e7f4f1;
  color: var(--ai-primary);
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.nav-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

/* 关怀模式：只放大字号和点击区域，不改变导航内容 */
.app-navbar--large .app-brand {
  font-size: 19px;
}

.app-navbar--large .nav-link {
  height: 48px;
  padding: 0 19px;
  font-size: 16px;
}

.app-navbar--large .nav-avatar {
  width: 30px;
  height: 30px;
  font-size: 13px;
}

@media (max-width: 640px) {
  .app-brand {
    font-size: 15px;
  }

  .app-brand span {
    display: none;
  }

  .nav-link {
    height: 36px;
    padding: 0 12px;
    font-size: 13px;
  }

  .nav-avatar {
    width: 22px;
    height: 22px;
    font-size: 10px;
  }

  .app-navbar--large .nav-link {
    height: 42px;
    padding: 0 14px;
    font-size: 14px;
  }
}
</style>
